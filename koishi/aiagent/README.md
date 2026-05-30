# pydantic-ai skills agent demo

This is a minimal example of a generic AI agent with skill support, located in `aiagent/code`.

The goal is not to hardcode a single workflow. Instead, this demo shows how to:

- connect a model through LiteLLM
- auto-discover and load skills with `pydantic-ai-skills`
- let the agent use `list_skills`, `load_skill`, `read_skill_resource`, and `run_skill_script` as needed
- provide a few generic local file tools for intermediate files and generated artifacts
- return the main textual response separately from detected file paths

This repository currently points `SKILLS_ROOT` at an external skills directory, for example:

- `/mnt/coder-workspaces/private-workspace/repos/local/notebook/hardback/AI/Skills`

`main.py` does not hardcode any single workflow anymore. `slide-generator` is just one example skill the agent can choose to use when it exists under the configured skills root.

## Installation

```bash
cd aiagent/code
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

Installed dependencies include:

- `pydantic-ai-slim[openai]`
- `pydantic-ai-skills`
- `uvicorn[standard]` for WebSocket support

## Environment Variables

This demo uses LiteLLM, and the following environment variables are required:

```bash
export LITELLM_API_BASE=http://127.0.0.1:4000
export LITELLM_API_KEY=sk-your-litellm-key
export AI_MODEL=openai/gpt-5.2
export AI_OUTPUT_DIR=/abs/path/to/aiagent-output
export SKILLS_ROOT=/mnt/coder-workspaces/private-workspace/repos/local/notebook/hardback/AI/Skills
```

If any of them are missing, the program exits at startup with an error.

If your LiteLLM instance routes to a different model, you can change `AI_MODEL`, for example:

```bash
export AI_MODEL=anthropic/claude-sonnet-4-5
```

If you are using a different OpenAI-compatible endpoint behind LiteLLM, update `LITELLM_API_BASE` accordingly.

## Skills

The agent always scans `SKILLS_ROOT` first. In the current local setup:

```bash
export SKILLS_ROOT=/mnt/coder-workspaces/private-workspace/repos/local/notebook/hardback/AI/Skills
```

For example, `slide-generator` would live here:

```bash
/mnt/coder-workspaces/private-workspace/repos/local/notebook/hardback/AI/Skills/slide-generator
```

You can also add more skill directories with `AI_SKILLS_DIRS`:

```bash
export AI_SKILLS_DIRS=/abs/path/to/my-skills
```

Example with multiple directories on Linux/macOS:

```bash
export AI_SKILLS_DIRS=/abs/path/skills-a:/abs/path/skills-b
```

To verify what was loaded:

```bash
python3 main.py --list-skills
```

## Skill Dependencies

The recommended rule is:

- the agent owns host-level dependencies
- each skill owns its own runtime dependencies

In practice, `aiagent/code/requirements.txt` should only contain dependencies needed to run the generic agent itself, such as:

- `pydantic-ai-slim[openai]`
- `pydantic-ai-skills`
- `uvicorn[standard]`

Do not merge every skill dependency into the agent's root environment. Different skills may use different runtimes, dependency managers, and system libraries. Keeping them local to the skill avoids version conflicts and makes each skill portable.

Recommended layout:

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

Suggested conventions by runtime:

- Python skill: use `pyproject.toml`, and preferably commit `uv.lock`. If the skill is simple, `requirements.txt` is also acceptable.
- Node.js skill: use `package.json`, and commit a lockfile such as `package-lock.json` or `pnpm-lock.yaml`.
- Other runtimes: keep dependency manifests in the skill directory and document install and run steps in `SKILL.md`.

Recommended operational contract:

- the agent discovers and loads skills
- the skill declares how its scripts should be installed and run
- dependency installation happens per skill, not globally across all skills

Each skill should document its dependency bootstrap clearly in `SKILL.md`, for example:

```text
## Setup

Python:

uv sync

Run:

uv run python scripts/example.py --input /abs/path/input.json
```

Or for Node.js:

```text
## Setup

npm ci

Run:

npm run start -- --input /abs/path/input.json
```

This keeps ownership clear:

- `aiagent/code` stays small and stable
- each skill can evolve its own dependencies independently
- Python, Node.js, and future runtimes can coexist under the same `SKILLS_ROOT`

## CLI Usage

Run the generic agent like this:

```bash
python3 main.py "Tell me what skills are available and how I should use them"
```

Example output:

```json
{
  "response": "...",
  "files": []
}
```

If you want the agent to use `slide-generator`, prompt it explicitly:

```bash
python3 main.py "Use the slide-generator skill to create a presentation about generative AI rollout strategy"
```

## API Usage

Start the server:

```bash
python3 main.py --serve
```

Then call the HTTP API:

```bash
curl -X POST http://127.0.0.1:8000/run \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt": "Use the slide-generator skill to create a presentation about AI opportunities in retail"
  }'
```

## WebUI Usage

When the server is started with:

```bash
python3 main.py --serve
```

open this page in your browser:

```text
http://127.0.0.1:8000/
```

The WebUI connects to the server through WebSocket:

```text
ws://127.0.0.1:8000/ws
```

If WebSocket upgrade requests fail with a warning such as `No supported WebSocket library detected`, rebuild the image after installing `uvicorn[standard]` from `requirements.txt`.

UI behavior:

- one prompt textarea
- one send button
- one response area for the server reply
- one file list area for returned output paths

## Local Artifact Tools

The agent includes a few generic local file tools so skills can write intermediate files or outputs:

- `write_artifact`
- `read_artifact`
- `list_artifacts`

The artifact directory is configured by the required `AI_OUTPUT_DIR` environment variable, for example:

```bash
export AI_OUTPUT_DIR=/abs/path/to/aiagent-output
```

If a skill needs to write JSON, Markdown, or some other content to disk before passing a path into `run_skill_script`, these tools are available.

## Response Format

The agent returns:

```json
{
  "response": "...",
  "files": [
    "/abs/path/to/file1",
    "/abs/path/to/file2"
  ]
}
```

Notes:

- `response` is the main natural-language answer from the agent
- `files` is a separate list of real files detected from the agent response
- only paths that actually exist on disk are kept
- this is intended to include final output files and relevant cited source files
- intermediate files should not be included unless the model explicitly presents them as relevant

## About `slide-generator`

`slide-generator` is only an example skill, but it has been updated to work correctly with `pydantic-ai-skills`.

Its scripts now accept named CLI arguments, for example:

- `template_schema.py --template_name my-template`
- `content_linter.py --template_name my-template --content_path /abs/path/content.json`
- `ppt_renderer.py --template my-template --content /abs/path/content.json --output /abs/path/output.pptx`

That matters because `pydantic-ai-skills` passes script arguments as named flags through the `args` dictionary in `run_skill_script(...)`.

When a skill needs output paths, the generic agent provides `allocate_artifact_path`, so the skill can reserve absolute paths under `AI_OUTPUT_DIR` instead of writing into its own `scripts/` directory.
