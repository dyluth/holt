# **Phase 6: Kubernetes-Native Support**

**Goal**: Evolve Holt from a Docker-based system to a fully Kubernetes-native platform, enabling scalable, resilient, and enterprise-ready deployments. This will be achieved by creating a **Holt Operator** that manages Holt instances and their components via Custom Resource Definitions (CRDs).

## **Phase Success Criteria**

- A user can define and deploy a complete Holt instance, including its agent clan, by applying a single `Holt` Custom Resource (CR) to a Kubernetes cluster.
- The Holt Operator successfully reconciles the `Holt` CR, deploying and managing the lifecycle of the Orchestrator, Redis (optional), and all defined Agents as Kubernetes resources (Deployments, Pods, Jobs, Services, etc.).
- The Orchestrator, when running in Kubernetes mode, no longer communicates with the Docker API. Instead, it creates `HoltAgent` CRs to request agent execution, which are then handled by the Operator.
- The system supports both an Operator-managed, in-cluster Redis instance and the ability to connect to an externally managed Redis service.
- A shared Persistent Volume (PV) is used for the Git workspace, accessible by all agent Pods within an instance.
- Enterprise-grade features such as `ServiceAccounts`, `NetworkPolicies`, and fine-grained RBAC are utilized to ensure secure, multi-tenant operation.
- The existing Docker-based mode and the new Kubernetes-native mode are both fully supported, allowing users to choose their preferred deployment model.

## **Key Architectural Changes & Features**

### 1. The Holt Operator

The central component of this phase is a new Kubernetes Operator. Its responsibilities include:
- **Watching for `Holt` CRs:** The Operator will monitor the cluster for the creation, update, and deletion of `Holt` resources.
- **Managing Core Infrastructure:** For each `Holt` CR, the Operator will deploy the Orchestrator (as a `Deployment`) and, optionally, a Redis instance (as a `StatefulSet` with a corresponding `Service`).
- **Managing Agent Execution:** The Operator will watch for `HoltAgent` CRs created by the Orchestrator. Upon seeing a new `HoltAgent` CR, it will translate it into the appropriate Kubernetes resource—either a long-running `Pod` for a `reuse` agent or a one-off `Job` for a `fresh_per_call` agent.

### 2. Custom Resource Definitions (CRDs)

The `holt.yml` file will be replaced by a set of CRDs for Kubernetes-native configuration.

*   **`Holt` CRD:** This is the top-level resource that defines an entire Holt instance. It will contain the equivalent of the `holt.yml` file, including the full list of agent definitions, service configurations, and new Kubernetes-specific settings like networking policies.

    ```yaml
    apiVersion: holt.io/v1alpha1
    kind: Holt
    metadata:
      name: production-instance
    spec:
      redis:
        # Use an external Redis instance
        external: redis.my-org.com:6379
      workspace:
        # Use an existing Persistent Volume Claim for the git repo
        persistentVolumeClaim:
          claimName: git-repo-pvc
      agents:
        Writer:
          image: holt-writer-agent:latest
          command: ["/app/run.sh"]
          # ... and so on
    ```

*   **`HoltAgent` CRD:** This resource represents a request from the Orchestrator to execute a specific agent for a specific claim. The Operator acts on these CRs.

    ```yaml
    apiVersion: holt.io/v1alpha1
    kind: HoltAgent
    metadata:
      name: writer-for-claim-abc123
    spec:
      role: "Writer"
      claimID: "abc123..."
      # ... other necessary parameters
    ```

### 3. The Orchestrator's New Role

When deployed in Kubernetes, the Orchestrator's responsibilities shift:
- **FROM:** Directly creating Docker containers.
- **TO:** Creating `HoltAgent` Custom Resources. It describes the work that needs to be done, and the Operator is responsible for fulfilling that request by creating a Pod or Job.

This decouples the Orchestrator from the underlying container runtime, making Holt more portable and aligned with cloud-native best practices.

### 4. Workspace Management with Persistent Volumes

- Each Holt instance will be associated with a **Persistent Volume Claim (PVC)** that provides the shared filesystem for the Git repository.
- Agent Pods will mount this PVC at `/workspace`.
- The design will need to account for initializing this volume, potentially with an `initContainer` that performs a `git clone` if the volume is empty.

### 5. Networking and Security

- By default, the Operator will create a `NetworkPolicy` that restricts communication within a Holt instance, allowing agents and the Orchestrator to talk to Redis but preventing cross-talk between agents unless explicitly allowed.
- Agents that require external network access (e.g., to call LLM APIs or other web services) will need this specified in their definition within the `Holt` CR, which the Operator will use to configure the appropriate egress rules.

## **Implementation Constraints**

- **Dual-Mode Support:** The existing Docker-based mode must continue to be fully supported. The Orchestrator will need to be configurable to run in either "docker" or "kubernetes" mode, and the `holt up` command will remain the entry point for local Docker-based development.
- The Kubernetes mode is an alternative deployment target, not a replacement for the simplicity of the local Docker setup.
- The core logic within Redis (artefacts, claims, etc.) will remain unchanged.
- The Agent Pup binary will require no significant changes, as it is abstracted from the underlying container runtime.

## **Dependencies**

- A running Kubernetes cluster (e.g., Minikube, Kind, or a cloud provider's offering).
- The `kubebuilder` or a similar framework to facilitate Operator development.
