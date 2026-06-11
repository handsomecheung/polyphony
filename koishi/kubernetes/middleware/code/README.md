# Middleware Service

This project is a Go-based middleware service, part of the Koishi Kubernetes cluster. It provides custom middleware functionality, likely integrated with Traefik or other ingress controllers.

## Project Structure

- `cmd/server/main.go`: The main entry point of the application.
- `internal/`: Contains the core application logic.
    - `handler/handlermitm/handlermitm.go`: Implementation of the core middleware logic.
- `config.go`: Handles application configuration.
- `build.sh`: Script to build the application.
- `go.mod`, `go.sum`: Go module files for dependency management.

## Tech Stack

- **Language:** Go
- **Environment:** Kubernetes (part of the Koishi cluster)
- **Deployment:** Containerized (built via `build.sh`)

## Common Workflows

### Building the Project
Run the build script to compile the binary:
```bash
./build.sh
```

### Dependency Management
Standard Go module commands apply:
```bash
go mod tidy
```
