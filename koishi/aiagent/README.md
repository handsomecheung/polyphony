# pydantic-ai skills agent demo

This is a minimal example of a generic AI agent with skill support, located in `aiagent/code`.

The goal is not to hardcode a single workflow. Instead, this demo shows how to:

- connect a model through LiteLLM
- auto-discover and load skills with `pydantic-ai-skills`
- let the agent use `list_skills`, `load_skill`, `read_skill_resource`, and `run_skill_script` as needed
- provide a few generic local file tools for intermediate files and generated artifacts
- return the main textual response separately from detected file paths

This repository currently includes one example skill:

- `aiagent/skills/slide-generator`

`main.py` does not hardcode the `slide-generator` workflow anymore. It is just one example skill the agent can choose to use.

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

## Environment Variables

This demo uses LiteLLM, and the following environment variables are required:

```bash
export LITELLM_API_BASE=http://127.0.0.1:4000
export LITELLM_API_KEY=sk-your-litellm-key
export AI_MODEL=openai/gpt-5.2
```

If any of them are missing, the program exits at startup with an error.

If your LiteLLM instance routes to a different model, you can change `AI_MODEL`, for example:

```bash
export AI_MODEL=anthropic/claude-sonnet-4-5
```

If you are using a different OpenAI-compatible endpoint behind LiteLLM, update `LITELLM_API_BASE` accordingly.

## Skills

By default, the agent scans:

```bash
aiagent/skills
```

The example local skill in this repository lives here:

```bash
aiagent/skills/slide-generator
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

Then call the API:

```bash
curl -X POST http://127.0.0.1:8000/run \
  -H 'Content-Type: application/json' \
  -d '{
    "prompt": "Use the slide-generator skill to create a presentation about AI opportunities in retail"
  }'
```

## Local Artifact Tools

The agent includes a few generic local file tools so skills can write intermediate files or outputs:

- `write_artifact`
- `read_artifact`
- `list_artifacts`

The default artifact directory is:

```bash
aiagent/output
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

When a skill needs output paths, the generic agent provides `allocate_artifact_path`, so the skill can reserve absolute paths under `aiagent/output` instead of writing into its own `scripts/` directory.
