# This Makefile builds all Docker images required for the Recipe Generator demo.
# It must be run from the root of the 'holt' project repository.
# M3.7: Role-based image naming (holt-{role}-recipe-demo:latest)
.PHONY: build-demo-recipe

build-demo-recipe:
	@echo "Building Writer agent (Recipe Drafter)..."
	@docker build -t holt-writer-recipe-demo:latest -f demos/recipe-generator/agents/drafter/Dockerfile .
	@echo "Building Validator agent..."
	@docker build -t holt-validator-recipe-demo:latest -f demos/recipe-generator/agents/validator/Dockerfile .
	@echo "Building Formatter agent..."
	@docker build -t holt-formatter-recipe-demo:latest -f demos/recipe-generator/agents/formatter/Dockerfile .
	@echo "Recipe demo images built successfully."
