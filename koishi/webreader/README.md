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
- `mode` (optional, string): Fetch mode — controls how the page is retrieved.
  - `"static"` (default): Fetches raw HTML without JavaScript rendering (via Jina Reader).
  - `"rendered"`: Fetches the page after full JavaScript execution, e.g. for SPAs (via Firecrawl).
- `language` (optional, string): Language / locale preference (e.g. `zh-CN`, `ja`, `en`), defaults to `DEFAULT_LANGUAGE`.
- `remove_media` (optional, string): Media filter mode — `"on"` removes image URLs, video, audio, embedded media, and other non-text media while preserving image alternative text (for example, a pagination link label); `"off"` (default) preserves them.
- `timeout_seconds` (optional, integer): Timeout override for the request.
- `cache` (optional, string): `"on"` reads from cache and writes fresh results; `"off"` bypasses cache reads and does not write results; when omitted, cached results may be read but fresh results are not written.
- `custom_headers` (optional, object): Additional HTTP headers to forward to the target/provider.
- `actions` (optional, array of objects): Browser interaction steps to execute before content extraction (supported when `mode: "rendered"`). Includes actions like `click`, `write`, `wait`, `executeJavascript`, etc. Different action sequences generate distinct cache keys.

Requests with `custom_headers` bypass cache reads and writes, preventing a response derived from credentials or user-specific headers from being shared.

**Example Request:**
```bash
curl -X POST "http://webreader/v1/markdown" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://en.wikipedia.org/wiki/Kubernetes",
    "mode": "static",
    "timeout_seconds": 30
  }'
```

**Example Request (JS-rendered page with browser actions):**
```bash
curl -X POST "http://webreader/v1/markdown" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/search-page",
    "mode": "rendered",
    "actions": [
      {
        "type": "executeJavascript",
        "script": "document.querySelector(\"#search\").value = \"query\"; document.querySelector(\"#form\").submit();"
      },
      {
        "type": "wait",
        "milliseconds": 3000
      }
    ]
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
| `DEFAULT_LANGUAGE` | *(required)* | Default language/locale preference (e.g. `zh-CN`, `ja`, `en`). Service fails to start if not set. |
| `JINA_API_KEY` | `""` | Optional API key for Jina Reader authentication |
| `FIRECRAWL_APIKEY` | `""` | API key for Firecrawl (required when `mode: "rendered"` is used) |
| `REDIS_URL` | *(required)* | Redis connection URL, e.g. `redis://:PASSWORD@redis.ck-dev:6379` |
| `REDIS_CACHE_TTL_SECONDS` | *(required)* | Positive cache expiration in seconds; service fails to start if absent or invalid |
| `CACHE_KEY_PREFIX` | `webreader` | Prefix applied to every Redis cache key (for example, `webreader:markdown:v1:…`) |
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
