# Prag (RAG Service)

`prag` is a pure Retrieval-Augmented Generation (RAG) tool designed to provide relevant text chunks for downstream LLM processing. It indexes markdown notes and provides a vector search API.

## Project Structure

- `code/`: Core application logic and scripts.
    - `common.py`: Centralized configuration and shared Redis/OpenAI clients.
    - `indexer.py`: Script to scan, chunk, embed, and store notes in Redis.
    - `search.py`: Pure retrieval logic.
    - `server.py`: FastAPI application.
    - `.env`: Local environment variables (managed via `pipenv`).
    - `Pipfile`: Dependency management.
- `Dockerfile`: Container definition for the RAG service.
- `build.sh`: Script to build the Docker image (`cloudpublic/default/rag:latest`).
- `deploy.sh`: Script to deploy to Kubernetes.
- `k8s.app.yaml`: Kubernetes Deployment and Service configuration.

## Tech Stack

- **Language:** Python 3.12+
- **Vector Database:** Redis Stack (RediSearch)
- **Embedding Model:** `bge-m3` (via Ollama)
- **Framework:** FastAPI
- **Key Libraries:** `redis`, `openai`, `langchain-text-splitters`, `numpy`, `pydantic`.

## Workflows & Commands

### Local Development
1. Initialize Environment:
   ```bash
   cd code && pipenv install
   ```
2. Run Server Locally (requires `.env`):
   ```bash
   cd code && ./run.server.sh
   ```

### Build & Deploy
1. Build Image:
   ```bash
   ./build.sh
   ```
2. Deploy to K8s:
   ```bash
   ./deploy.sh
   ```

## API Endpoints

- `GET /query?q=...&top_k=5`: Perform a vector search. Returns a JSON list with `content`, `score`, and `source`.
- `GET /health`: Basic health check.
- `POST /index`: Manually trigger the re-indexing process.

## Technical Details

- **Redis Index:** `prag_default` using the `HNSW` algorithm and `COSINE` distance.
- **Chunking Strategy:** `RecursiveCharacterTextSplitter` (chunk_size: 500, overlap: 50).
- **Vector Dimension:** 1024.
- **Storage:** Mounts `/mnt/coder-workspaces/private-workspace/repos/local/notebook/binder` to `/data/notes` on the `nur` node.
