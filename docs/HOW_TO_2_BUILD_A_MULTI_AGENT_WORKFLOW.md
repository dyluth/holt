# How-To: Build a Multi-Agent Content Workflow

**Purpose**: To explain the `recipe-generator` demo, showcasing how multiple specialized agents can collaborate in a phased workflow to generate, review, and format content.

**Based on Demo**: [`/demos/recipe-generator/`](../demos/recipe-generator)

---

## 1. The Goal

This demo uses a clan of three agents to automatically generate a recipe card based on a simple user prompt (e.g., "a spicy chicken dish"). This demonstrates a classic content generation pipeline and highlights Holt's phased execution model.

- **`recipe-writer`**: An LLM-based agent that generates the initial recipe text.
- **`recipe-reviewer`**: A second LLM-based agent that reviews the generated recipe for clarity and safety.
- **`card-formatter`**: A deterministic agent that formats the final, approved recipe into a styled HTML card.

## 2. The `holt.yml` Configuration

This configuration defines the three agents and their distinct roles and permissions.

```yaml
version: "1.0"
agents:
  recipe-writer:
    build:
      context: ./agents/recipe-writer
    command: ["/usr/bin/python", "run.py"]
    workspace:
      mode: ro # Only needs to read the goal, not write files
    environment:
      - OPENAI_API_KEY

  recipe-reviewer:
    build:
      context: ./agents/recipe-reviewer
    command: ["/usr/bin/python", "run.py"]
    workspace:
      mode: ro
    environment:
      - OPENAI_API_KEY

  card-formatter:
    build:
      context: ./agents/card-formatter
    command: ["/usr/bin/python", "run.py"]
    workspace:
      mode: rw # Needs to write the final HTML file
```

### Key Concepts Illustrated:

- **Specialized Roles**: Each agent has a unique role (`recipe-writer`, `recipe-reviewer`, `card-formatter`). This allows them to bid on claims differently.
- **Least Privilege**: The `writer` and `reviewer` agents have read-only (`ro`) access to the workspace because they only need to read the input artefacts. The `formatter` agent, however, has read-write (`rw`) access because its job is to create the final `recipe.html` file.
- **Environment Variables**: The LLM-based agents securely receive their `OPENAI_API_KEY` from the host environment, preventing secrets from being hardcoded in the agent's image.

## 3. The Workflow in Action: Phased Execution

When you run `holt forage --goal "a spicy chicken dish"`, the following sequence, enforced by the Orchestrator, occurs:

1.  **Claim Creation**: A `GoalDefined` artefact is created. The Orchestrator creates a claim for it.

2.  **Bidding**: All three agents receive the claim.
    *   `recipe-writer` bids **`exclusive`** (it wants to be the sole author).
    *   `recipe-reviewer` bids **`review`** (it wants to review the work of others).
    *   `card-formatter` bids **`exclusive`** (it also wants to be the final, sole processor).

3.  **Review Phase**: The Orchestrator sees the `review` bid and starts with the Review Phase.
    *   **Grant**: The `recipe-reviewer` is granted the claim.
    *   **Execution**: The reviewer sees there is no existing work to review yet, so it approves the claim by producing an empty `Review` artefact. This signals to the Orchestrator that the process can proceed.

4.  **Exclusive Phase (Round 1)**: The Orchestrator now moves to the Exclusive Phase. Based on an alphabetical sort of the bidders, `card-formatter` wins the initial exclusive grant.
    *   **Grant**: `card-formatter` is granted the claim.
    *   **Execution**: The formatter agent is smart enough to see that the input is just a `GoalDefined` artefact, not a structured recipe it can use. It cannot perform its function, so it simply ignores the claim for now, waiting for a valid `Recipe` artefact to appear on the Blackboard.

5.  **New Claim & Bidding (Round 2)**: Meanwhile, the `recipe-writer` agent, also seeing the `GoalDefined` artefact, proceeds to generate the recipe.
    *   **Grant**: The `recipe-writer` is granted the claim for the original `GoalDefined` artefact.
    *   **Execution**: The `recipe-writer` calls the OpenAI API, generates a recipe, and produces a `Recipe` artefact containing the structured recipe data.

6.  **New Claim & Bidding (Round 3)**: A new claim is created for the `Recipe` artefact.
    *   `recipe-reviewer` bids **`review`**.
    *   `card-formatter` bids **`exclusive`**.
    *   `recipe-writer` bids **`ignore`** (its job is done).

7.  **Review Phase (Round 2)**: The `recipe-reviewer` is granted the claim. It inspects the `Recipe` artefact and approves it by creating an empty `Review` artefact.

8.  **Exclusive Phase (Round 2)**: With the review complete, the `card-formatter` is granted the exclusive claim. It takes the structured `Recipe` data, formats it into HTML, writes `recipe.html` to the workspace, and produces a final `Card` artefact.

This complex dance is not programmed into the agents themselves; it is an emergent result of how they bid and the strict, phased workflow enforced by the Holt Orchestrator.

## 4. The Agent Implementations

Each agent has a `run.py` script that serves as its tool logic.

- **`recipe-writer/run.py`**: Connects to the OpenAI API, constructs a prompt to generate a recipe based on the input goal, and outputs a `Recipe` artefact.
- **`recipe-reviewer/run.py`**: Examines the input artefact. If it's a `Recipe`, it approves. If it's anything else, it also approves (a simplification for the demo). In a real-world scenario, it would call an LLM to critique the recipe.
- **`card-formatter/run.py`**: Checks if the input is a structured `Recipe`. If yes, it uses a Python template to generate an HTML file and saves it to the workspace. If no, it produces a `Question` artefact.

## 5. How to Run This Demo

1.  **Set your API Key:**
    ```bash
    export OPENAI_API_KEY="your-key-here"
    ```
2.  **Navigate to the demo directory:**
    ```bash
    cd demos/recipe-generator
    ```
3.  **Run the test workflow script:**
    This script automates building the Docker images, starting Holt, and running the `forage` command.
    ```bash
    ./test_workflow.sh
    ```
4.  **Verify the result:**
    After the script finishes, you will find a `recipe.html` file in the directory, beautifully formatted by the `card-formatter` agent.