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

### 1. Fetch Markdown

#### POST `/v1/markdown`
Fetch a webpage and return a JSON payload containing structured Markdown and metadata.

**Request Body (JSON):**
- `url` (required, string): Target webpage URL to read.
- `provider` (optional, string): Reader provider to use (default: `jina`).
- `language` (optional, string): Language / locale preference (e.g. `zh-CN`, `ja`, `en`), defaults to `DEFAULT_LANGUAGE`.
- `timeout_seconds` (optional, integer): Timeout override for the request.
- `with_links_summary` (optional, boolean): `true` to include links summary.
- `wait_for_selector` (optional, string): CSS selector to wait for before extracting.
- `target_selector` (optional, string): CSS selector to restrict extraction to.
- `custom_headers` (optional, object): Additional HTTP headers to forward to the target/provider.

**Example Request:**
```bash
curl -X POST "http://webreader/v1/markdown" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://en.wikipedia.org/wiki/Kubernetes",
    "provider": "jina",
    "timeout_seconds": 30
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

### 2. Service Health & Status

#### GET `/health`
Returns `{"status": "ok"}` for Kubernetes liveness/readiness probes.

#### GET `/v1/status`
Returns metrics about the current worker pool queue and rate limiting state.

**Example Response:**
```json
{
  "max_concurrent": 1,
  "requests_per_minute": 60,
  "active_requests": 0,
  "waiting_requests": 0
}
```

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
| `DEFAULT_LANGUAGE` | *(required)* | Default language/locale preference (e.g. `zh-CN`, `ja`, `en`). Service fails to start if not set. |
| `JINA_API_KEY` | `""` | Optional API key for Jina Reader authentication |
| `DEFAULT_TIMEOUT_SECONDS` | `45` | Default request timeout in seconds |
| `MAX_TIMEOUT_SECONDS` | `180` | Maximum allowed request timeout in seconds |
| `MAX_CONCURRENT_REQUESTS` | `1` | Worker pool parallel concurrency limit (default: 1 = serialized) |
| `MAX_REQUESTS_PER_MINUTE` | `60` | Maximum allowed outbound requests in any rolling 1-minute window (0 = disabled) |

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
