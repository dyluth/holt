# Future Enhancements & Ideas

**Purpose**: Capture promising ideas and enhancements for future consideration
**Scope**: beyond the highest current phase - ideas that improve the system but aren't critical for initial release
**Status**: Living document - add ideas as they emerge

---

## Git Workspace Management

### Git Worktrees for Agent Isolation

**Context**: Currently, agents share a single workspace and use `git checkout <commit>` to work on specific commits. This can leave the workspace in detached HEAD state and requires careful coordination.

**Idea**: Use git worktrees to give each agent an isolated workspace:

```bash
# Agent creates temporary worktree
worktree_path="/workspace/.holt-worktrees/$AGENT_NAME-$CLAIM_ID"
git worktree add "$worktree_path" "$commit_hash"
cd "$worktree_path"

# Work on files in isolation
# ... agent work ...

# Cleanup
cd /workspace
git worktree remove "$worktree_path"
```

**Benefits**:
- **True isolation**: Multiple agents can work on different commits simultaneously without conflicts
- **Cleaner workspace state**: Main workspace stays on its original branch
- **Safer concurrent execution**: No risk of one agent's checkout interfering with another
- **Better debugging**: Each worktree is independent, easier to inspect

**Challenges**:
- **Complexity**: Agents need to manage worktree lifecycle (create, work, cleanup)
- **Disk space**: Each worktree is a full checkout (though sharing git objects)
- **Path management**: Agents must be aware they're working in a subdirectory
- **Container mounts**: Need to ensure worktree paths are within mounted volume
- **Error handling**: Cleanup must be robust (what if agent crashes mid-work?)

**Current Solution**:
Terminal agents (e.g., ModulePackager) update the main branch pointer and checkout main after completion. This is simple and solves the detached HEAD UX issue for demos.

**When to Revisit**:
- When we need higher concurrency (e.g., dozens of parallel agents)
- When implementing advanced branching strategies (feature branches, PRs)
- When git state conflicts become a production issue
- Phase 5+ when scaling becomes a priority

**Design Considerations**:
- Should worktree management be in the pup, or left to agent scripts?
- How to handle cleanup on agent crash or timeout?
- Should we use a shared worktree pool, or create/destroy per-claim?
- What's the performance impact of worktree creation vs checkout?

---

## Template for New Ideas

When adding ideas, include:
1. **Context**: What problem does this solve?
2. **Idea**: High-level description of the enhancement
3. **Benefits**: Why is this valuable?
4. **Challenges**: What makes this hard?
5. **Current Solution**: How are we handling this now?
6. **When to Revisit**: Under what conditions should we implement this?
7. **Design Considerations**: Key questions to answer before implementing

---

*Last updated: 2025-10-29*
