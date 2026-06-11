# AI Agent with Pydantic AI Skills Framework

Generic AI agent framework that dynamically discovers, loads, and orchestrates reusable skills (e.g., slide-generator, prag) through pydantic-ai-skills. Connects to LiteLLM for model routing and provides WebUI, REST API, WebSocket, and CLI interfaces. Skills can be Python, Node.js, or other runtimes, each managing their own dependencies independently.

## Overview

`main.py` implements a FastAPI server that:

- auto-discovers skills from a configurable root directory (`SKILLS_ROOT`)
- routes all model requests through LiteLLM (OpenAI-compatible abstraction)
- provides multiple interfaces: CLI (sync), REST POST /run, WebSocket /ws (async streaming), and GET / (browser-based WebUI)
- includes built-in tools for skills to manage intermediate and output artifacts
- extracts file paths from agent responses (both regex and LLM-based detection)
- health checks on /ping (startup, liveness, readiness probes)

The agent is intentionally generic—it does not hardcode workflows. Skills declare their own interface, dependencies, and runtime. The agent loads and orchestrates them on demand.

## Architecture

### Model & LiteLLM Routing

The agent connects to LiteLLM via OpenAI-compatible interface:

```python
openai_client = AsyncOpenAI(
    api_key=api_key,
    base_url=api_base,  # e.g. http://litellm.default:4000
    timeout=DEFAULT_MODEL_TIMEOUT_SECONDS,
    max_retries=DEFAULT_MODEL_MAX_RETRIES,
)
```

Model selection is controlled by `AI_MODEL` environment variable:

- `balanced-model`: routed by LiteLLM to its default model
- `anthropic/claude-sonnet-4-5`: explicit provider/model routing
- `openai/gpt-5.2`: or any other OpenAI-compatible model slug

### Skills Isolation

Each skill owns its own dependencies. The agent core stays minimal:

**Agent dependencies** (`code/requirements.txt`):
- `pydantic-ai-slim[openai]`
- `pydantic-ai-skills`
- `fastapi`
- `uvicorn[standard]` (required for WebSocket support)

**Skill dependencies**: declared in the skill's own `pyproject.toml`, `requirements.txt`, `package.json`, etc.

### Skills Discovery & Loading

The agent scans `SKILLS_ROOT` at startup (recursively, max depth 4) via `pydantic-ai-skills.SkillsToolset`. Additional directories can be added via `AI_SKILLS_DIRS` (colon-separated on Linux/macOS).

Recommended skill layout:

```text
Skills/
  my-python-skill/
    SKILL.md
    pyproject.toml
    uv.lock
    scripts/
  my-nodejs-skill/
    SKILL.md
    package.json
    package-lock.json
    scripts/
```

Each skill should document its setup and invocation in `SKILL.md`:

```text
## Setup

uv sync

## Run

uv run python scripts/example.py --input /abs/path/input.json
```

### Tool-Driven Workflow

The agent provides the following built-in tools for skills:

| Tool                     | Purpose                                                                    |
|--------------------------|----------------------------------------------------------------------------|
| `allocate_artifact_path` | Reserve an absolute path under `AI_OUTPUT_DIR` for output (before writing) |
| `write_artifact`         | Write text content to a file and return its path                           |
| `read_artifact`          | Read content from a file by path                                           |
| `list_artifacts`         | List all files in the output directory                                     |
| `list_skills`            | List loaded skills                                                         |
| `load_skill`             | Load a specific skill's metadata and available scripts                     |
| `read_skill_resource`    | Read static resources from a skill (e.g., templates)                       |
| `run_skill_script`       | Execute a skill's script with named arguments                              |

When a skill needs to create output files, it must call `allocate_artifact_path` first to obtain an absolute path. This avoids hardcoded relative paths and ensures outputs are discoverable.

## Installation

### Local Development

```bash
cd aiagent/code
python3.12 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

### Docker

```bash
./build.sh
```

Builds `cloudpublic/default/aiagent:latest` using in-cluster Kaniko.

## Configuration

### Required Environment Variables

```bash
export LITELLM_API_BASE=http://127.0.0.1:4000       # or http://litellm.default in Kubernetes
export LITELLM_API_KEY=sk-your-litellm-key
export AI_MODEL=balanced-model                        # or anthropic/claude-sonnet-4-5
export AI_OUTPUT_DIR=/abs/path/to/aiagent-output
export SKILLS_ROOT=/path/to/skills/directory
```

### Optional Environment Variables

```bash
export AI_SKILLS_DIRS=/path/a:/path/b                # Additional skill directories
export AI_MODEL_TIMEOUT_SECONDS=90                    # Default: 90
export AI_MODEL_MAX_RETRIES=0                         # Default: 0
export AI_AGENT_HEARTBEAT_SECONDS=10                  # Heartbeat interval for long-running requests
```

The program exits with an error at startup if any required variable is missing.

## Usage

### CLI (Synchronous)

```bash
python3.12 main.py "Tell me what skills are available"
```

Returns JSON:

```json
{
  "response": "...",
  "files": []
}
```

Example with skill invocation:

```bash
python3.12 main.py "Use the slide-generator skill to create a 15-slide presentation on AI strategies"
```

List loaded skills:

```bash
python3.12 main.py --list-skills
```

### Server (REST + WebSocket)

Start the server:

```bash
python3.12 main.py --serve
```

Listens on `http://127.0.0.1:8000`.

**REST API:**

```bash
curl -X POST http://127.0.0.1:8000/run \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt": "Use the slide-generator skill to create a presentation about AI"
  }'
```

**WebUI:**

Open browser:

```text
http://127.0.0.1:8000/
```

Connects via WebSocket at `ws://127.0.0.1:8000/ws` for async streaming responses.

**Endpoints:**

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/ping` | GET | Health check |
| `/ok` | GET | Status ok |
| `/skills` | GET | List loaded skill names |
| `/download` | GET | Download a file by path |
| `/` | GET | Serve WebUI (index.html) |
| `/run` | POST | Execute agent with prompt (JSON) |
| `/ws` | WebSocket | Async streaming responses |

### WebUI Features

- One prompt textarea
- Send button
- Response area for agent reply
- File list area for output paths
- Real-time streaming via WebSocket

The WebUI requires `uvicorn[standard]` for WebSocket support. If you see "No supported WebSocket library detected", ensure `requirements.txt` includes `uvicorn[standard]`.

## Response Format

All responses follow this JSON schema:

```json
{
  "response": "Natural language answer from the agent",
  "files": [
    "/abs/path/to/file1",
    "/abs/path/to/file2"
  ]
}
```

Notes:

- `response` is the main text output from the agent
- `files` contains only real files that exist on disk
- both regex patterns and LLM-based extraction detect file paths mentioned in the response
- intermediate files are excluded unless explicitly presented as results by the model

## Deployment

### Kubernetes

Deploy to Kubernetes:

```bash
./deploy.sh
```

This calls `my-k8s-deploy --file=k8s.app.yaml`.

**Key deployment config** (`k8s.app.yaml`):

- **Deployment**: 1 replica on node `nur` (nodeSelector)
- **Image**: `cloudpublic/default/aiagent:latest` with `Always` pull policy
- **Mounts**:
  - `/data/skills` → external skills directory (read-only)
  - `/data/output` → artifact output directory (read-write)
- **Health Probes**:
  - Startup: 30 attempts × 10s (300s total)
  - Liveness: every 10s
  - Readiness: every 10s
- **Ingress**: Traefik with ACME (Cloudflare DNS-01), optional SSO middleware, gethomepage.dev annotations
- **Service**: ClusterIP on port 80 → container port 8000
- **Resources**: 100m CPU, 256Mi memory request

Environment variables in pod (note `__{{koishi.litellm}}__` is a template placeholder injected at deploy time):

```yaml
AI_MODEL: balanced-model
LITELLM_API_BASE: http://litellm.default
LITELLM_API_KEY: __{{koishi.litellm}}__
AI_OUTPUT_DIR: /data/output
SKILLS_ROOT: /data/skills
```

### Local Development

Use `code/run.sh` (with hardcoded paths for local testing):

```bash
cd code
source ../.env
bash run.sh
```

## File Structure

| Path | Purpose |
|------|---------|
| `code/main.py` | FastAPI server, CLI runner, agent orchestration, artifact tools |
| `code/requirements.txt` | Core dependencies |
| `code/webui/index.html` | Browser-based UI with WebSocket client |
| `code/run.sh` | Local development entry point |
| `Dockerfile` | Python slim base; installs uv, curl, jq; runs main.py --serve |
| `k8s.app.yaml` | Kubernetes Deployment, Service, Ingress |
| `build.sh` | Builds container image via `my-k8s-build-image` |
| `deploy.sh` | Deploys to Kubernetes via `my-k8s-deploy` |
| `output/` | Generated artifacts (mounted as `/data/output` in Kubernetes) |
| `.env` | LiteLLM credentials and model config |

## Skill Development Guidelines

### Python Skills

Use `pyproject.toml` with `uv`:

```toml
[project]
name = "my-skill"
dependencies = [
    "pydantic>=2.0",
]
```

Commit `uv.lock` for reproducibility.

Scripts accept named CLI arguments:

```bash
uv run python scripts/example.py --input /abs/path/input.json --output /abs/path/output.json
```

### Node.js Skills

Use `package.json` with npm or pnpm:

```json
{
  "name": "my-skill",
  "scripts": {
    "start": "node scripts/example.js"
  }
}
```

Commit `package-lock.json` or `pnpm-lock.yaml`.

### General Rules

- **Documentation**: Include `SKILL.md` with setup and run instructions
- **Isolation**: Each skill owns its dependencies; do not merge into agent core
- **Portability**: Skills should be runnable on any host with the declared runtime
- **Paths**: Use `allocate_artifact_path` for outputs, avoid hardcoded relative paths
- **Entrypoints**: Declare scripts and their CLI arguments in `SKILL.md` or skill metadata

## Troubleshooting

### WebSocket Errors

**Error**: "No supported WebSocket library detected"

**Solution**: Ensure `uvicorn[standard]` is in `requirements.txt`, not just `uvicorn`.

### LiteLLM Connection

**Error**: Model request fails, "Provider response: ..."

**Cause**: Mapped backend model unavailable, requires payment, or LiteLLM routing is incorrect.

**Check**:
- `LITELLM_API_BASE` points to correct LiteLLM instance
- `AI_MODEL` matches available models (e.g., list via LiteLLM admin)
- API key has correct permissions

### Skill Not Found

**Error**: Agent says "skill not available"

**Check**:
```bash
python3.12 main.py --list-skills
```

Ensure skill is in `SKILLS_ROOT` or `AI_SKILLS_DIRS`. Verify directory structure and `SKILL.md` exists.

### Output Files Not Detected

If the agent generates files but they don't appear in the `files` list:

- Ensure skill calls `allocate_artifact_path` before writing
- Verify paths are absolute and exist on disk
- Check `AI_OUTPUT_DIR` permissions

## Related

- [Koishi Cluster README](../README.md) — parent infrastructure
- [Pydantic AI documentation](https://ai.pydantic.dev/)
- [pydantic-ai-skills library](https://github.com/pydantic/pydantic-ai-skills)
