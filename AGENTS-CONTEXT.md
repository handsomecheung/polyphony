# Project Context
Before starting work, ensure you have read the `README.md` in the root directory to understand the project's background, objectives, and overall architecture. If a `README.md` exists in your current working directory (the project subdirectory you are modifying), refer to it for specific instructions and project details.

## polyphony Development Rules

This file provides context and rules for working within the project root directory, which contains the infrastructure and services management for the home network cluster.

### 1. Core Instructions & Policies

*   **Modification Policy:** Do not modify the code unless I explicitly ask you to. If you believe a modification is necessary, explain the reason and obtain my consent first.
*   **Bash Shebang:** For bash scripts, set the shebang to `#!/usr/bin/env bash`.


### 2. Directory Structure

Each subdirectory within the root represents a distinct service or architectural component (e.g., `koishi`, , `umbilical`, `koishi/kubernetes`, `koishi/devbox`). Refer to individual subdirectories for component-specific rules where a `AGENTS-CONTEXT.md` exists.

### 3. Container Environment (devbox)

*   **Detection:** AI Agents are likely running in a container environment. Confirm this by checking the hostname (`hostname == devbox`).
*   **Environment Details:** The `devbox` container (configured in `koishi/devbox/k8s.app.nur.yaml`) is based on Ubuntu 24.04.
*   **Command Execution & Privileges:**
    - Administrative commands (e.g., `kubectl`, `docker`) require implicit `sudo` authentication and will prompt for a password.
    - **Do NOT** use the `sudo` command explicitly; it is unavailable and will result in a "command not found" error.
    - If a password is required during execution, the user will provide it (the Agent must support user input).
*   **Package Management:** Although based on Ubuntu, `devbox` is configured with the **Nix package manager**. Use Nix to install any required packages.
