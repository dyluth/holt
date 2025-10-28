#!/bin/bash
# Build all agent images for the Terraform Module Generator demo
# Must be run from the root of the Holt project repository

set -e

echo "=========================================="
echo "Building Terraform Generator Demo Agents"
echo "=========================================="
echo ""

# Check we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "demos/terraform-generator" ]; then
    echo "❌ ERROR: This script must be run from the root of the Holt project repository"
    echo "   Current directory: $(pwd)"
    exit 1
fi

# Use the Makefile to build all images
make -f demos/terraform-generator/Makefile build-demo-terraform

echo ""
echo "=========================================="
echo "✅ All agent images built successfully"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  Option A: Run automated demo"
echo "    ./demos/terraform-generator/run-demo.sh"
echo ""
echo "  Option B: Manual setup"
echo "    1. Create workspace: mkdir /tmp/holt-terraform-demo && cd /tmp/holt-terraform-demo"
echo "    2. Initialize git: git init && git config user.email 'demo@example.com' && git config user.name 'Demo' && git commit --allow-empty -m 'init'"
echo "    3. Copy COMPLETE demo assets (CRITICAL):"
echo "       cp -r $(pwd)/demos/terraform-generator/agents ."
echo "       cp $(pwd)/demos/terraform-generator/holt.yml ."
echo "       git add . && git commit -m 'Add Holt config and agents'"
echo "    4. Initialize and start: holt init && holt up"
echo "    5. Run workflow: holt forage --goal 'Create a Terraform module for S3 static website hosting'"
