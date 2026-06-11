# Project Context

## Koishi Development Rules

This file provides context and rules for working within the `koishi` directory, which contains the infrastructure and services management for the home network cluster.

### Infrastructure & Development

*   **Cluster Management:** The cluster is managed via Kubernetes.
*   **Deployment:** Services are typically deployed using a `deploy.sh` script located in their respective subdirectories or the `koishi` directory itself.
*   **Secret Management:** Sensitive data should never be committed to the repository. Secrets are managed via Vaultwarden and rendered dynamically.
*   **Build System:** Container images are built in-cluster using Kaniko.

