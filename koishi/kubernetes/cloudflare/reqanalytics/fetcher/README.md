# Cloudflare Request Analytics (reqanalytics-fetcher)

Hourly CronJob that pulls per-host request totals from the Cloudflare GraphQL Analytics API for the Koishi domains and stores them as local JSON files.

## How it works

1. Reads the three Koishi domains from the `reqanalytics-fetcher-config` ConfigMap (`DOMAIN_X`, `DOMAIN_C`, `DOMAIN_D`).
2. Looks up each domain's Cloudflare zone ID.
3. Queries `httpRequestsAdaptiveGroups` per zone for the last `LOOKBACK_HOURS` completed hours (default 48), grouped by `datetimeHour` and `clientRequestHTTPHost`, filtered to `requestSource: "eyeball"`.
4. Writes one file per hour to `kubernetes/cloudflare/reqanalytics/data/hourly/YYYY-MM-DDTHH.json`:

```json
{
  "hour": "2026-11-08T10:00:00Z",
  "hosts": {
    "jellyfin.example.com": {"requests": 120, "visits": 80, "edgeResponseBytes": 3456}
  }
}
```

`requests` is the row count, `visits` is the page-view-like count, and `edgeResponseBytes` is bytes sent from the edge.

Hours with no traffic are written with an empty `hosts` object. Each run rewrites the whole lookback window, so re-runs are idempotent and late-arriving data is picked up.

If any domain fails, nothing is written for that run and the Job fails.

## Scope and limits

- Hosts are not filtered. Every hostname that received traffic in these zones is recorded, including wildcard-DNS hits for unknown subdomains.
- `httpRequestsAdaptiveGroups` keeps at least 31 days of data on every plan, including Free.
- Adaptive datasets are sampled at high volume, so counts are estimates.
- Totals are per host and hour.

## Deployment

Requires a Vaultwarden secret rendered into `cloudflare.analytics.token`. Create a dedicated API token with **Zone → Analytics → Read** and **Zone → Zone → Read** permissions for the Koishi zones. Do not reuse the DDNS token.

```bash
./deploy.sh
```

`deploy.sh` creates the `cloudflare` namespace, the ConfigMap from `code/fetcher.py`, and applies `k8s.app.yaml`.

## Files

- `k8s.app.yaml`: Namespace, config ConfigMap, CronJob (hostPath mounted to `data/` on node `nur`)
- `code/fetcher.py`: the fetcher (stdlib only)
- `deploy.sh`: deployment entry point
