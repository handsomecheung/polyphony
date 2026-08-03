# TurboPi Webapp Controller

A lightweight Express-based Node.js web application designed to stream video and control the camera mount servos of a Hiwonder TurboPi robot car.

The application proxies camera stream requests (MJPEG) and coordinates servo adjustments via JSON-RPC, providing a clean dashboard UI equipped with simulated fallback mode when the hardware is offline.

---

## Features

- **MJPEG Camera Stream Proxy**: Directly proxies the camera stream from the TurboPi to bypass CORS and simplify routing (`/api/stream`).
- **JSON-RPC Control Proxy**: Relays servo-position requests (`/api/rpc`) to the robot's control server.
- **Interactive Control Panels**: Includes D-Pad, slider controllers, and a virtual joystick for adjusting pitch and yaw angles.
- **LED Light Control**: A toggle switch to control the robot's headlights (ultrasonic sensor's RGB lights) in real-time.
- **Simulated Fallback Mode**: When the physical TurboPi is offline or unreachable, the webapp falls back to a simulated environment that renders servo motions and simulated LED glow effects on an interactive HTML5 Canvas.
- **Terminal Log Console**: Displays real-time JSON-RPC command payloads and connection status directly within the web dashboard.

---

## Directory Structure

```text
turbopi/
├── public/                 # Frontend assets
│   ├── app.js              # UI controller, D-Pad, joystick & Canvas simulation logic
│   ├── index.html          # Web dashboard layout
│   └── style.css           # Custom styles (dark theme, glassmorphism UI)
├── .env.example            # Template for environment variables
├── Dockerfile              # Containerization configuration (production setup)
├── build.sh                # Kaniko container build script
├── deploy.sh               # K8s deploy script using my-k8s-deploy helper
├── k8s.app.yaml            # Kubernetes manifests (Deployment, Ingress with Sablier)
├── server.js               # Node.js Express server acting as the proxy backend
└── start_webapp.sh         # Local startup helper script
```

---

## Configuration

The application requires specific environment variables to target the TurboPi hardware. These can be configured in a `.env` file at the project root.

```bash
# TurboPi target IP address and endpoint configuration
export TURBOPI_IP="1.1.1.100"
export RPC_URL="http://1.1.1.100:9030"
export CAMERA_STREAM_URL="http://1.1.1.100:8080"
```

Refer to [.env.example](file:///mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/turbopi/.env.example) for the default values.

---

## Development Setup

### Prerequisite

- Node.js (version 22 recommended)
- Access to a TurboPi device or the target IP network (optional; simulated mode is available)

### Local Launch

1. Clone or navigate to the subfolder.
2. Install dependencies:
   ```bash
   npm install
   ```
3. Copy the example env file and update the IP values:
   ```bash
   cp .env.example .env
   # Modify .env to match the current TurboPi IP address
   ```
4. Start the application:
   ```bash
   # Option A: using npm scripts
   npm run dev

   # Option B: using the local startup helper
   ./start_webapp.sh
   ```
5. Access the application at `http://localhost:3000`.

### SSH Port Forwarding

If running on a remote devbox:
```bash
ssh -L 3000:localhost:3000 <user>@<devbox-ip>
```
Open `http://localhost:3000` in your local browser.

---

## Deployment (Kubernetes)

This service is part of the `koishi` home-lab monorepo. It features Kaniko in-cluster builds and on-demand activation.

- **In-Cluster Build**: Run `./build.sh` to trigger a Kaniko image build tagged as `cloudpublic/default/turbopi:latest`.
- **Deploy**: Run `./deploy.sh` to apply `k8s.app.yaml` using the cluster tooling.

### Scale-to-Zero (Sablier Middleware)

The deployment is configured to save resources when not in use:
- The default replica count in [k8s.app.yaml](file:///mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/turbopi/k8s.app.yaml) is set to `0`.
- **Sablier** intercepts HTTP requests at the ingress. When a user visits the dashboard, Sablier wakes up the Pod, serves a loading screen, and transfers the traffic once the container is ready.
- The session expires and scales back to zero after 1 hour of inactivity.
