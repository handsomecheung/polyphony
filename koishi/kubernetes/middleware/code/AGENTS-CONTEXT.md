# AI Agent Context for Middleware Service

This file provides localized rules and instructions for AI Agents working on the Go-based middleware service.

## Core Rules & Policies

- **Main Entry Point:** Always use `cmd/server/main.go` as the application entry point.
- **Logic Isolation:** Core application logic must remain within the `internal/` directory to ensure proper encapsulation.
- **Configuration:** All application configurations should be handled within `config.go`.
- **Build System:** Use the provided `build.sh` script for building the application binary.
- **Style:** Adhere to standard Go project layouts and formatting.
