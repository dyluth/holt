#!/bin/bash
# E2E test script for Terraform Module Generator Demo
# Tests complete workflow from goal to packaged module

set -e

HOLT_REPO=$(pwd)
TEST_WORKSPACE="/tmp/holt-terraform-test-$$"
TEST_FAILED=0

echo "=========================================="
echo "Terraform Generator Demo E2E Test"
echo "=========================================="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up test environment..."
    cd /tmp
    if [ -d "$TEST_WORKSPACE" ]; then
        cd "$TEST_WORKSPACE"
        holt down 2>/dev/null || true
        cd /tmp
        rm -rf "$TEST_WORKSPACE"
    fi

    if [ $TEST_FAILED -eq 0 ]; then
        echo "✅ All tests passed!"
        exit 0
    else
        echo "❌ Tests failed!"
        exit 1
    fi
}

trap cleanup EXIT

# Check we're in the Holt project root
if [ ! -f "go.mod" ] || [ ! -d "demos/terraform-generator" ]; then
    echo "❌ ERROR: This script must be run from the root of the Holt project repository"
    TEST_FAILED=1
    exit 1
fi

# Test 1: Build all agent images
echo "Test 1/10: Building agent images..."
if ! ./demos/terraform-generator/build-all.sh > /dev/null 2>&1; then
    echo "❌ FAILED: Could not build agent images"
    TEST_FAILED=1
    exit 1
fi
echo "✅ PASSED: All agent images built successfully"

# Test 2: Create test workspace
echo ""
echo "Test 2/10: Creating test workspace..."
mkdir -p "$TEST_WORKSPACE"
cd "$TEST_WORKSPACE"
echo "✅ PASSED: Test workspace created at $TEST_WORKSPACE"

# Test 3: Initialize git repository
echo ""
echo "Test 3/10: Initializing git repository..."
git init > /dev/null 2>&1
git config user.email "test@holt.example"
git config user.name "Holt Test"
git commit --allow-empty -m "Initial commit" > /dev/null 2>&1
echo "✅ PASSED: Git repository initialized"

# Test 4: Initialize Holt
echo ""
echo "Test 4/10: Initializing Holt..."
if ! holt init > /dev/null 2>&1; then
    echo "❌ FAILED: holt init failed"
    TEST_FAILED=1
    exit 1
fi
echo "✅ PASSED: Holt initialized"

# Test 5: Copy all demo assets
echo ""
echo "Test 5/10: Copying all demo assets..."
cp -r "$HOLT_REPO/demos/terraform-generator/agents" .
cp "$HOLT_REPO/demos/terraform-generator/holt.yml" .
if [ ! -f "holt.yml" ] || [ ! -d "agents" ]; then
    echo "❌ FAILED: Could not copy demo assets"
    TEST_FAILED=1
    exit 1
fi

# Commit demo assets (required for clean workspace check)
git add .
git commit -m "Add Holt configuration and agents" > /dev/null 2>&1

echo "✅ PASSED: Demo assets copied and committed"

# Test 6: Start Holt instance
echo ""
echo "Test 6/10: Starting Holt instance..."
if ! holt up --force > /dev/null 2>&1; then
    echo "❌ FAILED: holt up failed"
    TEST_FAILED=1
    exit 1
fi
sleep 5  # Give services time to start
echo "✅ PASSED: Holt instance started"

# Test 7: Submit workflow
echo ""
echo "Test 7/10: Submitting workflow..."
forage_output=$(holt forage --goal "Create a Terraform module to provision a basic S3 bucket for static website hosting" 2>&1)
forage_exit=$?
if [ $forage_exit -ne 0 ]; then
    echo "❌ FAILED: holt forage failed with exit code $forage_exit"
    echo "Output: $forage_output"
    TEST_FAILED=1
    exit 1
fi
echo "✅ PASSED: Workflow submitted"

# Test 8: Wait for workflow completion
echo ""
echo "Test 8/10: Waiting for workflow completion (max 90 seconds)..."
if ! timeout 90 bash -c 'until holt hoard 2>/dev/null | grep -q "PackagedModule"; do sleep 2; done'; then
    echo "❌ FAILED: Workflow did not complete within 90 seconds"
    echo ""
    echo "Debug information:"
    echo "--- holt hoard ---"
    holt hoard || true
    echo ""
    echo "--- Agent logs ---"
    holt logs TerraformDrafter | tail -20 || true
    TEST_FAILED=1
    exit 1
fi
echo "✅ PASSED: Workflow completed"

# Test 9: Verify package created
echo ""
echo "Test 9/10: Verifying package contents..."
if [ ! -f "s3-module.tar.gz" ]; then
    echo "❌ FAILED: Package s3-module.tar.gz not created"
    TEST_FAILED=1
    exit 1
fi

# Extract and validate package contents
tar -xzf s3-module.tar.gz

if [ ! -f "main.tf" ]; then
    echo "❌ FAILED: main.tf missing from package"
    TEST_FAILED=1
    exit 1
fi

if [ ! -f "README.md" ]; then
    echo "❌ FAILED: README.md missing from package"
    TEST_FAILED=1
    exit 1
fi

# Validate Terraform code contains expected resources
if ! grep -q "aws_s3_bucket" main.tf; then
    echo "❌ FAILED: main.tf missing S3 bucket resource"
    TEST_FAILED=1
    exit 1
fi

# Validate README contains expected sections
if ! grep -q "Usage" README.md; then
    echo "❌ FAILED: README.md missing Usage section"
    TEST_FAILED=1
    exit 1
fi

echo "✅ PASSED: Package contains valid main.tf and README.md"

# Test 10: Verify audit trail
echo ""
echo "Test 10/10: Verifying audit trail..."
audit_trail=$(holt hoard 2>/dev/null)

if ! echo "$audit_trail" | grep -q "GoalDefined"; then
    echo "❌ FAILED: Missing GoalDefined artefact"
    TEST_FAILED=1
    exit 1
fi

if ! echo "$audit_trail" | grep -q "TerraformCode"; then
    echo "❌ FAILED: Missing TerraformCode artefact"
    TEST_FAILED=1
    exit 1
fi

if ! echo "$audit_trail" | grep -q "Review"; then
    echo "❌ FAILED: Missing Review artefacts"
    TEST_FAILED=1
    exit 1
fi

if ! echo "$audit_trail" | grep -q "TerraformDocumentation"; then
    echo "❌ FAILED: Missing TerraformDocumentation artefact"
    TEST_FAILED=1
    exit 1
fi

if ! echo "$audit_trail" | grep -q "FormattedDocumentation"; then
    echo "❌ FAILED: Missing FormattedDocumentation artefact"
    TEST_FAILED=1
    exit 1
fi

if ! echo "$audit_trail" | grep -q "PackagedModule"; then
    echo "❌ FAILED: Missing PackagedModule (Terminal) artefact"
    TEST_FAILED=1
    exit 1
fi

echo "✅ PASSED: Complete audit trail verified"

# Additional validation: Check git history
git_commits=$(git log --oneline | wc -l)
if [ "$git_commits" -lt 3 ]; then
    echo "⚠️  WARNING: Expected at least 3 git commits, found $git_commits"
fi

echo ""
echo "=========================================="
echo "Summary:"
echo "=========================================="
echo "Package size: $(ls -lh s3-module.tar.gz | awk '{print $5}')"
echo "Git commits: $git_commits"
echo "Artefacts created: $(echo "$audit_trail" | grep -c "id:" || echo "unknown")"
echo ""
