package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/dyluth/holt/pkg/blackboard"
)

// OutputFormat defines the output format for watch streaming
type OutputFormat string

const (
	OutputFormatDefault OutputFormat = "default"
	OutputFormatJSON    OutputFormat = "json"
)

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

// StreamActivity streams workflow events to the provided writer.
// Subscribes to artefact_events, claim_events, and workflow_events channels.
// Handles reconnection on transient failures with 2s retry interval and 60s timeout.
func StreamActivity(ctx context.Context, client *blackboard.Client, instanceName string, format OutputFormat, writer io.Writer) error {
	// Create formatter
	var formatter eventFormatter
	switch format {
	case OutputFormatJSON:
		formatter = &jsonFormatter{writer: writer}
	default:
		formatter = &defaultFormatter{writer: writer}
	}

	// Subscribe to all channels with reconnection logic
	for {
		err := streamWithSubscriptions(ctx, client, formatter)
		if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
			// Clean exit
			return err
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

// streamWithSubscriptions creates subscriptions and streams events until error or cancellation
func streamWithSubscriptions(ctx context.Context, client *blackboard.Client, formatter eventFormatter) error {
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
			if err := formatter.FormatArtefact(artefact); err != nil {
				log.Printf("⚠️  Failed to format artefact event: %v", err)
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

// jsonFormatter produces line-delimited JSON output
type jsonFormatter struct {
	writer io.Writer
}

func (f *jsonFormatter) FormatArtefact(artefact *blackboard.Artefact) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     "artefact_created",
		"data":      artefact,
	}
	return f.writeJSON(output)
}

func (f *jsonFormatter) FormatClaim(claim *blackboard.Claim) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     "claim_created",
		"data":      claim,
	}
	return f.writeJSON(output)
}

func (f *jsonFormatter) FormatWorkflow(event *blackboard.WorkflowEvent) error {
	output := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"event":     event.Event,
		"data":      event.Data,
	}
	return f.writeJSON(output)
}

func (f *jsonFormatter) writeJSON(data interface{}) error {
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
