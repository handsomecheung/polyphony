# Finance Services (Seek & Tempo)

Finance services provide Kubernetes-deployed financial data ingestion, storage, and query capabilities for stock and cryptocurrency prices. **Tempo** is a scheduled data fetcher (CronJob) that polls Yahoo Finance daily for GOOG, SPY, VGT, BTC-USD, ETH-USD, and ADA-USD prices. **Seek** is a FastAPI query service exposing price data via HTTP with dual access patterns: internal routing (TLS-secured DNS), and external API key-protected access via Traefik.

## Technology Stack

- **Language**: Python 3.12
- **API Framework**: FastAPI, Uvicorn
- **Database**: PostgreSQL (via psycopg2)
- **Data Source**: Yahoo Finance (yfinance)
- **Data Processing**: pandas, pytz
- **Orchestration**: Kubernetes (CronJob, Deployment, Service, Ingress, IngressRoute)
- **Networking**: Traefik ingress controller, cert-manager
- **Image Registry**: cloudpublic/finance/{seek,tempo}:latest

## Components

### Seek: Query Service

**Purpose**: Expose financial price data via HTTP endpoints.

**Endpoints**:
- `GET /ok` — Health check; returns `{"status": "ok"}`
- `GET /time?symbol=<SYMBOL>&time=<ISO-DATETIME>&simple=<bool>` — Query price data for a specific date (ISO format normalized to UTC midnight). Returns full OHLCV record by default; `simple=true` returns plain-text adjusted close price.
- `GET /latest?symbol=<SYMBOL>&simple=<bool>` — Fetch the most recent price record for a symbol.

**Symbols Supported**: GOOG, SPY, VGT, BTC-USD, ETH-USD, ADA-USD (defined in Tempo; Seek queries whatever is in the database).

**Database Schema**: Queries the `financial_prices` table with columns: `time` (UTC midnight), `symbol`, `open`, `high`, `low`, `close`, `adj_close`, `volume`. Primary key: `(symbol, time)`.

**Deployment** (k8s.app.yaml):
- **Deployment**: 1 replica, image `cloudpublic/finance/seek:latest`
- **Service**: ClusterIP on port 80 → container port 8000
- **Health Check**: HTTP liveness probe on `/ok` every 10 seconds
- **Dual Ingress Routes**:
  - **Internal** (Ingress): `finance-seek.home4p.__{{infra.domains:f:x}}__` with TLS via DNS-ACME challenge (cert-manager)
  - **External** (IngressRoute): `finance-seek.__{{infra.domains:f:x}}__` with Query parameter API key matcher (`key=__{{koishi.finance.seek:f:api-key}}__`). Only requests with correct key pass.
- **Secrets**: Database credentials injected via environment variables (host, port, name, user, password from template variables).

**Build**: `bash seek/build.sh` invokes `my-k8s-build-image cloudpublic/finance/seek:latest finance seek`, building the image in-cluster (Kaniko).

**Deploy**: `bash seek/deploy.sh` applies `seek/k8s.app.yaml` via `my-k8s-deploy`.

### Tempo: Scheduled Data Fetcher

**Purpose**: Periodically fetch and store financial data from Yahoo Finance.

**Schedule**: Daily at 6:00 AM UTC (CronJob: `0 6 * * *`).

**Symbols Fetched**: GOOG, SPY, VGT, BTC-USD, ETH-USD, ADA-USD (hardcoded in `SYMBOLS` dict).

**Behavior**:
1. **Initialization**: Creates `financial_prices` table idempotently (`IF NOT EXISTS`).
2. **Per-Symbol Logic**:
   - Queries database for the latest stored timestamp for the symbol.
   - If no data exists, fetches maximum available history from yfinance (`period="max"`).
   - If data exists, fetches updates from the day after the latest record to today (`yf.download(..., start=<next_day>)`).
   - Detects gaps: warns if the database lags more than 1 day.
3. **Rate Limiting**: Sleeps 1 second between symbol fetches to respect yfinance limits.
4. **Data Normalization**: Converts all timestamps to UTC and normalizes to midnight (daily data granularity).
5. **Upsert Strategy**: Inserts/updates via `ON CONFLICT (symbol, time) DO UPDATE`, allowing re-runs without duplication.

**CronJob Config** (k8s.app.yaml):
- **Image**: `cloudpublic/finance/tempo:latest`
- **Schedule**: `0 6 * * *` (6 AM UTC)
- **Concurrency**: `Forbid` (no overlapping runs)
- **History**: Keeps last 3 successful and 5 failed job records for debugging
- **Restart Policy**: Never (fails are logged; CronJob will retry on next schedule)
- **Database Access**: Via environment variables (same credentials as Seek)

**Build**: `bash tempo/build.sh` invokes `my-k8s-build-image cloudpublic/finance/tempo:latest`, building in-cluster.

**Deploy**: `bash tempo/deploy.sh` applies `tempo/k8s.app.yaml` via `my-k8s-deploy`.

**Local Testing**: `bash tempo/code/run.sh` loads `.env` and runs the fetcher locally (requires database access and valid credentials).

## Deployment

Run from the `finance/` directory:

```bash
bash deploy.sh
```

This executes:
1. `my-k8s-deploy --file=k8s.namespace.yaml` — Creates namespace
2. `bash tempo/deploy.sh` — Deploys CronJob
3. `bash seek/deploy.sh` — Deploys Deployment, Service, and Ingress rules

The top-level `deploy.sh` assumes `my-k8s-deploy` and `my-k8s-build-image` are available (custom deployment abstractions provided by the Koishi cluster).

## Database

**Host**: `postgres.database` (cross-namespace DNS, resolves to PostgreSQL in the `database` namespace).

**Database**: `finance`

**User**: `finance`

**Password**: Injected via template variable `__{{database.postgres-local.accounts:f:finance}}__` (resolved by the deployment system).

**Table**: `financial_prices`
- Columns: `time` (TIMESTAMPTZ), `symbol` (TEXT), `open`, `high`, `low`, `close`, `adj_close` (DOUBLE PRECISION), `volume` (BIGINT)
- Primary Key: `(symbol, time)` — ensures one record per symbol per day
- Indexes: Primary key provides implicit index on symbol+time for fast lookups by date range

## Access Control

**Internal Access** (finance-seek.home4p.*):
- TLS certificate auto-provisioned by cert-manager via DNS challenge (Cloudflare).
- Accessible from within the home network or VPN.

**External Access** (finance-seek.*):
- Requires Traefik Query parameter matcher: `key=<api-key>`.
- The API key is stored as a template variable `__{{koishi.finance.seek:f:api-key}}__` and is NOT committed to code (managed by Vaultwarden).
- Useful for exposing specific endpoints to external integrations while preventing unauthorized access.

## Architecture Notes

- **Stateless Seek**: The query service is stateless and can be scaled horizontally (replicas > 1) without coordination. All state resides in PostgreSQL.
- **Single-Shot Tempo**: The CronJob runs once per day at 6 AM UTC. `concurrencyPolicy: Forbid` ensures no overlapping executions, preventing duplicate writes or race conditions.
- **Daily Granularity**: All price data is indexed by `(symbol, date_at_midnight_UTC)`. Queries normalize timestamps to this granularity; intra-day prices are not tracked.
- **Resilience**: Tempo failures (e.g., network outage, yfinance API unavailable) are logged in job history. The next scheduled run (next day) will detect and fill gaps automatically.
- **Lightweight Images**: Both Dockerfile use `python:3.12-slim` to minimize footprint and deployment time.

## Examples

**Query latest price for Bitcoin**:
```bash
curl -X GET "http://finance-seek.home4p.local/latest?symbol=BTC-USD"
```

**Query price for a specific date**:
```bash
curl -X GET "http://finance-seek.home4p.local/time?symbol=GOOG&time=2024-03-06T00:00:00Z"
```

**Get plain-text adjusted close price**:
```bash
curl -X GET "http://finance-seek.home4p.local/latest?symbol=SPY&simple=true"
```

**External access with API key**:
```bash
curl -X GET "https://finance-seek.domain.com/latest?symbol=VGT&key=<your-api-key>"
```

## Related Documentation

- [Koishi Cluster README](../README.md) — Overview of the home-lab infrastructure, deployment patterns, and security model.
- [Database Services](../database/README.md) — PostgreSQL configuration and cluster-wide database management.
