# webreader

`webreader` is an in-cluster HTTP proxy and abstraction service that fetches web pages and converts them into clean, structured Markdown for other services (such as RAG pipelines, AI agents, and indexing tools) across the `koishi` cluster.

## Architecture

`webreader` abstracts the underlying extraction/scraping backend behind a uniform Go `Provider` interface.

```
Caller (gen / prag / etc.)
        │
        ▼
   [ webreader ] ─── (Provider Interface)
                            │
            ┌───────────────┼────────────────┐
            ▼               ▼                ▼
     Jina Reader       (Firecrawl)      (Self-hosted)
    (r.jina.ai)
```

This ensures callers never depend directly on specific third-party APIs or authentication details. When backend implementations are changed or extended (e.g., adding Firecrawl or a self-hosted crawler), caller code remains untouched.

---

## API Reference

### 1. Fetch Markdown (JSON)

#### GET `/v1/markdown`
Fetch a webpage and return a JSON payload containing structured Markdown and metadata.

**Query Parameters:**
- `url` (required): Target webpage URL to read (e.g. `https://example.com/article`).
- `provider` (optional): Reader provider to use (default: `jina`).
- `with_images_summary` (optional): `true` to include image captions/summary.
- `with_links_summary` (optional): `true` to include links summary.
- `no_cache` (optional): `true` to bypass provider cache.
- `wait_for_selector` (optional): CSS selector to wait for before extracting.
- `target_selector` (optional): CSS selector to restrict extraction to.

**Example Request:**
```bash
curl -G "http://webreader/v1/markdown" \
  --data-urlencode "url=https://en.wikipedia.org/wiki/Kubernetes"
```

#### POST `/v1/markdown`
Same as GET `/v1/markdown`, with options passed in JSON body.

**Example Request:**
```bash
curl -X POST "http://webreader/v1/markdown" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://en.wikipedia.org/wiki/Kubernetes",
    "provider": "jina",
    "timeout_seconds": 30,
    "no_cache": false
  }'
```

**Example JSON Response:**
```json
{
  "url": "https://en.wikipedia.org/wiki/Kubernetes",
  "title": "Kubernetes - Wikipedia",
  "description": "An open-source container orchestration system...",
  "content": "# Kubernetes\n\nKubernetes is an open-source container orchestration system...",
  "provider": "jina",
  "status_code": 200,
  "metadata": {
    "duration_ms": 612,
    "usage": {
      "tokens": 1420
    }
  }
}
```

---

### 2. Fetch Raw Markdown

#### GET `/raw`
Returns raw Markdown content directly with `Content-Type: text/markdown; charset=utf-8`.

**Example Request:**
```bash
curl -G "http://webreader/raw" \
  --data-urlencode "url=https://example.com"
```

*Tip:* You can also request raw Markdown from `/v1/markdown` by supplying an `Accept: text/markdown` header.

---

### 3. Service Health & Status

#### GET `/ping` / `/health` / `/ok`
Returns `{"status": "ok"}` for Kubernetes liveness/readiness probes.

#### GET `/v1/providers`
Lists registered reader providers and the default provider.

**Example Response:**
```json
{
  "default": "jina",
  "providers": [
    "jina"
  ]
}
```

---

## Configuration

The service is configured via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the HTTP server to listen on |
| `DEFAULT_PROVIDER` | `jina` | Default scraper provider |
| `JINA_API_KEY` | `""` | Optional API key for Jina Reader authentication |
| `DEFAULT_TIMEOUT_SECONDS` | `45` | Default request timeout in seconds |
| `MAX_TIMEOUT_SECONDS` | `180` | Maximum allowed request timeout in seconds |

---

## Build & Deployment

### Build Container Image
Build the container image using the cluster's Kaniko build pipeline:
```bash
./build.sh
```

### Deploy to Kubernetes
Deploy the service, deployment, and ingress manifests:
```bash
./deploy.sh
```
