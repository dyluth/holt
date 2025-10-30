package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// OutputFormat defines the output format for watch streaming
type OutputFormat string

const (
	OutputFormatDefault OutputFormat = "default"
	OutputFormatJSONL   OutputFormat = "jsonl"
)

// FilterCriteria defines filtering options for watch command.
// All filters are ANDed together.
type FilterCriteria struct {
	SinceTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	UntilTimestampMs int64  // Unix timestamp in milliseconds, 0 = no filter
	TypeGlob         string // Glob pattern for artefact type, empty = no filter
	AgentRole        string // Exact match for produced_by_role, empty = no filter
}

// matchesFilter returns true if the artefact matches all filter criteria.
func (fc *FilterCriteria) matchesFilter(art *blackboard.Artefact) bool {
	// Time filtering
	if fc.SinceTimestampMs > 0 && art.CreatedAtMs < fc.SinceTimestampMs {
		return false
	}
	if fc.UntilTimestampMs > 0 && art.CreatedAtMs > fc.UntilTimestampMs {
		return false
	}

	// Type filtering - glob pattern matching
	if fc.TypeGlob != "" {
		matched, err := filepath.Match(fc.TypeGlob, art.Type)
		if err != nil || !matched {
			return false
		}
	}

	// Agent filtering - exact match on produced_by_role
	if fc.AgentRole != "" && art.ProducedByRole != fc.AgentRole {
		return false
	}

	return true
}

// hasFilters returns true if any filters are active.
func (fc *FilterCriteria) hasFilters() bool {
	return fc.SinceTimestampMs > 0 ||
		fc.UntilTimestampMs > 0 ||
		fc.TypeGlob != "" ||
		fc.AgentRole != ""
}

// PollForClaim polls for claim creation for a given artefact ID.
// Returns the created claim or an error if timeout occurs.
// Polls every 200ms for the specified timeout duration.
func PollForClaim(ctx context.Context, client *blackboard.Client, artefactID string, timeout time.Duration) (*blackboard.Claim, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for claim after %v", timeout)

		case <-ticker.C:
			claim, err := client.GetClaimByArtefactID(ctx, artefactID)
			if err != nil {
				if blackboard.IsNotFound(err) {
					// Not found yet, continue polling
					continue
				}
				return nil, fmt.Errorf("failed to query for claim: %w", err)
			}

			// Success!
			return claim, nil
		}
	}
}

// StreamActivity streams workflow events to the provided writer with filtering support.
// Displays historical events first (if filters active), then streams live events.
// Subscribes to artefact_events, claim_events, and workflow_events channels.
// Handles reconnection on transient failures with 2s retry interval and 60s timeout.
//
// If exitOnCompletion is true, exits with nil when a Terminal artefact is detected.
func StreamActivity(ctx context.Context, client *blackboard.Client, instanceName string, format OutputFormat, filters *FilterCriteria, exitOnCompletion bool, writer io.Writer) error {
	// Create formatter
	var formatter eventFormatter
	switch format {
	case OutputFormatJSONL:
		formatter = &jsonlFormatter{writer: writer}
	default:
		formatter = &defaultFormatter{writer: writer}
	}

	// Phase 1: Query and display historical events if filters are active
	// Note: For now, we only query historical artefacts. Claims and workflow events
	// are typically short-lived and stored in Redis with TTL, so historical query
	// focuses on artefacts which are the primary persistent data.
	// Live streaming will show all event types (artefacts, claims, workflow events).
	if filters != nil && filters.hasFilters() {
		if err := displayHistoricalArtefacts(ctx, client, instanceName, filters, formatter); err != nil {
			// Log error but continue to live streaming
			log.Printf("⚠️  Failed to query historical artefacts: %v", err)
		}
	}

	// Phase 2: Subscribe to live events with reconnection logic
	for {
		err := streamWithSubscriptions(ctx, client, formatter, filters, exitOnCompletion)
		if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
			// Clean exit (includes Terminal detection if exitOnCompletion)
			return nil
		}

		// Connection error - attempt reconnection
		fmt.Fprintf(writer, "⚠️  Connection to blackboard lost. Reconnecting...\n")

		// Try to reconnect with timeout
		reconnectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err = reconnectWithRetry(reconnectCtx, client, 2*time.Second)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to reconnect after 60s: %w", err)
		}

		fmt.Fprintf(writer, "✓ Reconnected to blackboard\n")
	}
}

// displayHistoricalArtefacts queries and displays historical artefacts and claims matching filters.
func displayHistoricalArtefacts(ctx context.Context, client *blackboard.Client, instanceName string, filters *FilterCriteria, formatter eventFormatter) error {
	// Collect both artefacts and claims
	type event struct {
		timestamp int64
		artefact  *blackboard.Artefact
		claim     *blackboard.Claim
	}
	var events []event

	// Scan for all artefact keys
	artefactPattern := fmt.Sprintf("holt:%s:artefact:*", instanceName)
	iter := client.RedisClient().Scan(ctx, 0, artefactPattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()

		// Extract artefact ID from key
		artefactPrefix := fmt.Sprintf("holt:%s:artefact:", instanceName)
		if len(key) <= len(artefactPrefix) {
			continue
		}
		artefactID := key[len(artefactPrefix):]

		// Fetch artefact
		artefact, err := client.GetArtefact(ctx, artefactID)
		if err != nil {
			// Skip malformed artefacts
			continue
		}

		// Apply filters
		if filters.matchesFilter(artefact) {
			events = append(events, event{
				timestamp: artefact.CreatedAtMs,
				artefact:  artefact,
			})
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan artefacts: %w", err)
	}

	// Scan for all claim keys
	claimPattern := fmt.Sprintf("holt:%s:claim:*", instanceName)
	iter = client.RedisClient().Scan(ctx, 0, claimPattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()

		// Extract claim ID from key
		claimPrefix := fmt.Sprintf("holt:%s:claim:", instanceName)
		if len(key) <= len(claimPrefix) {
			continue
		}
		claimID := key[len(claimPrefix):]

		// Fetch claim
		claim, err := client.GetClaim(ctx, claimID)
		if err != nil {
			// Skip malformed claims
			continue
		}

		// Claims don't have timestamps, but we can infer from associated artefact
		// For now, add all claims (filters don't apply to claims)
		events = append(events, event{
			timestamp: 0, // Claims don't have creation time, will sort last
			claim:     claim,
		})
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan claims: %w", err)
	}

	// Sort by creation time (oldest first), claims with no timestamp come last
	sort.Slice(events, func(i, j int) bool {
		if events[i].timestamp == 0 && events[j].timestamp == 0 {
			return false // Keep original order for claims
		}
		if events[i].timestamp == 0 {
			return false // Claims go last
		}
		if events[j].timestamp == 0 {
			return true // Claims go last
		}
		return events[i].timestamp < events[j].timestamp
	})

	// Format and output each event
	for _, evt := range events {
		if evt.artefact != nil {
			if err := formatter.FormatArtefact(evt.artefact); err != nil {
				log.Printf("⚠️  Failed to format historical artefact: %v", err)
			}
		} else if evt.claim != nil {
			if err := formatter.FormatClaim(evt.claim); err != nil {
				log.Printf("⚠️  Failed to format historical claim: %v", err)
			}
		}
	}

	return nil
}

// streamWithSubscriptions creates subscriptions and streams events until error or cancellation
func streamWithSubscriptions(ctx context.Context, client *blackboard.Client, formatter eventFormatter, filters *FilterCriteria, exitOnCompletion bool) error {
	// Subscribe to all three channels
	artefactSub, err := client.SubscribeArtefactEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to artefact events: %w", err)
	}
	defer artefactSub.Close()

	claimSub, err := client.SubscribeClaimEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to claim events: %w", err)
	}
	defer claimSub.Close()

	workflowSub, err := client.SubscribeWorkflowEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to workflow events: %w", err)
	}
	defer workflowSub.Close()

	// Stream events from all channels
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case artefact, ok := <-artefactSub.Events():
			if !ok {
				return fmt.Errorf("artefact events channel closed")
			}

			// Apply filters
			if filters != nil && !filters.matchesFilter(artefact) {
				continue
			}

			// Format and output artefact
			if err := formatter.FormatArtefact(artefact); err != nil {
				log.Printf("⚠️  Failed to format artefact event: %v", err)
			}

			// Check for Terminal artefact if exitOnCompletion is enabled
			if exitOnCompletion && artefact.StructuralType == blackboard.StructuralTypeTerminal {
				return nil // Clean exit
			}

		case claim, ok := <-claimSub.Events():
			if !ok {
				return fmt.Errorf("claim events channel closed")
			}
			if err := formatter.FormatClaim(claim); err != nil {
				log.Printf("⚠️  Failed to format claim event: %v", err)
			}

		case workflow, ok := <-workflowSub.Events():
			if !ok {
				return fmt.Errorf("workflow events channel closed")
			}
			if err := formatter.FormatWorkflow(workflow); err != nil {
				log.Printf("⚠️  Failed to format workflow event: %v", err)
			}

		case err := <-artefactSub.Errors():
			log.Printf("⚠️  Failed to parse artefact event: %v", err)

		case err := <-claimSub.Errors():
			log.Printf("⚠️  Failed to parse claim event: %v", err)

		case err := <-workflowSub.Errors():
			log.Printf("⚠️  Failed to parse workflow event: %v", err)
		}
	}
}

// reconnectWithRetry attempts to reconnect to Redis with retries
func reconnectWithRetry(ctx context.Context, client *blackboard.Client, retryInterval time.Duration) error {
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := client.Ping(ctx); err == nil {
				return nil
			}
			// Continue retrying
		}
	}
}

// eventFormatter formats events for output
type eventFormatter interface {
	FormatArtefact(artefact *blackboard.Artefact) error
	FormatClaim(claim *blackboard.Claim) error
	FormatWorkflow(event *blackboard.WorkflowEvent) error
}

// defaultFormatter produces human-readable output with emojis
type defaultFormatter struct {
	writer io.Writer
}

func (f *defaultFormatter) FormatArtefact(artefact *blackboard.Artefact) error {
	// Filter out Review artefacts - they're shown via review_approved/review_rejected events
	if artefact.StructuralType == blackboard.StructuralTypeReview {
		return nil
	}

	// Filter out reworked artefacts (version > 1) - they're shown via artefact_reworked events
	if artefact.Version > 1 {
		return nil
	}

	timestamp := time.Now().Format("15:04:05.000") // M3.9: Millisecond precision

	// Special handling for Terminal artefacts - indicate workflow completion
	if artefact.StructuralType == blackboard.StructuralTypeTerminal {
		_, err := fmt.Fprintf(f.writer, "[%s] ✨ Artefact created: by=%s, type=%s, id=%s\n",
			timestamp, artefact.ProducedByRole, artefact.Type, artefact.ID)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f.writer, "[%s] 🎉 Workflow completed: Terminal artefact created (type=%s, id=%s)\n",
			timestamp, artefact.Type, artefact.ID)
		return err
	}

	_, err := fmt.Fprintf(f.writer, "[%s] ✨ Artefact created: by=%s, type=%s, id=%s\n",
		timestamp, artefact.ProducedByRole, artefact.Type, artefact.ID)
	return err
}

func (f *defaultFormatter) FormatClaim(claim *blackboard.Claim) error {
	timestamp := time.Now().Format("15:04:05.000") // M3.9: Millisecond precision
	_, err := fmt.Fprintf(f.writer, "[%s] ⏳ Claim created: claim=%s, artefact=%s, status=%s\n",
		timestamp, claim.ID, claim.ArtefactID, claim.Status)
	return err
}

func (f *defaultFormatter) FormatWorkflow(event *blackboard.WorkflowEvent) error {
	timestamp := time.Now().Format("15:04:05.000") // M3.9: Millisecond precision

	switch event.Event {
	case "bid_submitted":
		agentName, _ := event.Data["agent_name"].(string)
		claimID, _ := event.Data["claim_id"].(string)
		bidType, _ := event.Data["bid_type"].(string)
		_, err := fmt.Fprintf(f.writer, "[%s] 🙋 Bid submitted: agent=%s, claim=%s, type=%s\n",
			timestamp, agentName, claimID, bidType)
		return err

	case "claim_granted":
		agentName, _ := event.Data["agent_name"].(string)
		claimID, _ := event.Data["claim_id"].(string)
		grantType, _ := event.Data["grant_type"].(string)
		agentImageID, _ := event.Data["agent_image_id"].(string) // M3.9

		// M3.9: Display agent@imageID format
		agentDisplay := agentName
		if agentImageID != "" {
			agentDisplay = fmt.Sprintf("%s@%s", agentName, truncateImageID(agentImageID))
		}

		_, err := fmt.Fprintf(f.writer, "[%s] 🏆 Claim granted: agent=%s, claim=%s, type=%s\n",
			timestamp, agentDisplay, claimID, grantType)
		return err

	case "review_approved":
		reviewerRole, _ := event.Data["reviewer_role"].(string)
		originalArtefactID, _ := event.Data["original_artefact_id"].(string)

		_, err := fmt.Fprintf(f.writer, "[%s] ✅ Review Approved: by=%s for artefact %s\n",
			timestamp, reviewerRole, originalArtefactID)
		return err

	case "review_rejected":
		reviewerRole, _ := event.Data["reviewer_role"].(string)
		originalArtefactID, _ := event.Data["original_artefact_id"].(string)

		_, err := fmt.Fprintf(f.writer, "[%s] ❌ Review Rejected: by=%s for artefact %s\n",
			timestamp, reviewerRole, originalArtefactID)
		return err

	case "feedback_claim_created":
		targetAgentRole, _ := event.Data["target_agent_role"].(string)
		feedbackClaimID, _ := event.Data["feedback_claim_id"].(string)
		iteration := 1 // default
		if iter, ok := event.Data["iteration"].(int); ok {
			iteration = iter
		} else if iterFloat, ok := event.Data["iteration"].(float64); ok {
			iteration = int(iterFloat)
		}

		_, err := fmt.Fprintf(f.writer, "[%s] 🔄 Rework Assigned: to=%s for claim %s (iteration %d)\n",
			timestamp, targetAgentRole, feedbackClaimID, iteration)
		return err

	case "artefact_reworked":
		producedByRole, _ := event.Data["produced_by_role"].(string)
		artefactType, _ := event.Data["artefact_type"].(string)
		newArtefactID, _ := event.Data["new_artefact_id"].(string)
		newVersion := 1 // default
		if ver, ok := event.Data["new_version"].(int); ok {
			newVersion = ver
		} else if verFloat, ok := event.Data["new_version"].(float64); ok {
			newVersion = int(verFloat)
		}

		_, err := fmt.Fprintf(f.writer, "[%s] 🔄 Artefact Reworked (v%d): by=%s, type=%s, id=%s\n",
			timestamp, newVersion, producedByRole, artefactType, newArtefactID)
		return err

	default:
		_, err := fmt.Fprintf(f.writer, "[%s] ❓ Unknown event: %s\n", timestamp, event.Event)
		return err
	}
}

// jsonlFormatter produces line-delimited JSON output (JSONL format)
type jsonlFormatter struct {
	writer io.Writer
}

func (f *jsonlFormatter) FormatArtefact(artefact *blackboard.Artefact) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     "artefact_created",
		"data":      artefact,
	}
	if err := f.writeJSON(output); err != nil {
		return err
	}

	// Add workflow_completed event for Terminal artefacts
	if artefact.StructuralType == blackboard.StructuralTypeTerminal {
		completionOutput := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"event":     "workflow_completed",
			"data": map[string]interface{}{
				"artefact_id":   artefact.ID,
				"artefact_type": artefact.Type,
				"produced_by":   artefact.ProducedByRole,
			},
		}
		return f.writeJSON(completionOutput)
	}

	return nil
}

func (f *jsonlFormatter) FormatClaim(claim *blackboard.Claim) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     "claim_created",
		"data":      claim,
	}
	return f.writeJSON(output)
}

func (f *jsonlFormatter) FormatWorkflow(event *blackboard.WorkflowEvent) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     event.Event,
		"data":      event.Data,
	}
	return f.writeJSON(output)
}

func (f *jsonlFormatter) writeJSON(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f.writer, "%s\n", string(bytes))
	return err
}

// truncateImageID shortens an image ID/digest for display (M3.9).
// Extracts first 12 characters of sha256 hash.
func truncateImageID(imageID string) string {
	// Handle "sha256:..." format
	if len(imageID) > 7 && imageID[:7] == "sha256:" {
		hash := imageID[7:]
		if len(hash) >= 12 {
			return hash[:12]
		}
		return hash
	}

	// Handle other formats
	if len(imageID) >= 12 {
		return imageID[:12]
	}

	return imageID
}
