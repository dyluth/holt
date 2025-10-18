package cub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

const (
	// toolExecutionTimeout is the maximum time a tool can run before being killed
	toolExecutionTimeout = 5 * time.Minute

	// maxOutputSize is the maximum number of bytes to read from tool stdout/stderr (10MB)
	maxOutputSize = 10 * 1024 * 1024
)

// executeWork handles the complete workflow of executing an agent tool for a granted claim.
// This is the main entry point called by the work executor goroutine.
//
// Workflow:
//  1. Fetch target artefact from blackboard
//  2. Prepare tool input JSON (stdin)
//  3. Execute tool subprocess with timeout
//  4. Parse tool output JSON (stdout)
//  5. Create result artefact with derivative provenance
//  6. Publish artefact to blackboard
//
// On any failure, creates a Failure artefact and continues (never crashes).
func (e *Engine) executeWork(ctx context.Context, claim *blackboard.Claim) {
	log.Printf("[INFO] Work Executor received claim from queue: claim_id=%s artefact_id=%s",
		claim.ID, claim.ArtefactID)

	// Fetch target artefact
	targetArtefact, err := e.fetchTargetArtefact(ctx, claim)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch target artefact: %v", err)
		e.createFailureArtefact(ctx, claim, -1, "", "", fmt.Sprintf("Failed to fetch target artefact: %v", err))
		return
	}

	log.Printf("[INFO] Fetched target artefact: artefact_id=%s type=%s",
		targetArtefact.ID, targetArtefact.Type)

	// Prepare tool input with context assembly
	inputJSON, err := e.prepareToolInput(ctx, claim, targetArtefact)
	if err != nil {
		log.Printf("[ERROR] Failed to prepare tool input: %v", err)
		e.createFailureArtefact(ctx, claim, -1, "", "", fmt.Sprintf("Failed to prepare tool input: %v", err))
		return
	}

	// Execute tool subprocess
	log.Printf("[INFO] Executing tool: command=%v claim_id=%s", e.config.Command, claim.ID)
	startTime := time.Now()

	exitCode, stdout, stderr, err := e.executeToolSubprocess(ctx, inputJSON)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[ERROR] Tool execution failed: claim_id=%s exit_code=%d duration=%s error=%v",
			claim.ID, exitCode, duration, err)
		e.createFailureArtefact(ctx, claim, exitCode, stdout, stderr, err.Error())
		return
	}

	log.Printf("[INFO] Tool execution completed: claim_id=%s exit_code=%d duration=%s",
		claim.ID, exitCode, duration)

	// Parse tool output
	output, err := e.parseToolOutput(stdout)
	if err != nil {
		log.Printf("[ERROR] Failed to parse tool output: claim_id=%s error=%v stdout=%s",
			claim.ID, err, truncate(stdout, 200))
		e.createFailureArtefact(ctx, claim, exitCode, stdout, stderr,
			fmt.Sprintf("Failed to parse tool output: %v", err))
		return
	}

	// Create result artefact
	artefact, err := e.createResultArtefact(ctx, claim, output)
	if err != nil {
		log.Printf("[ERROR] Failed to create artefact: claim_id=%s error=%v", claim.ID, err)
		// Try to create a Failure artefact describing the artefact creation failure
		e.createFailureArtefact(ctx, claim, 0, stdout, stderr,
			fmt.Sprintf("Tool succeeded but artefact creation failed: %v", err))
		return
	}

	log.Printf("[INFO] Created artefact: artefact_id=%s type=%s logical_id=%s version=%d",
		artefact.ID, artefact.Type, artefact.LogicalID, artefact.Version)
}

// fetchTargetArtefact retrieves the artefact that the claim is for.
// Returns an error if the artefact doesn't exist (defensive check for orchestrator bugs).
func (e *Engine) fetchTargetArtefact(ctx context.Context, claim *blackboard.Claim) (*blackboard.Artefact, error) {
	artefact, err := e.bbClient.GetArtefact(ctx, claim.ArtefactID)
	if err != nil {
		return nil, fmt.Errorf("failed to get artefact %s: %w", claim.ArtefactID, err)
	}

	if artefact == nil {
		return nil, fmt.Errorf("artefact %s not found", claim.ArtefactID)
	}

	return artefact, nil
}

// prepareToolInput creates the JSON structure to pass to the tool via stdin.
// M2.4: Uses context assembly to populate context_chain with historical artefacts.
// M3.3: Passes claim to assembleContext for AdditionalContextIDs support.
func (e *Engine) prepareToolInput(ctx context.Context, claim *blackboard.Claim, targetArtefact *blackboard.Artefact) (string, error) {
	// Assemble context chain via BFS traversal with thread tracking
	// M3.3: Pass claim for feedback claim context support
	contextChain, err := e.assembleContext(ctx, targetArtefact, claim)
	if err != nil {
		return "", fmt.Errorf("failed to assemble context: %w", err)
	}

	log.Printf("[DEBUG] Prepared context chain: %d artefacts", len(contextChain))

	// Convert to []interface{} for JSON marshaling
	contextChainInterface := make([]interface{}, len(contextChain))
	for i, art := range contextChain {
		contextChainInterface[i] = art
	}

	input := &ToolInput{
		ClaimType:      "exclusive", // M2.4: still hardcoded (Phase 3 will support review/parallel)
		TargetArtefact: targetArtefact,
		ContextChain:   contextChainInterface,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool input: %w", err)
	}

	return string(jsonBytes), nil
}

// executeToolSubprocess runs the agent command as a subprocess with timeout and output limits.
// Returns exit code, stdout, stderr, and error.
//
// The subprocess is:
//   - Given a 5-minute timeout via context
//   - Run in /workspace directory
//   - Fed input JSON via stdin (pipe closed after write)
//   - Output captured with 10MB limit on stdout and stderr
//
// Returns (exitCode, stdout, stderr, error) where:
//   - exitCode is the process exit code (0 = success, non-zero = failure, -1 = couldn't start)
//   - stdout is the captured standard output (truncated at 10MB)
//   - stderr is the captured standard error (truncated at 10MB)
//   - error is non-nil if the process failed, timed out, or output exceeded limits
func (e *Engine) executeToolSubprocess(ctx context.Context, inputJSON string) (int, string, string, error) {
	// Validate /workspace directory exists (fail-fast check)
	if _, err := os.Stat("/workspace"); os.IsNotExist(err) {
		return -1, "", "", fmt.Errorf("/workspace directory does not exist - agent container must mount workspace")
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, toolExecutionTimeout)
	defer cancel()

	// Create command
	if len(e.config.Command) == 0 {
		return -1, "", "", fmt.Errorf("command array is empty")
	}

	var cmd *exec.Cmd
	if len(e.config.Command) == 1 {
		cmd = exec.CommandContext(execCtx, e.config.Command[0])
	} else {
		cmd = exec.CommandContext(execCtx, e.config.Command[0], e.config.Command[1:]...)
	}

	// Set working directory
	cmd.Dir = "/workspace"

	// Create stdin pipe
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return -1, "", "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Create limited readers for stdout and stderr
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = &limitedWriter{w: stdoutBuf, limit: maxOutputSize}
	cmd.Stderr = &limitedWriter{w: stderrBuf, limit: maxOutputSize}

	// Start process
	if err := cmd.Start(); err != nil {
		return -1, "", "", fmt.Errorf("failed to start process: %w", err)
	}

	// Write input JSON to stdin and close pipe
	go func() {
		defer stdinPipe.Close()
		if _, err := io.WriteString(stdinPipe, inputJSON); err != nil {
			log.Printf("[WARN] Failed to write to stdin: %v", err)
		}
	}()

	// Wait for process to complete
	err = cmd.Wait()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Check for output size limit exceeded
	if stdoutBuf.Len() >= maxOutputSize || stderrBuf.Len() >= maxOutputSize {
		return -1, stdout, stderr, fmt.Errorf("tool output exceeded 10MB limit")
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Process couldn't be started or context timeout
			if execCtx.Err() == context.DeadlineExceeded {
				return -1, stdout, stderr, fmt.Errorf("tool execution timeout (5 minutes)")
			}
			return -1, stdout, stderr, err
		}
	}

	// Non-zero exit code is an error
	if exitCode != 0 {
		return exitCode, stdout, stderr, fmt.Errorf("process exited with code %d", exitCode)
	}

	return exitCode, stdout, stderr, nil
}

// parseToolOutput unmarshals and validates the tool's stdout JSON.
// Returns the parsed ToolOutput or an error if the JSON is invalid or missing required fields.
func (e *Engine) parseToolOutput(stdout string) (*ToolOutput, error) {
	if len(stdout) == 0 {
		return nil, fmt.Errorf("tool produced no output on stdout")
	}

	var output ToolOutput
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if err := output.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &output, nil
}

// createResultArtefact builds a new artefact from the tool output and writes it to the blackboard.
// M2.4: Uses the DERIVATIVE relationship model for new work
// M3.3: Automatically manages versioning for feedback claims (rework)
//
// For regular claims:
//   - New logical_id (creates new logical thread)
//   - Version = 1 (first version of this new work)
//   - source_artefacts = [claim.ArtefactID] (links back to input)
//
// For feedback claims (pending_assignment):
//   - Same logical_id as target (continues thread)
//   - Version = target.Version + 1 (increment version)
//   - Same type as target (rework, not new type)
//   - source_artefacts = [target.ID] + claim.AdditionalContextIDs (includes Review artefacts)
//
// Agents remain completely unaware of this versioning logic.
func (e *Engine) createResultArtefact(ctx context.Context, claim *blackboard.Claim, output *ToolOutput) (*blackboard.Artefact, error) {
	// M2.4: Validate git commit for CodeCommit artefacts
	if output.ArtefactType == "CodeCommit" {
		log.Printf("[INFO] Validating git commit: hash=%s", output.ArtefactPayload)
		if err := validateCommitExists(output.ArtefactPayload); err != nil {
			return nil, fmt.Errorf("git commit validation failed for hash %s: %w",
				output.ArtefactPayload, err)
		}
		log.Printf("[DEBUG] Git commit validation passed: hash=%s", output.ArtefactPayload)
	}

	// M3.3: Check if this is a feedback claim (rework scenario)
	if claim.Status == blackboard.ClaimStatusPendingAssignment {
		// This is a feedback claim - create rework artefact
		return e.createReworkArtefact(ctx, claim, output)
	}

	// Regular claim - create new work artefact
	// Generate new UUIDs for the artefact
	artefactID := uuid.New().String()
	logicalID := artefactID // Derivative: new logical thread

	artefact := &blackboard.Artefact{
		ID:              artefactID,
		LogicalID:       logicalID,
		Version:         1, // First version of new thread
		StructuralType:  output.GetStructuralType(),
		Type:            output.ArtefactType,
		Payload:         output.ArtefactPayload,
		SourceArtefacts: []string{claim.ArtefactID}, // Derivative from target artefact
		ProducedByRole:  e.config.AgentRole,
	}

	// Create artefact in Redis (also publishes event)
	if err := e.bbClient.CreateArtefact(ctx, artefact); err != nil {
		return nil, fmt.Errorf("failed to create artefact: %w", err)
	}

	// Add to thread tracking
	if err := e.bbClient.AddVersionToThread(ctx, logicalID, artefactID, 1); err != nil {
		// Log but don't fail - artefact was created successfully
		log.Printf("[WARN] Failed to add version to thread: logical_id=%s error=%v", logicalID, err)
	}

	return artefact, nil
}

// createFailureArtefact creates a Failure artefact describing a tool execution failure.
// Uses the same derivative provenance model as success artefacts.
// The failure payload contains diagnostic information (exit code, stdout, stderr, error message).
func (e *Engine) createFailureArtefact(ctx context.Context, claim *blackboard.Claim, exitCode int, stdout, stderr, reason string) {
	log.Printf("[INFO] Creating Failure artefact: claim_id=%s reason=%s", claim.ID, reason)

	// Prepare failure data
	failureData := &FailureData{
		Reason:   reason,
		ExitCode: exitCode,
		Stdout:   truncate(stdout, 5000), // Limit stored output
		Stderr:   truncate(stderr, 5000),
		Error:    reason,
	}

	payload, err := MarshalFailurePayload(failureData)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal failure payload: %v", err)
		payload = fmt.Sprintf(`{"reason": "Failed to marshal failure data: %v"}`, err)
	}

	// Generate new UUIDs
	artefactID := uuid.New().String()
	logicalID := artefactID // Derivative: new logical thread

	artefact := &blackboard.Artefact{
		ID:              artefactID,
		LogicalID:       logicalID,
		Version:         1,
		StructuralType:  blackboard.StructuralTypeFailure,
		Type:            "ToolExecutionFailure",
		Payload:         payload,
		SourceArtefacts: []string{claim.ArtefactID},
		ProducedByRole:  e.config.AgentRole,
	}

	// Create artefact
	if err := e.bbClient.CreateArtefact(ctx, artefact); err != nil {
		log.Printf("[ERROR] Failed to create Failure artefact: %v", err)
		return
	}

	// Add to thread tracking
	if err := e.bbClient.AddVersionToThread(ctx, logicalID, artefactID, 1); err != nil {
		log.Printf("[WARN] Failed to add Failure artefact to thread: %v", err)
	}

	log.Printf("[INFO] Created Failure artefact: artefact_id=%s claim_id=%s", artefactID, claim.ID)
}

// limitedWriter wraps a writer and enforces a size limit.
// Once the limit is reached, further writes are discarded.
type limitedWriter struct {
	w       io.Writer
	limit   int
	written int
}

func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	remaining := lw.limit - lw.written
	if remaining <= 0 {
		// Already hit limit, discard this write
		return len(p), nil
	}

	// Write up to the limit
	toWrite := p
	if len(p) > remaining {
		toWrite = p[:remaining]
	}

	n, err = lw.w.Write(toWrite)
	lw.written += n
	return len(p), err // Return len(p) to satisfy the writer interface
}

// truncate limits a string to maxLen characters, appending "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// createReworkArtefact builds a new version of the target artefact using agent output.
// M3.3: Automatically manages versioning for feedback claims - agents remain unaware.
//
// The cub:
//   - Fetches the target artefact to get its logical_id, version, and type
//   - Creates a new artefact with:
//     * Same logical_id (continues the thread)
//     * Version = target.version + 1 (automatic increment)
//     * Same type as target (rework, not new type)
//     * source_artefacts = [target.ID] + claim.AdditionalContextIDs (target + Review artefacts)
//   - Agent tool is completely unaware of versioning
func (e *Engine) createReworkArtefact(ctx context.Context, claim *blackboard.Claim, output *ToolOutput) (*blackboard.Artefact, error) {
	// Fetch the target artefact to get logical_id, version, and type
	targetArtefact, err := e.bbClient.GetArtefact(ctx, claim.ArtefactID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target artefact: %w", err)
	}
	if targetArtefact == nil {
		return nil, fmt.Errorf("target artefact %s not found", claim.ArtefactID)
	}

	log.Printf("[INFO] Creating rework artefact: logical_id=%s version=%d→%d type=%s",
		targetArtefact.LogicalID, targetArtefact.Version, targetArtefact.Version+1, targetArtefact.Type)

	// Build source_artefacts: target + Review artefacts
	sourceArtefacts := []string{targetArtefact.ID}
	sourceArtefacts = append(sourceArtefacts, claim.AdditionalContextIDs...)

	// Generate new artefact ID
	artefactID := uuid.New().String()

	artefact := &blackboard.Artefact{
		ID:              artefactID,
		LogicalID:       targetArtefact.LogicalID,       // Same thread
		Version:         targetArtefact.Version + 1,      // Increment version
		StructuralType:  output.GetStructuralType(),
		Type:            targetArtefact.Type,             // Same type (rework)
		Payload:         output.ArtefactPayload,
		SourceArtefacts: sourceArtefacts,                // Target + Reviews
		ProducedByRole:  e.config.AgentRole,
	}

	// Create artefact in Redis (also publishes event)
	if err := e.bbClient.CreateArtefact(ctx, artefact); err != nil {
		return nil, fmt.Errorf("failed to create rework artefact: %w", err)
	}

	// Add to thread tracking
	if err := e.bbClient.AddVersionToThread(ctx, artefact.LogicalID, artefact.ID, artefact.Version); err != nil {
		// Log but don't fail - artefact was created successfully
		log.Printf("[WARN] Failed to add rework artefact to thread: logical_id=%s error=%v",
			artefact.LogicalID, err)
	}

	log.Printf("[INFO] Rework artefact created: id=%s logical_id=%s version=%d (agent unaware of versioning)",
		artefact.ID, artefact.LogicalID, artefact.Version)

	return artefact, nil
}
