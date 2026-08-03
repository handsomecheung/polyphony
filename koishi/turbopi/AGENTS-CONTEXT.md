# TurboPi Webapp - Agent Context

This document provides specialized guidelines and context for AI agents working within the `koishi/turbopi` subdirectory.

---

## Architecture & Integration

The `turbopi` component is a Node.js Express application that proxies camera streams and relays control commands to a physical Hiwonder TurboPi robot car.

### 1. Hardware Communication
- The server acts as a proxy for the TurboPi's camera stream (`CAMERA_STREAM_URL`) and JSON-RPC control endpoint (`RPC_URL`).
- The JSON-RPC endpoint exposes methods like `SetPWMServo` and `GetPWMServo`.
- **JSON-RPC Schema:**
  - Method: `SetPWMServo`
    - Parameters format: `[TIME_MS, SERVO_COUNT, SERVO1_ID, SERVO1_ANGLE, SERVO2_ID, SERVO2_ANGLE, ...]`
    - Example call: `[500, 2, 1, 90, 2, 90]` (sets Servo 1 and Servo 2 to 90 degrees over 500ms).
  - Method: `SetSonarRGB`
    - Parameters format: `[INDEX, RED, GREEN, BLUE]`
    - Example call: `[0, 255, 255, 255]` (turns on both LED headlights to white) or `[0, 0, 0, 0]` (turns them off).

### 2. Environment Variables & Templates
In the Kubernetes deployment ([k8s.app.yaml](file:///mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/turbopi/k8s.app.yaml)), target configurations use a double-underscore template syntax to fetch the physical machine's IP address dynamically at deploy-time:
- `TURBOPI_IP`: `__{{infra.machine.r4b8gTurboPi:f:ip}}__`
- `RPC_URL`: `http://__{{infra.machine.r4b8gTurboPi:f:ip}}__:9030`
- `CAMERA_STREAM_URL`: `http://__{{infra.machine.r4b8gTurboPi:f:ip}}__:8080`

> [!WARNING]
> Do not replace these placeholders with hardcoded IP addresses in the `k8s.app.yaml` file.

### 3. Kubernetes Deployment & Sablier
- This app leverages **Sablier** for scale-to-zero capability.
- In `k8s.app.yaml`, the replica count is initially set to `0` (`replicas: 0`).
- When a user visits the ingress URL (`https://turbopi.domain`), Sablier starts a container instance dynamically.
- **Agent Note:** If you see the deployment replica count is 0 or no pods are running in the namespace, this is normal behavior. Do not try to scale it up manually unless requested.

---

## Agent Instructions & Rules

1. **Modification Policy:**
   - Do not modify files in this directory unless explicitly instructed by the user. If changes are proposed, justify them clearly beforehand.

2. **Bash Scripts:**
   - Any script must start with the shebang `#!/usr/bin/env bash`.

3. **Comment Language:**
   - Keep all comments in code files and scripts in **English** only.

4. **Testing in Offline Environments:**
   - If the actual TurboPi hardware is offline or unreachable, the webapp automatically falls back to **Simulation Mode**.
   - You can test UI adjustments, local endpoints, and proxy fallbacks by checking the browser logs and using the Canvas simulation screen.

---

## Core Operations

### Dev Server
To start the app locally:
```bash
npm install
npm run dev
# or execute
./start_webapp.sh
```

### Build & Deploy
- **Build image (Kaniko):** Runs the in-cluster build tool to compile the docker image.
  ```bash
  ./build.sh
  ```
- **Deploy application:** Triggers the deployment toolchain.
  ```bash
  ./deploy.sh
  ```
