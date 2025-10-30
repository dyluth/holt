# Terraform Module Generator Demo (M3.8)

**Purpose**: Sophisticated demonstration of hybrid LLM-and-tool workflow for Infrastructure as Code
**Phase**: Phase 3 (Coordination) - Multi-agent orchestration with review gates and parallel transformations

## What This Demo Shows

This demo showcases Holt's ability to orchestrate a realistic DevOps workflow involving:
- **6 specialized agents** (2 LLM-based, 4 tool-based)
- **Multi-phase coordination** (review → exclusive → parallel → exclusive → terminal)
- **Automated feedback loop** (TerraformDrafter reworks code after review rejection)
- **Type-based workflow progression** (agents bid based on artefact types)
- **Validation gates** (both linters must approve before proceeding)
- **Complete audit trail** (every step traceable in git and blackboard)

### Workflow Overview

```
User Goal: "Create a Terraform module for S3 static website hosting"
    ↓
[TerraformDrafter] → Generates main.tf (v1, poorly formatted - DEMO)
    ↓
[TerraformFmt + TfLint] → Review phase → REJECT with feedback
    ↓
[TerraformDrafter] → Rework: Generates main.tf (v2, properly formatted)
    ↓
[TerraformFmt + TfLint] → Review phase → APPROVE
    ↓
[DocGenerator] → Generates README.md
    ↓
[MarkdownLint] → Parallel phase (formats documentation)
    ↓
[ModulePackager] → Creates s3-module.tar.gz (Terminal artefact)
```

**Note**: The first attempt deliberately generates poorly formatted code to demonstrate the automated feedback loop (M3.3). This shows how Holt handles review rejections and automatic rework without manual intervention.

## Quick Start

### Prerequisites

- Docker installed and running
- Git installed
- Holt CLI installed (`holt` command available)
- **(Optional)** OpenAI API key for real LLM generation

### Option 1: Automated (Recommended)

Use the provided script that handles all setup automatically:

```bash
cd <holt-repo>

# For mocked mode (no API key needed)
./demos/terraform-generator/run-demo.sh

# For real LLM mode (requires OpenAI API key)
export OPENAI_API_KEY="sk-..."
./demos/terraform-generator/run-demo.sh
```

### Option 2: Manual Setup

#### 1. Build Agent Images

From the Holt project root:

```bash
make -f demos/terraform-generator/Makefile build-demo-terraform
```

#### 2. Create Demo Workspace

```bash
mkdir /tmp/holt-terraform-demo && cd /tmp/holt-terraform-demo
git init
git config user.email "demo@example.com"
git config user.name "Demo User"
git commit --allow-empty -m "Initial commit"
```

#### 3. Copy Configuration

Copy the `holt.yml` configuration to your workspace:

```bash
HOLT_REPO=<path-to-holt-repo>
cp $HOLT_REPO/demos/terraform-generator/holt.yml .
git add holt.yml
git commit -m "Add Holt configuration"
```

**Note**: The agent scripts are baked into the Docker images, so you only need `holt.yml` in your workspace.

#### 4. Initialize and Run

```bash
holt init
holt up

# Optional: Set OpenAI API key for real LLM generation
export OPENAI_API_KEY="sk-..."

holt forage --goal "Create a Terraform module to provision a basic S3 bucket for static website hosting"
```

### 4. Watch and Inspect

```bash
# In another terminal
holt watch

# After completion
ls -l s3-module.tar.gz

# Extract to a new directory (files already exist in workspace)
mkdir extracted-module
tar -xzf s3-module.tar.gz -C extracted-module/
cat extracted-module/main.tf extracted-module/README.md

# View git history and audit trail
git log --oneline
holt hoard
```

## Automated Testing

```bash
cd <holt-repo>
./demos/terraform-generator/test-workflow.sh
```

## Troubleshooting

### "Error: Failed to load holt.yml: no such file or directory"

**Cause**: The `holt.yml` file is not present in your workspace.

**Solution**: Copy the `holt.yml` configuration file to your workspace:

```bash
cp /path/to/holt/demos/terraform-generator/holt.yml .
git add holt.yml
git commit -m "Add Holt configuration"
```

**Note**: Agent scripts are baked into the Docker images, so you don't need to copy the `agents/` directory to your workspace.

### "Git workspace is not clean"

**Cause**: `holt forage` requires all changes to be committed.

**Solution**: Commit any uncommitted files before running `holt forage`:

```bash
git add .
git commit -m "Your commit message"
```

### Workflow Hangs / Never Completes

**Cause**: One or more agents may have crashed or failed.

**Solution**: Check agent logs:

```bash
holt logs TerraformDrafter
holt logs TerraformFmt
holt logs TfLint
# ... check other agents

# View orchestrator logs
docker logs holt-orchestrator-<instance-name>
```

## Implementation Notes

### Dual-Mode LLM Operation

The `TerraformDrafter` and `DocGenerator` agents support **dual-mode operation**:

**Mocked Mode (Default)**:
- Uses hardcoded responses for deterministic testing
- No external API dependencies or costs
- Works offline
- First attempt deliberately generates poorly formatted code to demonstrate M3.3 feedback loop

**Real LLM Mode (OpenAI API)**:
- Activated when `OPENAI_API_KEY` environment variable is set
- Uses `gpt-4o-mini` model for cost-effective generation
- Generates actual Terraform code and documentation based on your goal
- Requires internet connection and valid API key
- Natural generation (may produce well-formatted code on first attempt)

**To enable real LLM mode:**
```bash
export OPENAI_API_KEY="sk-your-api-key-here"
holt up
holt forage --goal "your infrastructure goal"
```

The environment variable is passed through from the host environment via `holt.yml` configuration. The `${OPENAI_API_KEY}` placeholder in `holt.yml` is automatically expanded by Holt when creating containers.

### Git Workspace Management

Holt respects your branch workflow:

1. **Branch preservation**: All agents capture the original branch at startup
2. **Linear history**: Each agent updates the original branch pointer as it creates commits
3. **Final cleanup**: ModulePackager returns workspace to your starting branch

**Example**: If you start on `feature-branch`:
- All commits (TerraformDrafter, DocGenerator, MarkdownLint) update `feature-branch`
- After completion, you're back on `feature-branch` with all commits
- Git history is linear on your original branch

**Note**: Agents temporarily checkout commits to read previous work, but always update your original branch with new commits to maintain a clean, linear history.

**Future Enhancement**: For higher concurrency scenarios, git worktrees could provide better isolation. See `design/future-enhancements.md` for details.

---

For detailed documentation, architecture diagrams, and troubleshooting, see the full README in the repository.
