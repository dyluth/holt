package cub

import (
	"context"
	"fmt"
	"log"

	"github.com/dyluth/sett/pkg/blackboard"
)

const (
	// maxContextDepth is the hard limit for BFS traversal depth to prevent
	// infinite loops in malformed graphs and excessive context size.
	maxContextDepth = 10
)

// assembleContext performs breadth-first traversal of the artefact dependency graph
// to build a rich historical context for agent execution.
//
// Algorithm (from agent-cub.md):
//  1. Start with target artefact's source_artefacts
//  2. M3.3: Also add claim.AdditionalContextIDs for feedback claims
//  3. For each level (max 10):
//     - Fetch each artefact from blackboard
//     - Use thread tracking to get latest version of that logical artefact
//     - Store latest version in context map (de-duplicates by logical_id)
//     - Add source_artefacts to next level queue
//  4. Filter context to Standard and Answer artefacts only
//  5. Sort chronologically (oldest → newest)
//  6. Return filtered, sorted context chain
//
// Returns empty array for root artefacts (no source_artefacts).
func (e *Engine) assembleContext(ctx context.Context, targetArtefact *blackboard.Artefact, claim *blackboard.Claim) ([]*blackboard.Artefact, error) {
	log.Printf("[INFO] Assembling context for artefact: artefact_id=%s type=%s",
		targetArtefact.ID, targetArtefact.Type)

	// Initialize BFS queue with target's source artefacts
	queue := make([]string, len(targetArtefact.SourceArtefacts))
	copy(queue, targetArtefact.SourceArtefacts)

	// M3.3: Add additional context IDs for feedback claims
	if len(claim.AdditionalContextIDs) > 0 {
		queue = append(queue, claim.AdditionalContextIDs...)
		log.Printf("[INFO] Feedback claim detected, adding %d Review artefacts to context",
			len(claim.AdditionalContextIDs))
	}

	// Context map keyed by logical_id for de-duplication
	// Also serves as cache for GetLatestVersion results
	contextMap := make(map[string]*blackboard.Artefact)

	// Track depth to enforce limit
	depth := 0

	// Track seen logical_ids to avoid duplicates
	seenLogicalIDs := make(map[string]bool)

	// BFS traversal
	for len(queue) > 0 && depth < maxContextDepth {
		depth++
		currentLevelSize := len(queue)

		log.Printf("[DEBUG] BFS level %d: processing %d artefacts", depth, currentLevelSize)

		// Process all artefacts at current level
		for i := 0; i < currentLevelSize; i++ {
			artefactID := queue[0]
			queue = queue[1:] // Dequeue (pop from front)

			// Fetch artefact from blackboard
			artefact, err := e.bbClient.GetArtefact(ctx, artefactID)
			if err != nil {
				log.Printf("[WARN] Failed to fetch artefact %s: %v (skipping)", artefactID, err)
				continue // Skip this artefact, continue traversal
			}

			if artefact == nil {
				log.Printf("[WARN] Artefact %s not found (skipping)", artefactID)
				continue
			}

			// Get latest version of this logical artefact via thread tracking
			latestArtefact, err := e.getLatestVersionForContext(ctx, artefact)
			if err != nil {
				log.Printf("[WARN] Failed to get latest version for logical_id=%s: %v (using discovered version)",
					artefact.LogicalID, err)
				latestArtefact = artefact // Fallback to discovered version
			}

			// De-duplicate by logical_id (keep first occurrence in BFS order)
			if seenLogicalIDs[latestArtefact.LogicalID] {
				log.Printf("[DEBUG] De-duplication: logical_id=%s already in context, skipping",
					latestArtefact.LogicalID)
				continue
			}

			seenLogicalIDs[latestArtefact.LogicalID] = true
			contextMap[latestArtefact.LogicalID] = latestArtefact
			log.Printf("[DEBUG] Added to context: logical_id=%s version=%d type=%s",
				latestArtefact.LogicalID, latestArtefact.Version, latestArtefact.Type)

			// Add source artefacts to queue for next level
			queue = append(queue, latestArtefact.SourceArtefacts...)
		}
	}

	if len(queue) > 0 {
		log.Printf("[WARN] Depth limit reached: max_depth=%d artefacts_pending=%d",
			maxContextDepth, len(queue))
	}

	// Filter to Standard and Answer artefacts only
	filtered := filterContextArtefacts(contextMap)
	log.Printf("[DEBUG] Context filtering: total=%d filtered_to=%d",
		len(contextMap), len(filtered))

	// Sort chronologically (oldest → newest)
	sortedContext := sortContextChronologically(filtered)

	log.Printf("[DEBUG] Context assembly complete: total=%d depth=%d",
		len(sortedContext), depth)

	return sortedContext, nil
}

// getLatestVersionForContext retrieves the latest version of a logical artefact
// using thread tracking. Returns the discovered artefact if thread tracking fails
// or returns an older version.
func (e *Engine) getLatestVersionForContext(ctx context.Context, discoveredArtefact *blackboard.Artefact) (*blackboard.Artefact, error) {
	// Query thread tracking for latest version
	latestID, latestVersion, err := e.bbClient.GetLatestVersion(ctx, discoveredArtefact.LogicalID)
	if err != nil {
		return nil, fmt.Errorf("GetLatestVersion failed: %w", err)
	}

	// Thread tracking returned empty (no thread exists)
	if latestID == "" {
		log.Printf("[DEBUG] No thread tracking for logical_id=%s, using discovered version %d",
			discoveredArtefact.LogicalID, discoveredArtefact.Version)
		return discoveredArtefact, nil
	}

	// Thread tracking returned same or older version - use discovered
	if latestVersion <= discoveredArtefact.Version {
		log.Printf("[DEBUG] Thread has version %d, discovered version %d, using discovered",
			latestVersion, discoveredArtefact.Version)
		return discoveredArtefact, nil
	}

	// Thread tracking found a newer version - fetch it
	log.Printf("[DEBUG] Found latest version: logical_id=%s version=%d (discovered was %d)",
		discoveredArtefact.LogicalID, latestVersion, discoveredArtefact.Version)

	latestArtefact, err := e.bbClient.GetArtefact(ctx, latestID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest version: %w", err)
	}

	if latestArtefact == nil {
		return nil, fmt.Errorf("latest version artefact not found: %s", latestID)
	}

	return latestArtefact, nil
}

// filterContextArtefacts filters the context map to include only Standard, Answer, and Review artefacts.
// M3.3: Review artefacts are included for feedback claims to provide review feedback to agents.
// This provides agents with a clean, actionable history without failures or terminal artefacts.
func filterContextArtefacts(contextMap map[string]*blackboard.Artefact) []*blackboard.Artefact {
	filtered := make([]*blackboard.Artefact, 0, len(contextMap))

	for _, artefact := range contextMap {
		if artefact.StructuralType == blackboard.StructuralTypeStandard ||
			artefact.StructuralType == blackboard.StructuralTypeAnswer ||
			artefact.StructuralType == blackboard.StructuralTypeReview {
			filtered = append(filtered, artefact)
		} else {
			log.Printf("[DEBUG] Filtered out artefact: logical_id=%s type=%s structural_type=%s",
				artefact.LogicalID, artefact.Type, artefact.StructuralType)
		}
	}

	return filtered
}

// sortContextChronologically sorts artefacts to provide chronological ordering.
// Since Artefact structs don't have timestamps in Phase 2, we use BFS traversal order
// as a proxy for chronological order. The graph structure (source_artefacts relationships)
// implicitly encodes chronological dependencies: if A → B, then A was created before B.
//
// For Phase 2 with single-agent linear chains, BFS order is equivalent to chronological order.
// Future phases may add explicit timestamps if needed for complex multi-agent scenarios.
func sortContextChronologically(artefacts []*blackboard.Artefact) []*blackboard.Artefact {
	// In Phase 2, artefacts are already in BFS traversal order, which is chronologically
	// correct for linear chains. We reverse the order so oldest artefacts come first.
	sorted := make([]*blackboard.Artefact, len(artefacts))

	// BFS discovers newest artefacts first (closest to target), so reverse the array
	// to get oldest-first ordering
	for i := 0; i < len(artefacts); i++ {
		sorted[i] = artefacts[len(artefacts)-1-i]
	}

	return sorted
}
