#!/usr/bin/env python3.12

from __future__ import annotations

import asyncio
import json
import os
import re
import sys
import threading
import time
import traceback
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

from fastapi import FastAPI, HTTPException, Query, WebSocket, WebSocketDisconnect
from fastapi.responses import FileResponse
from openai import AsyncOpenAI
from pydantic import BaseModel, Field
from pydantic_ai import Agent, RunContext
from pydantic_ai.exceptions import ModelHTTPError
from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.litellm import LiteLLMProvider
from pydantic_ai_skills import SkillsToolset


class RunRequest(BaseModel):
    prompt: str = Field(description="User request for the generic AI agent.")


class ArtifactResult(BaseModel):
    path: str
    bytes_written: int


class ArtifactPathResult(BaseModel):
    path: str


class FileExtractionResult(BaseModel):
    files: list[str] = Field(default_factory=list)


class RunResult(BaseModel):
    response: str
    files: list[str] = Field(default_factory=list)


class SkillsResult(BaseModel):
    skills: list[str] = Field(default_factory=list)


@dataclass
class AgentDeps:
    output_dir: Path


WEBUI_DIR = Path(__file__).resolve().parent / "webui"
DEFAULT_MODEL_TIMEOUT_SECONDS = float(os.environ.get("AI_MODEL_TIMEOUT_SECONDS", "90"))
DEFAULT_MODEL_MAX_RETRIES = int(os.environ.get("AI_MODEL_MAX_RETRIES", "0"))
DEFAULT_HEARTBEAT_SECONDS = int(os.environ.get("AI_AGENT_HEARTBEAT_SECONDS", "10"))


def log(message: str) -> None:
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{timestamp}] {message}", file=sys.stderr, flush=True)


def log_exception(context: str, exc: Exception) -> None:
    log(f"{context}: {exc}")
    formatted = "".join(traceback.format_exception(type(exc), exc, exc.__traceback__)).rstrip()
    if formatted:
        log(formatted)


def get_required_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"Required environment variable {name} is not set.")
    return value


SKILLS_ROOT = Path(get_required_env("SKILLS_ROOT")).resolve()


def get_required_path_env(name: str) -> Path:
    return Path(get_required_env(name)).expanduser().resolve()


OUTPUT_DIR = get_required_path_env("AI_OUTPUT_DIR")


def build_model() -> OpenAIChatModel:
    model_name = get_required_env("AI_MODEL")
    api_base = get_required_env("LITELLM_API_BASE")
    api_key = get_required_env("LITELLM_API_KEY")
    log(
        "Building LiteLLM model client "
        f"(AI_MODEL={model_name}, LITELLM_API_BASE={api_base}, "
        f"timeout={DEFAULT_MODEL_TIMEOUT_SECONDS}s, max_retries={DEFAULT_MODEL_MAX_RETRIES})"
    )
    openai_client = AsyncOpenAI(
        api_key=api_key,
        base_url=api_base,
        timeout=DEFAULT_MODEL_TIMEOUT_SECONDS,
        max_retries=DEFAULT_MODEL_MAX_RETRIES,
    )
    return OpenAIChatModel(
        model_name,
        provider=LiteLLMProvider(
            openai_client=openai_client,
        ),
    )


def slugify(value: str) -> str:
    slug = re.sub(r"[^a-zA-Z0-9]+", "-", value).strip("-").lower()
    return slug[:48] or "slides"


def make_output_stem(filename_hint: str | None = None) -> str:
    if filename_hint:
        return slugify(filename_hint)
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    return f"{timestamp}-artifact"


def get_skill_directories() -> list[str]:
    extra_dirs = os.environ.get("AI_SKILLS_DIRS", "")
    directories = [str(SKILLS_ROOT)]
    if extra_dirs:
        directories.extend(path for path in extra_dirs.split(os.pathsep) if path)

    deduped_directories: list[str] = []
    seen: set[str] = set()
    for directory in directories:
        normalized = str(Path(directory).resolve())
        if normalized in seen:
            continue
        seen.add(normalized)
        deduped_directories.append(normalized)

    log(f"Resolved skill directories: {deduped_directories}")
    return deduped_directories


def build_skills_toolset() -> SkillsToolset:
    log("Loading skills toolset")
    return SkillsToolset(
        directories=get_skill_directories(),
        validate=True,
        max_depth=4,
    )


def format_model_error(exc: Exception) -> str:
    if isinstance(exc, ModelHTTPError):
        return (
            f"Model request failed for AI_MODEL={exc.model_name} via LiteLLM.\n"
            f"LITELLM_API_BASE={os.environ.get('LITELLM_API_BASE', '')}\n"
            f"Provider response: {exc.body}\n"
            "Check whether the mapped backend model is available, requires payment/subscription, "
            "or whether your LiteLLM routing points to the expected provider."
        )
    return str(exc)


skills_toolset = build_skills_toolset()
loaded_skills = getattr(skills_toolset, "skills", {})
if loaded_skills:
    log(f"Loaded skills: {', '.join(sorted(loaded_skills.keys()))}")
else:
    log("Loaded skills: none")


def get_loaded_skill_names() -> list[str]:
    return sorted(str(name) for name in loaded_skills.keys())

BASE_INSTRUCTIONS = (
    "You are a general-purpose AI agent with skill support. "
    "When a task matches an available skill, first call list_skills, then load_skill, "
    "and follow that skill's instructions. "
    "Use read_skill_resource or run_skill_script only after load_skill succeeds. "
    "When scripts require file paths or intermediate files, use the local artifact tools. "
    "If a skill needs to create files, always call allocate_artifact_path first to get an absolute path. "
    "Do not invent relative output filenames such as report.pptx or content.json. "
    "Be explicit about what you are doing and do not claim success unless the relevant tool or script has succeeded."
)


def add_output_rules() -> str:
    return (
        "If you use skills, prefer exact script/resource names from load_skill output. "
        "For file-based scripts in pydantic-ai-skills, args are passed as named CLI flags via the args dictionary. "
        "When a skill needs an output path or intermediate file path, first use allocate_artifact_path and then pass that absolute path into the script args."
    )


def build_agent() -> Agent[AgentDeps, str]:
    agent = Agent(
        build_model(),
        deps_type=AgentDeps,
        output_type=str,
        toolsets=[skills_toolset],
        instructions=BASE_INSTRUCTIONS,
    )
    agent.instructions(add_output_rules)
    agent.tool(write_artifact)
    agent.tool(allocate_artifact_path)
    agent.tool(read_artifact)
    agent.tool(list_artifacts)
    return agent


def write_artifact(
    ctx: RunContext[AgentDeps],
    content: str,
    filename_hint: str | None = None,
    extension: str | None = None,
) -> ArtifactResult:
    ext = extension or ".txt"
    if not ext.startswith("."):
        ext = f".{ext}"
    stem = make_output_stem(filename_hint)
    ctx.deps.output_dir.mkdir(parents=True, exist_ok=True)
    artifact_path = ctx.deps.output_dir / f"{stem}{ext}"
    artifact_path.write_text(content, encoding="utf-8")
    size = artifact_path.stat().st_size
    log(f"write_artifact saved file: {artifact_path} ({size} bytes)")
    return ArtifactResult(path=str(artifact_path), bytes_written=size)


def allocate_artifact_path(
    ctx: RunContext[AgentDeps],
    filename_hint: str | None = None,
    extension: str | None = None,
) -> ArtifactPathResult:
    ext = extension or ".txt"
    if not ext.startswith("."):
        ext = f".{ext}"
    stem = make_output_stem(filename_hint)
    ctx.deps.output_dir.mkdir(parents=True, exist_ok=True)
    artifact_path = ctx.deps.output_dir / f"{stem}{ext}"
    log(f"allocate_artifact_path reserved path: {artifact_path}")
    return ArtifactPathResult(path=str(artifact_path))


def read_artifact(_: RunContext[AgentDeps], path: str) -> str:
    artifact_path = Path(path)
    if not artifact_path.exists():
        raise RuntimeError(f"Artifact does not exist: {artifact_path}")
    content = artifact_path.read_text(encoding="utf-8")
    log(f"read_artifact loaded file: {artifact_path} ({len(content)} chars)")
    return content


def list_artifacts(ctx: RunContext[AgentDeps]) -> list[str]:
    ctx.deps.output_dir.mkdir(parents=True, exist_ok=True)
    artifacts = sorted(str(path) for path in ctx.deps.output_dir.iterdir() if path.is_file())
    log(f"list_artifacts found {len(artifacts)} files")
    return artifacts


def get_output_files(output_dir: Path) -> list[str]:
    output_dir.mkdir(parents=True, exist_ok=True)
    return sorted(str(path) for path in output_dir.iterdir() if path.is_file())


def normalize_candidate_file_path(candidate: str) -> Path | None:
    raw = candidate.strip()
    if not raw:
        return None
    raw = raw.strip("`")
    raw = raw.strip()
    raw = raw.rstrip(".,;")
    raw = raw.strip("'\"")
    if not raw:
        return None

    path = Path(raw).expanduser()
    if path.exists():
        return path.resolve()

    if not path.is_absolute():
        cwd_path = (Path.cwd() / path).resolve()
        if cwd_path.exists():
            return cwd_path

    return None


def filter_existing_files(candidates: list[str]) -> list[str]:
    existing: list[str] = []
    seen: set[str] = set()
    for candidate in candidates:
        normalized = normalize_candidate_file_path(candidate)
        if normalized is None:
            continue
        normalized_str = str(normalized)
        if normalized_str in seen:
            continue
        seen.add(normalized_str)
        existing.append(normalized_str)
    return existing


def extract_paths_by_regex(text: str) -> list[str]:
    patterns = [
        r"`(/[^`\n]+)`",
        r"(?<!\w)(/[^\s`'\"<>]+)",
    ]
    matches: list[str] = []
    for pattern in patterns:
        matches.extend(re.findall(pattern, text))
    return matches


def build_file_extractor_agent() -> Agent[None, FileExtractionResult]:
    return Agent(
        build_model(),
        output_type=FileExtractionResult,
        instructions=(
            "Extract file paths from the given assistant response. "
            "Return only paths that are explicitly mentioned in the text. "
            "Prefer absolute paths. "
            "Do not invent paths. "
            "Do not include intermediate files unless the text explicitly presents them as relevant result files or cited source files."
        ),
    )


def extract_files_from_response(response_text: str) -> list[str]:
    regex_candidates = extract_paths_by_regex(response_text)
    log(f"Regex file extraction found {len(regex_candidates)} candidate paths")

    llm_candidates: list[str] = []
    try:
        extractor = build_file_extractor_agent()
        extraction = extractor.run_sync(
            "Extract file paths from this response:\n\n"
            f"{response_text}"
        )
        llm_candidates = extraction.output.files
        log(f"LLM file extraction found {len(llm_candidates)} candidate paths")
    except Exception as exc:
        log(f"LLM file extraction failed, falling back to regex-only: {exc}")

    combined = regex_candidates + llm_candidates
    return filter_existing_files(combined)


def start_run_heartbeat() -> tuple[threading.Event, threading.Thread]:
    stop_event = threading.Event()

    def heartbeat() -> None:
        elapsed = 0
        while not stop_event.wait(DEFAULT_HEARTBEAT_SECONDS):
            elapsed += DEFAULT_HEARTBEAT_SECONDS
            log(
                "Still waiting for agent/model execution "
                f"({elapsed}s elapsed, heartbeat={DEFAULT_HEARTBEAT_SECONDS}s)"
            )

    thread = threading.Thread(target=heartbeat, daemon=True, name="agent-run-heartbeat")
    thread.start()
    return stop_event, thread


def run_agent(request: RunRequest) -> RunResult:
    log(
        "Starting agent request "
        f"(prompt={request.prompt})"
    )
    deps = AgentDeps(output_dir=OUTPUT_DIR)
    log(f"Using output directory: {deps.output_dir}")
    log("Invoking agent.run_sync; waiting for model + skill workflow to complete")
    agent = build_agent()
    heartbeat_stop, heartbeat_thread = start_run_heartbeat()
    try:
        result = agent.run_sync(request.prompt, deps=deps)
    finally:
        heartbeat_stop.set()
        heartbeat_thread.join(timeout=1)
    existing_files = extract_files_from_response(result.output)
    log(
        f"Agent completed request (response_chars={len(result.output)}, files={len(existing_files)})"
    )
    return RunResult(response=result.output, files=existing_files)


app = FastAPI(title="PydanticAI Skills Agent Demo")


@app.get("/ok")
def ok() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/ping")
def ping() -> str:
    return "pong"


@app.get("/skills", response_model=SkillsResult)
def skills() -> SkillsResult:
    return SkillsResult(skills=get_loaded_skill_names())


@app.get("/download")
def download(path: str = Query(..., description="Absolute file path to download.")) -> FileResponse:
    file_path = Path(path).expanduser().resolve()
    if not file_path.exists() or not file_path.is_file():
        raise HTTPException(status_code=404, detail="File not found.")
    return FileResponse(file_path, filename=file_path.name)


@app.get("/")
def webui() -> FileResponse:
    return FileResponse(WEBUI_DIR / "index.html")


@app.post("/run", response_model=RunResult)
def run(request: RunRequest) -> RunResult:
    try:
        log("Received HTTP request: POST /run")
        return run_agent(request)
    except Exception as exc:
        log_exception("HTTP request failed", exc)
        raise HTTPException(status_code=500, detail=format_model_error(exc)) from exc


@app.websocket("/ws")
async def websocket_run(websocket: WebSocket) -> None:
    await websocket.accept()
    log("WebSocket client connected")
    try:
        while True:
            payload = await websocket.receive_json()
            prompt = str(payload.get("prompt", "")).strip()
            if not prompt:
                await websocket.send_json(
                    {
                        "type": "error",
                        "message": "Prompt is required.",
                    }
                )
                continue

            await websocket.send_json(
                {
                    "type": "status",
                    "message": "Running agent...",
                }
            )
            try:
                result = await asyncio.to_thread(run_agent, RunRequest(prompt=prompt))
            except Exception as exc:
                log_exception("WebSocket request failed", exc)
                await websocket.send_json(
                    {
                        "type": "error",
                        "message": format_model_error(exc),
                    }
                )
                continue

            await websocket.send_json(
                {
                    "type": "result",
                    "data": result.model_dump(),
                }
            )
    except WebSocketDisconnect:
        log("WebSocket client disconnected")


if __name__ == "__main__":
    import argparse

    import uvicorn

    parser = argparse.ArgumentParser(description="PydanticAI generic skills agent demo")
    parser.add_argument("--serve", action="store_true", help="Run FastAPI server")
    parser.add_argument("--list-skills", action="store_true", help="Print loaded skills and exit")
    parser.add_argument("prompt", nargs="?", help="Prompt for the generic agent")
    args = parser.parse_args()

    if args.list_skills:
        log("Listing loaded skills")
        print(f"Skill directories: {', '.join(get_skill_directories())}")
        skills = getattr(skills_toolset, "skills", {})
        if not skills:
            print("No skills loaded.")
        else:
            for name, skill in skills.items():
                description = getattr(skill, "description", None)
                if not description and isinstance(getattr(skill, "metadata", None), dict):
                    description = skill.metadata.get("description")
                print(f"- {name}: {description or '(no description)'}")
    elif args.serve:
        log("Starting FastAPI server on 0.0.0.0:8000")
        uvicorn.run(app, host="0.0.0.0", port=8000)
    else:
        if not args.prompt:
            raise SystemExit("A prompt is required unless --serve or --list-skills is used.")

        payload = RunRequest(prompt=args.prompt)
        try:
            log("Running CLI agent request")
            print(json.dumps(run_agent(payload).model_dump(), ensure_ascii=False, indent=2))
        except Exception as exc:
            log_exception("CLI request failed", exc)
            raise SystemExit(format_model_error(exc)) from exc
