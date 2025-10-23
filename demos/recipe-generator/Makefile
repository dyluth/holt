# This Makefile builds all Docker images required for the Recipe Generator demo.
# It must be run from the root of the 'holt' project repository.
.PHONY: build-demo-recipe

build-demo-recipe:
	@echo "Building Recipe Drafter agent..."
	@docker build -t recipe-drafter-agent:latest -f demos/recipe-generator/agents/drafter/Dockerfile .
	@echo "Building Recipe Validator agent..."
	@docker build -t recipe-validator-agent:latest -f demos/recipe-generator/agents/validator/Dockerfile .
	@echo "Building Recipe Formatter agent..."
	@docker build -t recipe-formatter-agent:latest -f demos/recipe-generator/agents/formatter/Dockerfile .
	@echo "Recipe demo images built successfully."
