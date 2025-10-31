# Contributing to Holt

First off, thank you for considering contributing to Holt! It’s people like you that make open source such a great community. We welcome any and all contributions, from documentation fixes to major feature implementations.

This guide will walk you through the process, which is a little different from most projects. We not only welcome contributions, but we actively encourage you to contribute **in collaboration with your own AI coding assistants**.

---

## The Holt Way: AI-Augmented Development

Holt is a platform for orchestrating AI agents. It is also a project that is **built using AI agents**. 

All of our documentation and much of our core code has been developed through a process of close human-AI collaboration. Our **[DEVELOPMENT_PROCESS.md](./DEVELOPMENT_PROCESS.md)** outlines this lifecycle, and the **[AI_AGENT_GUIDE.md](./AI_AGENT_GUIDE.md)** shows how we expect AI agents to navigate the codebase. We "dogfood" our own philosophy.

We encourage you to embrace this approach. Whether you are a seasoned Go developer or a student looking to get started, you can contribute to Holt. Use your favorite AI assistant (like Gemini, Claude, or ChatGPT) to help you!

**Don't know where to start? Try asking an AI:**

> "I want to contribute to the Holt project. I've cloned the repo. Read the `CONTRIBUTING.md` and `DEVELOPMENT_PROCESS.md` files and explain the steps I need to take to fix a bug. Then, help me set up my local environment."

---

## How to Contribute

### Finding an Issue to Work On

The best place to start is our GitHub Issues tab. 

- Look for issues tagged with **`good first issue`**. These are tasks that the core team has identified as being great entry points into the project.
- Look for issues tagged with **`help wanted`**. These are tasks that we would love community support on.

If you have an idea for a new feature or a change, please **open an issue first** to discuss it with the maintainers. This ensures that your work aligns with the project's goals before you invest a lot of time in it.

### The Contribution Workflow

Holt follows a standard GitHub fork-and-pull-request workflow.

1.  **Fork the repository** to your own GitHub account.
2.  **Clone your fork** to your local machine: `git clone https://github.com/YOUR_USERNAME/holt.git`
3.  **Create a new branch** for your changes: `git checkout -b my-awesome-feature`
4.  **Make your changes.** Remember to write tests!
5.  **Commit your work.** We use the [Conventional Commits](https://www.conventionalcommits.org/) standard. For example: `git commit -m "feat: Add support for new agent mode"`
6.  **Push your branch** to your fork: `git push origin my-awesome-feature`
7.  **Open a Pull Request** from your branch to the `main` branch of the original Holt repository.
8.  **Engage in the code review process.** The maintainers will review your PR and may ask for changes. This is a collaborative process!

### Setting Up Your Development Environment

**Prerequisites:**
- Go (version 1.22 or later)
- Docker Engine
- `make`

**Build & Test:**

Holt uses a simple `Makefile` for all common development tasks.

```bash
# Build all binaries (holt, orchestrator, pup) into the ./bin directory
make build

# Run all unit tests
make test

# Run all integration tests (requires Docker to be running)
make test-integration

# Check test coverage
make coverage

# Clean up build artifacts
make clean
```

---

## Code of Conduct

This project and everyone participating in it is governed by our **[Code of Conduct](./CODE_OF_CONDUCT.md)**. By participating, you are expected to uphold this code. Please report unacceptable behavior.

## Questions?

If you get stuck, please don't hesitate to open an issue. We are here to help you contribute.