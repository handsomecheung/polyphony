# Vaultwarden: Self-Hosted Password Manager & Secret Store

Provides a self-hosted, Bitwarden-compatible password manager (Vaultwarden) designed to serve as the cluster's centralized secret store. The system integrates seamless, secure secret rendering into configuration templates, ensuring that no sensitive credentials are ever committed to version control.

## Design Philosophy

The Vaultwarden ecosystem is designed around three core architectural goals:

1. **Centralized Secret Store (Vaultwarden Server)**:
   A single, self-hosted source of truth for all secrets, credentials, and configuration values. By self-hosting a Bitwarden-compatible server, the home lab retains full control over its sensitive data without relying on third-party cloud managers.

2. **Infrastructure as Code Integration (BWW)**:
   BWW (BitWarden for Webapp) provides an authorized API layer that resolves templated secret placeholders at deployment time. This allows Kubernetes manifests, script configurations, and application templates to remain fully declarative while referencing secrets dynamically.

3. **Secure-by-Default Isolation & Resilience (Network Policies & Backups)**:
   - **Isolation**: Vaultwarden operates under strict, zero-egress network policies. Since the password manager does not need to connect to the external internet, all outbound connections are blocked to prevent data exfiltration.
   - **Resilience**: Backups are performed using dual, automated strategies—combining logical item exports with full physical database backups. All backups are encrypted symmetrically at rest using GPG with keys derived dynamically, ensuring backups remain secure even if storage endpoints are compromised.

For specific implementation files, configuration values, build commands, and recovery procedures, refer to [AGENTS-CONTEXT.md](file:///mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/vaultwarden/AGENTS-CONTEXT.md).
