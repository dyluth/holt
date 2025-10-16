# Sett

**A container-native AI agent orchestrator for automating complex software engineering workflows**

Sett manages a clan of specialized, tool-equipped AI agents that collaborate on multi-step development tasks while maintaining complete auditability and human oversight.

---

## Project Status

**Phase 2 Complete** ✅ - Single-agent workflows with Git integration

Current capabilities:
- ✅ Event-driven orchestration via Redis blackboard
- ✅ Container-native agent execution
- ✅ Git-centric workflow with commit tracking
- ✅ Complete immutable audit trail
- ✅ Human-in-the-loop support
- ✅ Multi-instance workspace safety

Coming in Phase 3:
- 🚧 Multi-agent coordination (review → parallel → exclusive phases)
- 🚧 Controller-worker scaling pattern
- 🚧 Advanced consensus mechanisms

---

## Quick Start

### Prerequisites

- **Docker** (20.10+) - For running agent containers
- **Git** (2.x+) - For workspace management
- **Go** (1.21+) - For building Sett binaries

### Installation & First Workflow

```bash
# 1. Clone repository
git clone https://github.com/anthropics/sett.git
cd sett

# 2. Build binaries
make build

# 3. Create test project
mkdir my-project && cd my-project
git init
git commit --allow-empty -m "Initial commit"

# 4. Initialize Sett
sett init

# 5. Build example git agent
docker build -t example-git-agent:latest -f agents/example-git-agent/Dockerfile ..

# 6. Configure agent in sett.yml
cat > sett.yml <<EOF
version: "1.0"
agents:
  git-agent:
    role: "Git Agent"
    image: "example-git-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: rw
services:
  redis:
    image: redis:7-alpine
EOF

# 7. Start Sett instance
sett up

# 8. Create workflow
sett forage --goal "hello.txt"

# 9. Watch agent execute
sett watch

# 10. View results
sett hoard
git log --oneline
ls -la hello.txt  # File created by agent!
```

**What just happened?**

1. Sett started an orchestrator and your git agent in Docker containers
2. The orchestrator created a claim for your goal ("hello.txt")
3. The git agent bid on and won the claim
4. The agent created `hello.txt` in your workspace and committed it
5. A `CodeCommit` artefact was created on the blackboard with the commit hash
6. Complete audit trail preserved in Redis and Git history

---

## How It Works: A Conceptual Overview

This diagram illustrates the high-level conceptual workflow of the Sett system, demonstrating how a user goal initiates a collaborative, auditable process between the orchestrator and a clan of specialized AI agents interacting via the central Redis Blackboard.

```mermaid
graph TD
    User([fa:fa-user User]) -- "1. `sett forage --goal '...'`" --> CLI(fa:fa-terminal Sett CLI)

    subgraph "Sett System"
        direction LR
        
        subgraph "Execution Plane"
            Agents["fa:fa-users AI Agent Clan<br/>(e.g., Coder, Tester, Reviewer)"]
            Tools([fa:fa-wrench Tools<br/>Git, Linters, etc.])
        end

        subgraph "Control & Data Plane"
            Orchestrator(fa:fa-sitemap Orchestrator)
            
            subgraph Blackboard [fa:fa-database Redis Blackboard]
                Artefacts("fa:fa-file-alt Artefacts")
                Claims("fa:fa-check-square Claims")
                Bids("fa:fa-gavel Bids")
            end
        end
    end

    CLI -- "2. Writes Goal Artefact" --> Blackboard
    
    Blackboard -- "3. Event" --> Orchestrator
    Orchestrator -- "4. Creates Claim" --> Blackboard
    
    Blackboard -- "5. Event" --> Agents
    Agents -- "6. Submit Bids" --> Blackboard
    
    Orchestrator -- "7. Grants Claim" --> Blackboard
    Blackboard -- "8. Notifies Winning Agent" --> Agents
    
    Agents -- "9. Executes Work Using" --> Tools
    Tools -- "10. Produces Result (e.g., Git Commit)" --> Agents
    Agents -- "11. Writes New Artefact" --> Blackboard

    Blackboard -- "12. Loop: Next Cycle Begins..." --> Orchestrator

    %% Style Definitions
    classDef core fill:#d4edda,stroke:#155724,color:#000;
    classDef agent fill:#ddebf7,stroke:#3b7ddd,color:#000;
    classDef user fill:#f8d7da,stroke:#721c24,color:#000;
    classDef data fill:#fff3cd,stroke:#856404,color:#000;

    class Orchestrator,Blackboard core;
    class Agents agent;
    class User,CLI user;
    class Tools data;
```

## Core Concepts

### The Blackboard

A Redis-based shared state system where all components interact. Think of it as a lightweight ledger storing:

- **Artefacts**: Immutable work products (code commits, designs, analyses)
- **Claims**: The orchestrator's decisions about work assignment
- **Bids**: Agents' expressions of interest in claims

Every interaction is timestamped and recorded, creating an immutable audit trail perfect for regulated environments.

### Artefacts

Immutable data objects representing work products. Each artefact has:

- **type**: User-defined (e.g., "CodeCommit", "DesignSpec", "TestReport")
- **payload**: Main content (commit hash, JSON data, text)
- **source_artefacts**: Dependency chain for provenance tracking
- **structural_type**: Role in orchestration (Standard, Review, Question, Answer, Failure, Terminal)

Artefacts never change. Instead, new versions form logical threads tracked via `logical_id`.

### Claims & Phased Execution

When an artefact is created, the orchestrator creates a **claim** and agents bid on it. Claims progress through phases:

1. **Review Phase**: Parallel review by multiple agents (Phase 3)
2. **Parallel Phase**: Concurrent work by multiple agents (Phase 3)
3. **Exclusive Phase**: Single agent gets exclusive access ✅ (Phase 2)

Agents submit bids ("review", "claim", "exclusive", "ignore") based on their capabilities and the work required.

### The Agent Cub

A lightweight Go binary that runs as the entrypoint in every agent container. It:

- Watches for claims via Pub/Sub
- Submits bids on behalf of the agent
- Assembles historical context via graph traversal
- Executes the agent's tool script with JSON contract
- Creates artefacts from tool output
- Operates concurrently to remain responsive

**Key insight:** The cub handles orchestration complexity. You just write the tool logic.

### Agents

Docker containers packaging:

1. **Agent cub binary** (handles Sett integration)
2. **Tool script** (your custom logic - shell, Python, anything)
3. **Dependencies** (LLM APIs, compilers, CLIs, etc.)

Agents communicate with the cub via stdin/stdout JSON:

**Input:**
```json
{
  "claim_type": "exclusive",
  "target_artefact": { "type": "GoalDefined", "payload": "build auth API", ... },
  "context_chain": [ /* historical artefacts */ ]
}
```

**Output:**
```json
{
  "artefact_type": "CodeCommit",
  "artefact_payload": "abc123def456...",
  "summary": "Implemented authentication endpoints"
}
```

### Git-Centric Workflow

Sett assumes and requires a clean Git repository. Code artefacts are git commit hashes, and agents are responsible for:

- Creating or modifying files
- Committing changes with descriptive messages
- Returning commit hashes as `CodeCommit` artefacts

The cub validates commit hashes exist before creating artefacts, ensuring integrity.

### Human-in-the-Loop

Sett is designed for human oversight:

- **Question artefacts**: Agents can ask humans for guidance (Phase 4)
- **Review phase**: Humans or review agents can provide feedback before execution (Phase 3)
- **Complete audit trail**: Every decision is traceable for compliance
- **Manual intervention**: Humans can inspect state and intervene at any point
```

---

## CLI Commands

### Instance Management

```bash
# Initialize new Sett project
sett init

# Start Sett instance (auto-incremented name: default-1, default-2, ...)
sett up

# Start with specific name
sett up --name prod

# Stop instance (infers most recent if name omitted)
sett down
sett down --name prod

# List all running instances
sett list
```

### Workflow Management

```bash
# Create workflow with a goal
sett forage --goal "Build REST API for user management"

# Target specific instance
sett forage --name prod --goal "Refactor authentication"

# Validate orchestrator creates claim (Phase 1)
sett forage --watch --goal "Add logging to endpoints"
```

### Monitoring & Debugging

```bash
# View live activity (infers most recent instance)
sett watch

# Target specific instance
sett watch --name prod

# View all artefacts on blackboard
sett hoard

# View agent logs
sett logs git-agent
sett logs orchestrator

# View questions requiring human input (Phase 4)
sett questions --wait

# Answer a question (Phase 4)
sett answer <question-id> "Use JWT tokens with RS256"
```

---

## Building Custom Agents

### Minimal Echo Agent

**agents/my-agent/run.sh:**
```bash
#!/bin/sh
input=$(cat)

# Extract goal from payload
goal=$(echo "$input" | grep -o '"payload":"[^"]*"' | head -1 | cut -d'"' -f4)

echo "Processing: $goal" >&2

# Output result
cat <<EOF
{
  "artefact_type": "Processed",
  "artefact_payload": "Result for $goal",
  "summary": "Processing complete"
}
EOF
```

**agents/my-agent/Dockerfile:**
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/cub ./cmd/cub
COPY internal/cub ./internal/cub
COPY pkg/blackboard ./pkg/blackboard
RUN CGO_ENABLED=0 go build -o cub ./cmd/cub

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/cub /app/cub
COPY agents/my-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh
RUN adduser -D -u 1000 agent
USER agent
ENTRYPOINT ["/app/cub"]
```

**sett.yml:**
```yaml
version: "1.0"
agents:
  my-agent:
    role: "My Agent"
    image: "my-agent:latest"
    command: ["/app/run.sh"]
    workspace:
      mode: ro
services:
  redis:
    image: redis:7-alpine
```

**Build & Run:**
```bash
docker build -t my-agent:latest -f agents/my-agent/Dockerfile .
sett up
sett forage --goal "test input"
sett logs my-agent
```

For complete agent development guide, see: **[docs/agent-development.md](./docs/agent-development.md)**

---

## Example Agents

### Echo Agent
**Location:** `agents/example-agent/`

Simple agent demonstrating basic stdin/stdout contract. Reads goal, logs it, outputs success artefact.

**Use case:** Learning, testing, proof-of-concept

### Git Agent
**Location:** `agents/example-git-agent/`

Creates files in workspace and commits them, returning `CodeCommit` artefacts.

**Use case:** Code generation, file creation, project scaffolding

**Example workflow:**
```bash
# Build agent
docker build -t example-git-agent:latest -f agents/example-git-agent/Dockerfile .

# Start Sett
sett up

# Create file via agent
sett forage --goal "implementation.go"

# Verify result
git log --oneline  # Shows commit by agent
ls implementation.go  # File exists
sett hoard  # Shows CodeCommit artefact
```

---

## Development

### Building from Source

```bash
# Build all binaries
make build

# Build specific binary
make build-cli
make build-orchestrator
make build-cub

# Run tests
make test

# Run integration tests (requires Docker)
make test-integration

# Check coverage
make coverage
```

### Project Structure

```
sett/
├── cmd/                      # Binaries
│   ├── sett/                # CLI
│   ├── orchestrator/        # Orchestrator daemon
│   └── cub/                 # Agent cub binary
├── internal/                # Private packages
│   ├── cub/                 # Cub logic
│   ├── orchestrator/        # Orchestrator engine
│   ├── config/              # Configuration
│   ├── git/                 # Git integration
│   └── testutil/            # E2E test helpers
├── pkg/blackboard/          # Public blackboard client
├── agents/                  # Example agents
│   ├── example-agent/       # Echo agent
│   └── example-git-agent/   # Git workflow agent
├── design/                  # Design documents
│   ├── features/            # Feature specs by phase
│   └── sett-system-specification.md
└── docs/                    # User documentation
    ├── agent-development.md # Agent building guide
    └── troubleshooting.md   # Common issues & solutions
```

---

## Key Design Principles

### Pragmatism over Novelty (YAGNI)

We use battle-hardened tools (Docker, Redis, Git) rather than building custom solutions. Sett is an orchestrator, not a database or container runtime.

### Zero-Configuration, Progressively Enhanced

`sett init && sett up` creates a working system. Smart defaults cover 90% of use cases. Advanced features available when needed.

### Auditability as a Core Feature

Artefacts are immutable. Every decision is recorded on the blackboard with timestamps. Complete audit trail for compliance and debugging.

### Small, Single-Purpose Components

Each component (orchestrator, CLI, agent cub) has one job and does it excellently. Complexity is managed through composition.

### Container-Native by Design

Agents can use any tool that can be containerized - not just Python functions. This enables orchestration of compilers, CLIs, infrastructure tools, and more.

---

## Use Cases

### Software Engineering

- **Multi-step code generation**: Design → Implement → Test → Deploy
- **Refactoring workflows**: Analyze → Refactor → Validate → Commit
- **Documentation generation**: Read code → Generate docs → Update README
- **Test automation**: Write tests → Run tests → Fix failures → Verify

### Regulated Industries

Sett's immutable audit trail and human-in-the-loop design make it uniquely suited for:

- **Financial services**: Auditable AI workflows for risk assessment, compliance reporting
- **Healthcare**: Traceable AI-assisted processes for clinical documentation, research protocols
- **Government**: Controllable AI automation with full audit trails for policy analysis
- **Legal**: Documented AI workflows for contract analysis, due diligence

### DevOps & Infrastructure

- **Infrastructure as code**: Generate Terraform → Review → Apply → Validate
- **Deployment automation**: Build → Test → Stage → Deploy with approval gates
- **Monitoring & alerting**: Detect issue → Diagnose → Propose fix → Human approval → Apply

---

## What Makes Sett Different

### vs. LangChain / LlamaIndex

- **Container-native**: Orchestrates any CLI tool, not just Python functions
- **Event-driven**: Agents don't poll - they react to Pub/Sub events
- **Immutable audit trail**: Every decision permanently recorded
- **Human-in-the-loop**: Designed for oversight, not autonomous operation

### vs. CrewAI / AutoGPT

- **Production-ready**: Built for reliability, not research demos
- **Git-centric**: Version control integrated from the ground up
- **Multi-instance**: Multiple Setts can run concurrently with workspace safety
- **Compliance-focused**: Audit trail and human controls for regulated industries

### vs. Temporal / Airflow

- **AI-native**: Designed for LLM agent orchestration, not general workflows
- **Blackboard pattern**: Shared state with event-driven coordination
- **Container-per-agent**: Isolation and security by default
- **Zero infrastructure**: Just Docker and Redis - no clusters to manage

---

## Roadmap

### Phase 1: "Heartbeat" ✅

- Redis blackboard with Pub/Sub
- CLI lifecycle management
- Orchestrator claim engine
- Basic E2E workflow

### Phase 2: "Single Agent" ✅

- Agent cub implementation
- Claim watching and bidding
- Tool execution contract
- Git workspace integration
- Context assembly algorithm

### Phase 3: "Coordination" 🚧

- Multi-agent coordination (review → parallel → exclusive)
- Controller-worker scaling pattern
- Full consensus bidding model
- Enhanced error handling

### Phase 4: "Human-in-the-Loop" 📋

- Question/Answer artefacts
- Human approval gates
- Interactive workflows
- Production monitoring

### Beyond Phase 4

- Dynamic agent registration
- Advanced context strategies
- LLM-based bidding
- Distributed deployment

---

## Documentation

- **[Agent Development Guide](./docs/agent-development.md)** - Build custom agents
- **[Troubleshooting Guide](./docs/troubleshooting.md)** - Common issues & solutions
- **[Project Context](./PROJECT_CONTEXT.md)** - Philosophy, principles, vision
- **[System Specification](./design/sett-system-specification.md)** - Complete architecture
- **[Feature Design Template](./design/sett-feature-design-template.md)** - Development process

---

## Contributing

Sett uses a systematic, template-driven feature design process. Every feature must be designed using the standardized template before implementation.

**Process:**

1. **Design**: Create feature document using `design/sett-feature-design-template.md`
2. **Review**: Iterate on design with human review
3. **Implement**: Build feature according to approved design
4. **Test**: Comprehensive unit, integration, and E2E tests
5. **Validate**: Verify against success criteria and Definition of Done

See `DEVELOPMENT_PROCESS.md` for details.

---

## License

[License information to be added]

---

## Support

- **Issues**: https://github.com/anthropics/sett/issues
- **Documentation**: Start with this README, then see `docs/`
- **Examples**: See `agents/` directory for reference implementations

---

## Acknowledgments

Built by Anthropic as a reference implementation for container-native AI agent orchestration with auditability and human oversight as first-class features.

---

**Ready to build AI workflows with full audit trails? Start with the [Quick Start](#quick-start) above.**
