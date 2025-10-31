## Holt: A Guide to Compliance

**Purpose**: To guide security, compliance, and engineering teams on how Holt's native features can be used to help meet their organization's compliance obligations.  
**Audience**: Security professionals, compliance officers, technical decision-makers.

---

## The Shared Responsibility Model

Achieving compliance with frameworks like HIPAA, SOC 2, or ISO 27001 is a **shared responsibility**. Holt is a self-hosted orchestration tool, not a managed service. This means that while Holt provides powerful, built-in features that directly support compliance objectives, your organization is ultimately responsible for building, configuring, and maintaining a compliant system architecture.

This guide is designed to assist your security and compliance teams by mapping Holt's native capabilities to the specific technical controls required by major regulatory and security frameworks.

---

## HIPAA (Health Insurance Portability and Accountability Act)

The HIPAA Security Rule establishes national standards for protecting electronic protected health information (ePHI). Holt's architecture provides features that map directly to the rule's required Technical Safeguards.

#### **Access Control (`§164.312(a)`)**

*   **Requirement**: Implement technical policies and procedures to allow access only to those persons or software programs that have been granted access rights.
*   **Holt's Features**:
    *   **Declarative Policy**: The `holt.yml` file acts as a central, version-controllable access policy. Only agent roles explicitly defined in this file can participate in the system.
    *   **Least Privilege**: The `workspace: ro` (read-only) setting for an agent in `holt.yml` enforces the principle of least privilege at the filesystem level, preventing agents from modifying code unless explicitly permitted.

#### **Audit Controls (`§164.312(b)`)**

*   **Requirement**: Implement hardware, software, and/or procedural mechanisms that record and examine activity in information systems that contain or use ePHI.
*   **Holt's Features**:
    *   **Chronological Ledger**: The **Blackboard** is Holt's core architectural feature. It is an append-only, chronological ledger where every significant event—artefact creation, claims, bids, and grants—is recorded with a unique ID and timestamp. This provides a complete and tamper-evident audit trail of all orchestration activity.
    *   **Native Audit Tools**: The `holt hoard` and `holt unearth <id>` CLI commands are built-in mechanisms for examining the audit trail recorded on the Blackboard.

#### **Integrity Controls (`§164.312(c)`)**

*   **Requirement**: Implement policies and procedures to protect ePHI from improper alteration or destruction.
*   **Holt's Features**:
    *   **Git-Native Integrity**: For all code-related work, Holt uses Git commit hashes as the payload for `CodeCommit` artefacts. This leverages Git's cryptographic hashing to ensure the integrity of the work product cannot be tampered with.
    *   **Orchestration Integrity**: The append-only nature of the Blackboard ensures that the record of the workflow itself is protected from unauthorized modification at the application level.

#### **Transmission Security (`§164.312(e)`)**

*   **Requirement**: Implement technical security measures to guard against unauthorized access to ePHI that is being transmitted over an electronic network.
*   **Holt's Features**:
    *   **Private by Design**: Holt is a fully self-hosted platform. All components run on your infrastructure. All data transmission between the Orchestrator, the Blackboard (Redis), and the agents occurs within your secure network perimeter.
    *   **Air-Gap Ready**: Because Holt has no dependencies on external cloud services for its core operation, it can be deployed in fully **air-gapped environments**, providing the highest possible level of transmission security.

---

## SOC 2 (Service Organization Control 2)

SOC 2 reports on an organization's controls related to the Trust Services Criteria. Holt provides features that support all five criteria.

*   **Security**: Holt's entire architecture is built on principles of isolation and declarative policy. The `holt.yml` file, container-native agent execution, and workspace permissioning directly support the Security criterion.

*   **Availability**: The Orchestrator is designed with **restart resilience**, allowing it to recover its state from the Blackboard after a crash. The **controller-worker pattern** for agent scaling enables high-availability configurations.

*   **Processing Integrity**: Holt's **Git-native workflow** and **append-only Blackboard** ensure that both the work product and the record of the work are complete, accurate, and protected from unauthorized modification at the application level.

*   **Confidentiality & Privacy**: Holt's **private-by-design** architecture is its strongest feature for these criteria. By running entirely on your infrastructure, often in an **air-gapped network**, you maintain absolute control over confidential data and personal information.

---

## ISO 27001:2022

ISO 27001 is the international standard for an Information Security Management System (ISMS). Holt provides tools that help satisfy many of the technical controls listed in Annex A.

*   **A.5.15 - Access Control**: Access to Holt's orchestration is governed by the declarative policies in the `holt.yml` file.

*   **A.8.16 - Monitoring Activities**: Holt provides two mechanisms for monitoring: the **Blackboard** for a persistent, chronological audit trail and **structured JSON logging** from all components for integration with real-time monitoring platforms (e.g., Splunk, Datadog).

*   **A.8.28 - Secure Coding**: Holt's **phased execution model** (`Review -> Parallel -> Exclusive`) can be used to enforce secure coding practices. For example, a mandatory `review` phase can be configured to run a Static Application Security Testing (SAST) tool, preventing code from being merged if vulnerabilities are found.

*   **A.8.25 - Configuration Management**: All Holt workflows are defined in the `holt.yml` file, which is a version-controllable configuration file that can be managed with the same rigor as application code.

---

**Disclaimer**: This document is for informational purposes only and does not constitute legal advice or a guarantee of compliance. Achieving compliance requires a comprehensive program that includes policies, procedures, and independent auditing. Please consult with a qualified compliance professional to assess your organization's specific needs.