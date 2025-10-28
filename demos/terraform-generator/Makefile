# Makefile for M3.8 Terraform Module Generator Demo
# Must be run from the root of the 'holt' project repository

.PHONY: build-demo-terraform clean-demo-terraform

build-demo-terraform:
	@echo "Building TerraformDrafter agent (mocked LLM)..."
	@docker build -t holt-terraformdrafter-tf-demo:latest -f demos/terraform-generator/agents/terraform-drafter/Dockerfile .
	@echo "Building TerraformFmt agent (tool-based reviewer)..."
	@docker build -t holt-terraformfmt-tf-demo:latest -f demos/terraform-generator/agents/terraform-fmt/Dockerfile .
	@echo "Building TfLint agent (tool-based reviewer)..."
	@docker build -t holt-tflint-tf-demo:latest -f demos/terraform-generator/agents/tflint/Dockerfile .
	@echo "Building DocGenerator agent (mocked LLM)..."
	@docker build -t holt-docgenerator-tf-demo:latest -f demos/terraform-generator/agents/doc-generator/Dockerfile .
	@echo "Building MarkdownLint agent (tool-based parallel worker)..."
	@docker build -t holt-markdownlint-tf-demo:latest -f demos/terraform-generator/agents/markdown-lint/Dockerfile .
	@echo "Building ModulePackager agent (tool-based terminal)..."
	@docker build -t holt-modulepackager-tf-demo:latest -f demos/terraform-generator/agents/module-packager/Dockerfile .
	@echo "✅ All Terraform demo images built successfully."

clean-demo-terraform:
	@echo "Removing Terraform demo images..."
	@docker rmi -f holt-terraformdrafter-tf-demo:latest || true
	@docker rmi -f holt-terraformfmt-tf-demo:latest || true
	@docker rmi -f holt-tflint-tf-demo:latest || true
	@docker rmi -f holt-docgenerator-tf-demo:latest || true
	@docker rmi -f holt-markdownlint-tf-demo:latest || true
	@docker rmi -f holt-modulepackager-tf-demo:latest || true
	@echo "✅ Terraform demo images removed."
