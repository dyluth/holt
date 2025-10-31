# How-To: Build a Simple File-Creation Agent

**Purpose**: To walk through the simplest possible agent in the Holt ecosystem, the `example-git-agent`, and explain its core components. This is the "Hello, World!" of Holt.

**Based on Demo**: [`/agents/example-git-agent/`](../agents/example-git-agent)

---

## 1. The Goal

This agent has one simple job: when given a filename as a `GoalDefined` artefact, it creates that file in the workspace, commits it to Git, and produces a `CodeCommit` artefact as its output.

This demonstrates the most fundamental workflow in Holt: receiving a task, interacting with the local filesystem and a command-line tool (Git), and producing a result that can be audited.

## 2. The `holt.yml` Configuration

To use this agent, it must be declared in your `holt.yml` file. The configuration is minimal:

```yaml
version: "1.0"
agents:
  git-agent: # This is the agent's unique role
    image: "example-git-agent:latest" # Assumes you have built the Docker image
    command: ["/app/run.sh"] # The tool script to execute
    workspace:
      mode: rw # Read-write access is required to create files
```

- **`git-agent`**: We assign the agent the role `git-agent`. This is how the Orchestrator knows about it.
- **`image`**: We point to a pre-built Docker image. You build this from the Dockerfile in the agent's directory.
- **`command`**: This tells the Agent Pup to execute the `/app/run.sh` script when it receives work.
- **`workspace.mode: rw`**: This is critical. The agent needs to write files into the project workspace, so it requires read-write access.

## 3. The Tool Script: `run.sh`

This is the core logic of the agent. It's a simple shell script that interacts with the Agent Pup via `stdin` and `stdout`.

```bash
#!/bin/sh

# Read the JSON input from the Agent Pup via stdin
input=$(cat)

# Extract the 'payload' from the target_artefact JSON.
# This is a simple and crude way to parse JSON in shell.
# In a real agent, you would use a more robust tool like `jq`.
goal=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

# Log to stderr so the user can see what's happening via `holt logs`
echo "Received goal: Create file named $goal" >&2

# Create the file and commit it using standard Git commands
# This is the agent's "work"
touch "$goal"
git add "$goal"
git commit -m "feat: Create $goal"

# Get the commit hash of the new commit
hash=$(git rev-parse HEAD)

# Output a valid JSON object to stdout for the Agent Pup.
# This will become the new CodeCommit artefact.
cat <<EOF
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "$hash",
  "summary": "Created file $goal as per request"
}
EOF
```

### Key Concepts Illustrated:

- **Input/Output Contract**: The script reads a JSON object from `stdin` and writes a single JSON object to `stdout`. This is the fundamental contract between the Agent Pup and the tool script.
- **Performing Work**: The agent uses standard, battle-hardened command-line tools (`touch`, `git`) to perform its task. Holt agents can use *any* tool that can be run in a container.
- **Logging**: The script writes progress and debug information to `stderr`. This is how you provide visibility into the agent's operations, which can be viewed with the `holt logs git-agent` command.
- **Producing an Artefact**: The script's final output is a JSON object that defines the next artefact to be created on the Blackboard. Here, it creates a `CodeCommit` artefact, providing the `hash` as the payload.

## 4. The `Dockerfile`

The Dockerfile packages the agent's logic and the Holt Agent Pup into a self-contained, runnable image.

```dockerfile
# Use a multi-stage build to keep the final image small
FROM golang:1.24-alpine AS builder
WORKDIR /build
# Copy only the necessary Go files to build the pup
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY pkg/blackboard ./pkg/blackboard
# Build the pup binary
RUN CGO_ENABLED=0 go build -o pup ./cmd/pup

# Final, minimal image
FROM alpine:latest
# Git is a required tool for this agent
RUN apk --no-cache add ca-certificates git
WORKDIR /app
# Copy the pup binary from the builder stage
COPY --from=builder /build/pup /app/pup
# Copy the tool script
COPY agents/example-git-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh
# Create a non-root user for security
RUN adduser -D -u 1000 agent
USER agent
# The pup is the entrypoint that wraps the tool script
ENTRYPOINT ["/app/pup"]
```

### Key Concepts Illustrated:

- **Packaging Dependencies**: The agent's required tools (in this case, `git`) are installed directly into the container.
- **Agent Pup Integration**: The `pup` binary is copied into the image and set as the `ENTRYPOINT`. The pup then handles all the communication with the Orchestrator and executes the `run.sh` script defined in the `command` section of `holt.yml`.
- **Security**: The agent runs as a non-root `agent` user for better security.

## 5. How to Run This Demo

These steps are also covered in the main `README.md` Quick Start.

1.  **Build the image:**
    ```bash
    docker build -t example-git-agent:latest -f agents/example-git-agent/Dockerfile .
    ```
2.  **Configure `holt.yml`** as shown in section 2.
3.  **Start Holt:**
    ```bash
    holt up
    ```
4.  **Give the agent a goal:**
    ```bash
    holt forage --goal "my-first-file.txt"
    ```
5.  **Watch it work:**
    ```bash
    holt watch
    # You will see the claim being created, bid on, and granted.
    ```
6.  **Verify the result:**
    ```bash
    ls my-first-file.txt # The file exists!
git log -n 1         # See the commit made by the agent.
holt hoard          # See the CodeCommit artefact on the Blackboard.
    ```