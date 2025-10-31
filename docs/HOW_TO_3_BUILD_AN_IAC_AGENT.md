# How-To: Build an Infrastructure-as-Code Agent

**Purpose**: To explain the powerful `terraform-generator` demo, which shows how Holt can orchestrate a real-world Infrastructure-as-Code (IaC) workflow using a stateful tool (Terraform).

**Based on Demo**: [`/demos/terraform-generator/`](../demos/terraform-generator)

---

## 1. The Goal

This demo showcases Holt's ability to manage complex, multi-step workflows that involve external, stateful tools. The goal is to have an AI agent write, plan, and apply a Terraform configuration to create a real resource (in this case, a simple `null_resource`).

This demonstrates several advanced Holt concepts:

-   **Interaction with stateful CLIs** (Terraform)
-   **A multi-step, chained workflow** driven by a single agent
-   **Using the Blackboard to pass state** between agent executions
-   **A sophisticated agent** that changes its behavior based on the type of artefact it receives

## 2. The Agent & `holt.yml`

There is only one agent in this demo, the `terraform-generator`. Its job is to act as a Terraform expert, taking a high-level goal and turning it into applied infrastructure.

```yaml
version: "1.0"
agents:
  terraform-generator:
    build:
      context: ./agents/terraform-generator
    command: ["/usr/bin/python", "run.py"]
    workspace:
      mode: rw # Must be rw to write .tf files and manage terraform.tfstate
    environment:
      - OPENAI_API_KEY
```

-   **`workspace: rw`**: This is essential. The agent needs read-write access to write the `main.tf` file, and more importantly, so that Terraform can create and manage its state file (`terraform.tfstate`) within the workspace.

## 3. The Workflow: A State-Driven Agent

This agent is more advanced than the previous examples. It's a single agent that bids `exclusive` on multiple, sequential claims, effectively creating a chain of actions. Its behavior changes depending on the `type` of the artefact it receives.

Here is the sequence of events when you run `holt forage --goal "a null resource named 'example'"`:

1.  **Claim 1: `GoalDefined` Artefact**
    *   **Agent Action**: The agent receives the `GoalDefined` artefact. Its Python script identifies this `type` and knows its job is to **write** the Terraform code.
    *   It calls an LLM with a prompt to convert the goal into HCL (HashiCorp Configuration Language).
    *   It writes the received HCL into a `main.tf` file in the workspace.
    *   It produces a `TerraformCode` artefact, with the `payload` containing the HCL code it just wrote.

2.  **Claim 2: `TerraformCode` Artefact**
    *   **Agent Action**: The same `terraform-generator` agent now receives the `TerraformCode` artefact it just created. It inspects the `type` and knows its next job is to **plan** the execution.
    *   It runs the `terraform plan -out=tfplan` command inside the workspace.
    *   It produces a `TerraformPlan` artefact. The `payload` contains the text output of the plan.

3.  **Claim 3: `TerraformPlan` Artefact**
    *   **Agent Action**: The agent receives the `TerraformPlan` artefact. It inspects the `type` and knows its final job is to **apply** the plan.
    *   It runs the `terraform apply -auto-approve tfplan` command.
    *   It produces a final `TerraformApply` artefact with the output of the apply command.

This demonstrates how a single, intelligent agent can orchestrate a complex, stateful process by creating a chain of artefacts and reacting to them, using the Blackboard as its external state machine.

## 4. The Tool Script: `run.py`

The intelligence of this workflow is in the `run.py` script. It's a Python script that acts as a router.

```python
# Simplified pseudo-code for run.py

# Read input from stdin
input_json = read_stdin()
target_artefact = input_json["target_artefact"]
artefact_type = target_artefact["type"]

if artefact_type == "GoalDefined":
    # Call LLM to generate HCL code from the goal
    hcl_code = generate_hcl(target_artefact["payload"])
    # Write main.tf
    write_file("main.tf", hcl_code)
    # Output a TerraformCode artefact
    produce_artefact("TerraformCode", hcl_code, "Wrote Terraform HCL")

elif artefact_type == "TerraformCode":
    # Run terraform init and plan
    run_subprocess("terraform init")
    plan_output = run_subprocess("terraform plan -out=tfplan")
    # Output a TerraformPlan artefact
    produce_artefact("TerraformPlan", plan_output, "Planned Terraform execution")

elif artefact_type == "TerraformPlan":
    # Run terraform apply
    apply_output = run_subprocess("terraform apply -auto-approve tfplan")
    # Output a TerraformApply artefact
    produce_artefact("TerraformApply", apply_output, "Applied Terraform plan")

else:
    # Ignore other artefact types
    pass
```

This script demonstrates a powerful pattern: a single agent that can handle multiple steps in a workflow by inspecting the `type` of the incoming artefact and routing its logic accordingly.

## 5. How to Run This Demo

1.  **Set your API Key:**
    ```bash
    export OPENAI_API_KEY="your-key-here"
    ```
2.  **Navigate to the demo directory:**
    ```bash
    cd demos/terraform-generator
    ```
3.  **Run the build script:**
    This script builds the agent's Docker image.
    ```bash
    ./build-all.sh
    ```
4.  **Run the demo script:**
    This script starts Holt and runs the `forage` command.
    ```bash
    ./run-demo.sh
    ```
5.  **Watch the workflow:**
    In another terminal, run `holt watch`. You will see the three distinct phases of the workflow as the `TerraformCode`, `TerraformPlan`, and `TerraformApply` artefacts are created one after another.
6.  **Verify the result:**
    Check the `terraform.tfstate` file in the directory. Its existence proves that Terraform successfully ran and managed a real resource.