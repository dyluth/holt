# Terraform Module Generator Demo (M3.8)

**Purpose**: Sophisticated demonstration of hybrid LLM-and-tool workflow for Infrastructure as Code
**Phase**: Phase 3 (Coordination) - Multi-agent orchestration with review gates and parallel transformations

## What This Demo Shows

This demo showcases Holt's ability to orchestrate a realistic DevOps workflow involving:
- **6 specialized agents** (2 LLM-based, 4 tool-based)
- **Multi-phase coordination** (review → exclusive → parallel → exclusive → terminal)
- **Type-based workflow progression** (agents bid based on artefact types)
- **Validation gates** (both linters must approve before proceeding)
- **Complete audit trail** (every step traceable in git and blackboard)

### Workflow Overview

```
User Goal: "Create a Terraform module for S3 static website hosting"
    ↓
[TerraformDrafter] → Generates main.tf
    ↓
[TerraformFmt + TfLint] → Review phase (both must approve)
    ↓
[DocGenerator] → Generates README.md
    ↓
[MarkdownLint] → Parallel phase (formats documentation)
    ↓
[ModulePackager] → Creates s3-module.tar.gz (Terminal artefact)
```

## Quick Start

### Option 1: Automated (Recommended)

Use the provided script that handles all setup automatically:

```bash
cd <holt-repo>
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

#### 3. Copy Complete Demo Assets

**CRITICAL**: Copy the entire demo structure, not just holt.yml:

```bash
HOLT_REPO=<path-to-holt-repo>
cp -r $HOLT_REPO/demos/terraform-generator/agents .
cp $HOLT_REPO/demos/terraform-generator/holt.yml .
git add .
git commit -m "Add Holt configuration and agents"
```

#### 4. Initialize and Run

```bash
holt init
holt up
holt forage --goal "Create a Terraform module to provision a basic S3 bucket for static website hosting"
```

### 4. Watch and Inspect

```bash
# In another terminal
holt watch

# After completion
ls -l s3-module.tar.gz
tar -xzf s3-module.tar.gz
cat main.tf README.md
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

**Cause**: The orchestrator cannot find the agent Dockerfiles in the workspace.

**Solution**: Ensure you copied the **complete demo structure** (both `agents/` directory and `holt.yml`), not just `holt.yml`:

```bash
# WRONG - only copies holt.yml
cp /path/to/holt/demos/terraform-generator/holt.yml .

# CORRECT - copies complete structure
cp -r /path/to/holt/demos/terraform-generator/agents .
cp /path/to/holt/demos/terraform-generator/holt.yml .
git add .
git commit -m "Add Holt configuration and agents"
```

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

## Implementation Note

**Current Status**: Uses **mocked LLM responses** (hardcoded Terraform and README content) for deterministic testing. This validates the complete 6-agent orchestration without external API dependencies.

**Future Enhancement**: Replace mocked agents with real OpenAI API calls once orchestration is proven.

---

For detailed documentation, architecture diagrams, and troubleshooting, see the full README in the repository.
