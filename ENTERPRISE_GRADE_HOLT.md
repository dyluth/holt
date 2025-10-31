# Holt: Enterprise-Grade By Design

**Purpose**: To articulate the enterprise-ready security, auditability, and governance features that are fundamental to the Holt platform.  
**Audience**: Technical decision-makers, architects, and security/compliance teams.

---

Holt is an enterprise-grade orchestration platform built on a philosophy of first principles. It was designed from the ground up for organizations that require maximum security, auditability, and control over their AI agent workflows.

While some frameworks provide high-level abstractions, Holt provides an opinionated, integrated system of core components—an Orchestrator, an agent runtime (`pup`), and a CLI. These components work together to deliver a robust, secure, and auditable environment **out-of-the-box**, without sacrificing transparency.

## 1. Absolute Data Sovereignty: Private by Design

Holt is architected for absolute data sovereignty. Unlike frameworks that may require special "Enterprise" versions to prevent data from leaving your network, Holt is **private by design**.

The entire platform—the Orchestrator, the Redis Blackboard, and every agent container—runs entirely within your own environment. This allows for deployment in fully **air-gapped networks**, as Holt has no dependency on external cloud services for its core functionality.

*   **Zero Third-Party Services**: No data, metadata, logs, or telemetry ever leave your security perimeter. There is no cloud-based UI or external monitoring service to disable.
*   **Platform Agnostic**: Holt runs wherever Docker runs, from a local machine to a VPC or a government cloud. You are never locked into a specific vendor's infrastructure.
*   **Built-in Security Posture**: Agents run as non-root users in isolated containers with the minimum necessary privileges. The `workspace` configuration in `holt.yml` allows for specifying read-only (`ro`) or read-write (`rw`) access on a per-agent basis, enforcing the principle of least privilege at the filesystem level.


## 2. Unparalleled Auditability: The Chronological Ledger

Holt provides a complete and chronological audit trail out-of-the-box. This is not an add-on feature; it is an intrinsic property of Holt's event-sourced architecture.

The central **Blackboard** acts as a chronological, append-only ledger. Every `Artefact` created, `Claim` made, `Bid` submitted, and `Grant` issued is persisted to Redis with a timestamp and a unique ID. This provides a complete, step-by-step history of every workflow for compliance and forensic analysis.

*   **Native Control Plane**: The `holt` CLI provides built-in tools to inspect this ledger directly. There is no need for an external UI.
    *   `holt hoard`: Lists all artefacts, providing a high-level overview of the workflow's output.
    *   `holt unearth <id>`: Retrieves the full content and metadata of any specific artefact or claim.
    *   `holt logs <agent-role>`: Provides direct access to the logs of any agent container.
*   **Hooks for Enterprise Monitoring**: All Holt components emit high-quality, structured (JSON) logs to `stdout`. This is the standard mechanism for integrating with any enterprise-grade log aggregation platform (Splunk, Datadog, ELK, etc.).


## 3. Built-in Governance: Declarative & Enforced

Holt's architecture includes multiple layers of governance that are enforced by the Orchestrator, enabling safe and controlled agent operation.

*   **Declarative Policy**: The `holt.yml` file acts as a central policy document. It explicitly declares the entire "clan" of trusted agents, their capabilities, and their access rights (`workspace: ro/rw`). An agent not defined in this file cannot participate in the system.
*   **Phased Execution Model**: The `Review -> Parallel -> Exclusive` phased execution is a powerful, built-in governance workflow that ensures no action is taken without passing through a formal review gate.
*   **Native Human-in-the-Loop (HITL)**: The `Question`/`Answer` artefact flow is Holt's native HITL mechanism. Agents can pause a workflow to request human guidance, and the `holt questions` and `holt answer` CLI commands provide the interface for this intervention.
*   **Programmatic Control Points**: The `bid_script` feature allows for sophisticated, programmatic control over an agent's behavior, enabling complex logic to govern when an agent should engage.


## Conclusion: Enterprise-Ready from First Principles

Holt's minimalist philosophy delivers the capabilities of a large framework without the opaque abstractions. It combines the transparency of a simple, auditable architecture with the robust, out-of-the-box features of a complete platform.

## A Note on Compliance

Holt was designed with the needs of regulated industries in mind. For a more detailed breakdown of how Holt's features can be used to help satisfy the technical controls of frameworks like HIPAA, SOC 2, and ISO 27001, please see our dedicated compliance guide:

**[➡️ Holt: A Guide to Compliance](./HOLT_COMPLIANCE_GUIDE.md)**