#!/bin/bash
# One-command demo execution script
# Builds images, creates workspace, and runs the complete workflow

set -e

HOLT_REPO=$(pwd)
DEMO_WORKSPACE="/tmp/holt-terraform-demo-$$"

echo "=========================================="
echo "Terraform Module Generator Demo"
echo "=========================================="
echo ""

# Check we're in the Holt project root
if [ ! -f "go.mod" ] || [ ! -d "demos/terraform-generator" ]; then
    echo "❌ ERROR: This script must be run from the root of the Holt project repository"
    exit 1
fi

# Step 1: Build agent images
echo "Step 1/6: Building agent images..."
./demos/terraform-generator/build-all.sh

# Step 2: Create demo workspace
echo ""
echo "Step 2/6: Creating demo workspace at $DEMO_WORKSPACE..."
mkdir -p "$DEMO_WORKSPACE"
cd "$DEMO_WORKSPACE"

# Step 3: Initialize git repository
echo ""
echo "Step 3/6: Initializing git repository..."
git init
git config user.email "demo@holt.example"
git config user.name "Holt Demo User"
git commit --allow-empty -m "Initial commit"

# Step 4: Copy all demo assets
echo ""
echo "Step 4/6: Copying all demo assets into workspace..."
# Copy the entire demo directory structure (excluding scripts and markdown)
cp -r "$HOLT_REPO/demos/terraform-generator/agents" .
cp "$HOLT_REPO/demos/terraform-generator/holt.yml" .

# Commit the demo assets to git (required for clean workspace check)
git add .
git commit -m "Add Holt configuration and agents"

# Step 5: Initialize Holt
echo ""
echo "Step 5/6: Initializing Holt instance..."
holt init

# Step 6: Start Holt
echo ""
echo "Step 6/6: Starting Holt instance..."
holt up --force

echo ""
echo "=========================================="
echo "✅ Demo setup complete!"
echo "=========================================="
echo ""
echo "Demo workspace: $DEMO_WORKSPACE"
echo ""
echo "Running workflow in 3 seconds..."
sleep 3

# Run the workflow
echo ""
echo "Submitting goal to Holt..."
holt forage --goal "Create a Terraform module to provision a basic S3 bucket for static website hosting"

echo ""
echo "Workflow submitted. Waiting for completion (max 60 seconds)..."
echo ""

# Wait for Terminal artefact (PackagedModule)
timeout 60 bash -c 'until holt hoard 2>/dev/null | grep -q "PackagedModule"; do sleep 2; done' || {
    echo "⚠️  WARNING: Workflow did not complete within 60 seconds"
    echo "   Check status with: holt watch"
    echo "   View logs with: holt logs <agent-name>"
}

# Check if package was created
if [ -f "s3-module.tar.gz" ]; then
    echo ""
    echo "=========================================="
    echo "✅ Workflow completed successfully!"
    echo "=========================================="
    echo ""
    echo "Package created: s3-module.tar.gz"
    echo ""
    echo "Package contents:"
    tar -tzf s3-module.tar.gz | sed 's/^/  - /'
    echo ""
    echo "View audit trail: holt hoard"
    echo "View git history: git log --oneline"
    echo "Extract package: tar -xzf s3-module.tar.gz"
    echo ""
    echo "Demo workspace preserved at: $DEMO_WORKSPACE"
    echo "Cleanup: holt down && cd /tmp && rm -rf $DEMO_WORKSPACE"
else
    echo ""
    echo "=========================================="
    echo "⚠️  Workflow may still be running"
    echo "=========================================="
    echo ""
    echo "Check status:"
    echo "  cd $DEMO_WORKSPACE"
    echo "  holt watch"
    echo "  holt hoard"
fi
