# Grant Type Bug Fix - Claim Granted Events

## Problem

When a claim bid was granted during the **parallel phase**, the resulting `claim_granted` event was incorrectly reporting the grant type as **"review"** instead of **"claim"**.

### Example of the Bug
```
# Formatter bid with "claim" (parallel), but the grant type shows "review"
[21:38:01] 🏆 Claim granted: agent=Formatter, claim=b62edbc6-..., type=review  ❌ WRONG
```

### Root Cause

The `publishClaimGrantedEvent` function was **inferring** the grant type from the claim's fields:

```go
// OLD BUGGY CODE
func (e *Engine) publishClaimGrantedEvent(ctx context.Context, claim *blackboard.Claim, agentName string) error {
    // Detect grant type from claim fields
    var grantType string
    if claim.GrantedExclusiveAgent != "" {
        grantType = "exclusive"
    } else if len(claim.GrantedReviewAgents) > 0 {
        grantType = "review"  // ⚠️ BUG: This check happens BEFORE parallel check
    } else if len(claim.GrantedParallelAgents) > 0 {
        grantType = "parallel"
    }
    ...
}
```

**The Issue**: When transitioning from **review phase → parallel phase**, the claim object **still had the `GrantedReviewAgents` array populated** from the previous phase. This caused the function to incorrectly infer "review" instead of "claim".

The inference logic was checking grant arrays in this order:
1. ✅ Exclusive (empty)
2. ❌ **Review (populated from previous phase!)** ← Bug triggered here
3. ❓ Parallel (never reached)

---

## Solution

Refactored `publishClaimGrantedEvent` to accept the grant type as an **explicit parameter** instead of inferring it from claim state.

### New Signature

```go
// BEFORE
func (e *Engine) publishClaimGrantedEvent(ctx context.Context, claim *blackboard.Claim, agentName string) error

// AFTER
func (e *Engine) publishClaimGrantedEvent(ctx context.Context, claimID string, agentName string, grantType string) error
```

### Changes Made

1. **`internal/orchestrator/granting.go:111-129`**
   - Removed grant type inference logic
   - Changed signature to accept explicit `grantType` parameter
   - Simplified to just publish the provided grant type

2. **`internal/orchestrator/review_phase.go:50`**
   - Updated call: `publishClaimGrantedEvent(ctx, claim.ID, agentName, "review")`

3. **`internal/orchestrator/parallel_phase.go:50`**
   - Updated call: `publishClaimGrantedEvent(ctx, claim.ID, agentName, "claim")`

4. **`internal/orchestrator/phase_transitions.go:201, 245`**
   - Updated both calls (controller-worker and traditional):
   - `publishClaimGrantedEvent(ctx, claim.ID, winner, "exclusive")`

5. **`internal/orchestrator/engine_test.go:144, 171, 195, 220`**
   - Updated all test calls to use new signature with explicit grant types

---

## Benefits

1. **Eliminates Ambiguity**: Grant type is now explicitly declared by the caller
2. **Prevents State Pollution**: No longer affected by previously populated grant arrays
3. **More Explicit**: Makes the code intention clear at the call site
4. **Easier to Debug**: Grant type is hardcoded at source, not inferred

---

## Verification

### Test Results

All tests pass successfully:

✅ **`TestPublishClaimGrantedEvent`**
- `publishes_exclusive_grant_event` - Verifies "exclusive" type
- `publishes_review_grant_event` - Verifies "review" type
- `publishes_parallel_grant_event` - Verifies "claim" type (the fix!)
- `publishes_event_with_explicit_grant_type` - Verifies explicit types work

✅ **All orchestrator tests** (`internal/orchestrator/...`) - 100% passing

✅ **Build** - Clean compilation

### Expected Output (After Fix)

```
# Review phase
[21:38:01] 🏆 Claim granted: agent=Validator, claim=..., type=review ✅

# Parallel phase (FIXED!)
[21:38:01] 🏆 Claim granted: agent=Formatter, claim=..., type=claim ✅

# Exclusive phase
[21:38:01] 🏆 Claim granted: agent=Writer, claim=..., type=exclusive ✅
```

---

## Code Diff Summary

**Files Modified**: 5
- `internal/orchestrator/granting.go` (refactored function)
- `internal/orchestrator/review_phase.go` (updated call site)
- `internal/orchestrator/parallel_phase.go` (updated call site - THE FIX)
- `internal/orchestrator/phase_transitions.go` (updated 2 call sites)
- `internal/orchestrator/engine_test.go` (updated 4 test cases)

**Lines Changed**: ~30 lines

**Risk Level**: Low (improved clarity, all tests pass)

---

## Architecture Notes

This fix aligns with the **explicit over implicit** principle. By making grant types explicit parameters rather than inferred state, we:

1. Make the code more maintainable
2. Reduce coupling to claim state
3. Prevent future phase transition bugs
4. Improve code readability at call sites

The original inference logic was error-prone because claim objects accumulate state across phases, making inference unreliable during transitions.
