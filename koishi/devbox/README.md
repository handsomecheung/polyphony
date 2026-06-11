# devbox

`devbox` is the primary remote development environment for the Koishi cluster. It provides a consistent, reproducible, and secure workspace accessible via SSH or a web browser.

## Overview

The devbox is built on Ubuntu 24.04 but leverages the **Nix package manager** to provide a flexible and declarative toolchain. It is designed to be "always ready," mounting shared persistent storage for code, configurations, and build caches.

## Key Features

- **Multi-Access**: Connect via traditional SSH or use **VS Code in the browser** via `code-server`.
- **Declarative Tooling**: Most development tools (Go, Node.js, Python, Kubernetes CLI, etc.) are managed via Nix.
- **Reproducible Environment**: Environment initialization is handled by Ansible and shell scripts during container startup.
- **Secure by Design**: SSH access requires both a password and **Google Authenticator MFA**.
- **Docker & Kubernetes Integration**: Pre-configured with `kubectl`, `helm`, and access to the host's Docker daemon.
- **Persistent Workspaces**: Mounts shared NFS volumes for `/mnt/coder-workspaces`, `/mnt/coder-sharepoint`, and `/mnt/user-data-slight`.

## Technology Stack

- **Base OS**: Ubuntu 24.04 (Noble Numbat)
- **Package Managers**: Nix (primary), APT (system dependencies)
- **Editors**: 
    - Emacs 30 (with Doom Emacs support)
    - `code-server` (Web IDE)
    - Neovim
- **Languages**: Go, Node.js 22, Python 3.12, Ruby 3.2, JDK 21
- **MFA**: `libpam-google-authenticator`

## Security

### SSH MFA
To connect via SSH, you must provide:
1. The password for the `box` user.
2. A 6-digit TOTP code from your Google Authenticator app.

### Privileges
- **Implicit sudo**: Administrative tasks like `kubectl`, `bwww` or `docker` commands require `sudo`.
- **Explicit sudo**: The `sudo` command itself is **not available** in the environment. Instead, privileged commands are configured to prompt for a password implicitly when needed.
- **User**: The environment runs as the unprivileged `box` user (UID 1000).

## Usage Notes

- **Hostname**: The hostname is set to `devbox` (or `devbox-<node>`) to allow AI Agents and scripts to detect the environment.
- **Docker**: Access to Docker is provided by mounting `/var/run/docker.sock` from the host.
- **Kubernetes**: The container uses a `ServiceAccount` with `cluster-admin` privileges to manage the local cluster.

See parent [koishi README](../README.md) for cluster-wide architecture.
