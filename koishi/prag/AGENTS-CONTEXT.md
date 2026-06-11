# AI Agent Context for Prag

This file provides localized rules and instructions for AI Agents working on the `prag` component.

## Core Rules & Policies

- **Modification Policy:** Do not modify the code unless explicitly requested. Explain the reason and obtain consent first.
- **Data Isolation:** Always use Redis Stack (DB 0) with the prefix `prag:default:` for vector storage.
- **Model Routing:** Use Ollama (`/v1/embeddings`) for generating vectors.
- **Configuration:** All configurations must be handled via environment variables managed by `pipenv` and `.env` in the `code/` directory.
- **Code Integrity:** Maintain strict separation of concerns:
    - `common.py`: Configuration and clients.
    - `indexer.py`: Ingestion logic.
    - `search.py`: Retrieval logic.
    - `server.py`: FastAPI server.

## Technical Specifications

- **Vector Dimension:** 1024 (must match `bge-m3` output).
- **Index Configuration:** Name is `prag_default`, using `HNSW` algorithm and `COSINE` distance.
- **Chunking Strategy:** Use `RecursiveCharacterTextSplitter` with `chunk_size=500` and `chunk_overlap=50`.
- **Output Format:** API must return a JSON list of objects containing `content`, `score`, and `source`.
