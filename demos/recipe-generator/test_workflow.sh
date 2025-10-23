#!/bin/bash
# Integration test for recipe-generator demo
# Tests the complete M3.3 feedback workflow with dynamic bidding

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Recipe Generator Demo Integration Test ===${NC}"
echo ""

# Configuration
TEST_DIR="/tmp/holt-recipe-test-$$"
HOLT_BIN="${HOLT_BIN:-$(pwd)/bin/holt}"
DEMO_DIR="$(cd "$(dirname "$0")" && pwd)"

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up test environment...${NC}"
    cd /tmp
    if [ -d "$TEST_DIR" ]; then
        cd "$TEST_DIR"
        if [ -f ".holt/instance_name" ]; then
            INSTANCE_NAME=$(cat .holt/instance_name)
            $HOLT_BIN down --name "$INSTANCE_NAME" 2>/dev/null || true
        fi
        rm -rf "$TEST_DIR"
    fi
}

trap cleanup EXIT

# Test step function
test_step() {
    local step_num=$1
    local description=$2
    echo -e "${GREEN}[Step $step_num]${NC} $description"
}

# Assert function
assert() {
    local condition=$1
    local message=$2
    if ! eval "$condition"; then
        echo -e "${RED}✗ FAILED:${NC} $message"
        exit 1
    fi
    echo -e "${GREEN}✓ PASSED:${NC} $message"
}

# Main test flow
main() {
    test_step 1 "Creating test workspace"
    mkdir -p "$TEST_DIR"
    cd "$TEST_DIR"
    git init --quiet
    git config user.email "test@holt.example"
    git config user.name "Test User"
    git commit --allow-empty -m "Initial commit" --quiet
    assert "[ -d .git ]" "Git repository initialized"

    test_step 2 "Initializing Holt"
    $HOLT_BIN init
    assert "[ -d .holt ]" "Holt initialized"

    test_step 3 "Copying demo configuration"
    cp "$DEMO_DIR/holt.yml" .
    assert "[ -f holt.yml ]" "Demo configuration copied"

    test_step 4 "Starting Holt instance"
    $HOLT_BIN up --name recipe-test
    sleep 3
    assert "[ -f .holt/instance_name ]" "Holt instance started"

    test_step 5 "Running workflow: Create recipe for spaghetti bolognese"
    $HOLT_BIN forage --goal "Create a recipe for a classic spaghetti bolognese" --name recipe-test &
    FORAGE_PID=$!

    # Wait for workflow to complete (max 60 seconds)
    echo "Waiting for workflow to complete..."
    for i in {1..60}; do
        if [ -f "RECIPE.md" ]; then
            echo "✓ Workflow completed in ${i} seconds"
            break
        fi
        if [ $i -eq 60 ]; then
            echo -e "${RED}✗ Workflow did not complete in 60 seconds${NC}"
            kill $FORAGE_PID 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done

    test_step 6 "Verifying artefacts created"
    assert "[ -f recipe.yaml ]" "recipe.yaml exists"
    assert "[ -f RECIPE.md ]" "RECIPE.md exists"

    test_step 7 "Verifying git commits"
    COMMIT_COUNT=$(git log --oneline | wc -l)
    assert "[ $COMMIT_COUNT -ge 4 ]" "At least 4 commits exist (initial + draft v1 + draft v2 + format)"

    test_step 8 "Verifying recipe content"
    # Check that final recipe has improved instruction (not just "Cook.")
    assert "grep -q 'Simmer' recipe.yaml || grep -q 'simmer' recipe.yaml || grep -q 'minutes' recipe.yaml" \
        "Recipe contains detailed instructions (not vague 'Cook.')"

    test_step 9 "Verifying RECIPE.md format"
    assert "grep -q '^# ' RECIPE.md" "RECIPE.md has title"
    assert "grep -q 'Ingredients' RECIPE.md" "RECIPE.md has ingredients section"
    assert "grep -q 'Instructions' RECIPE.md" "RECIPE.md has instructions section"

    test_step 10 "Checking holt hoard output"
    HOARD_OUTPUT=$($HOLT_BIN hoard --name recipe-test)
    assert "echo '$HOARD_OUTPUT' | grep -q 'GoalDefined'" "GoalDefined artefact exists"
    assert "echo '$HOARD_OUTPUT' | grep -q 'RecipeYAML'" "RecipeYAML artefact exists"
    assert "echo '$HOARD_OUTPUT' | grep -q 'Review'" "Review artefact exists"
    assert "echo '$HOARD_OUTPUT' | grep -q 'RecipeMarkdown'" "RecipeMarkdown artefact exists"

    test_step 11 "Verifying feedback loop occurred"
    # Should have at least 2 RecipeYAML artefacts (v1 rejected, v2 approved)
    RECIPE_YAML_COUNT=$(echo "$HOARD_OUTPUT" | grep -c 'RecipeYAML' || true)
    assert "[ $RECIPE_YAML_COUNT -ge 2 ]" "Feedback loop occurred (multiple RecipeYAML versions)"

    test_step 12 "Stopping Holt instance"
    $HOLT_BIN down --name recipe-test
    sleep 2

    echo ""
    echo -e "${GREEN}=== All Tests Passed! ===${NC}"
    echo ""
    echo "Workflow successfully demonstrated:"
    echo "  1. Dynamic bidding (agents used bid scripts)"
    echo "  2. Review phase (validator reviewed RecipeYAML)"
    echo "  3. Feedback loop (drafter reworked after rejection)"
    echo "  4. Approval and formatting (formatter created RECIPE.md)"
    echo ""
}

# Run the test
main
