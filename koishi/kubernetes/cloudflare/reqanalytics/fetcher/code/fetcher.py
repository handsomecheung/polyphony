#!/usr/bin/env python3

import json
import os
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path

CF_API = "https://api.cloudflare.com/client/v4"
GRAPHQL_LIMIT = 10000
TIME_FORMAT = "%Y-%m-%dT%H:%M:%SZ"

CF_TOKEN = os.environ["CLOUDFLARE_ANALYTICS_TOKEN"]
DOMAINS = [
    os.environ[name]
    for name in ("DOMAIN_X", "DOMAIN_C", "DOMAIN_D")
    if os.environ.get(name)
]
OUTPUT_DIR = Path(os.environ.get("OUTPUT_DIR", "/data"))
LOOKBACK_HOURS = int(os.environ.get("LOOKBACK_HOURS", "48"))


def log(message):
    print(message, file=sys.stderr, flush=True)


def cf_get(url, payload=None):
    headers = {"Authorization": f"Bearer {CF_TOKEN}"}
    data = None
    if payload is not None:
        data = json.dumps(payload).encode()
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(url, data=data, headers=headers)
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def zone_id_for(domain):
    body = cf_get(f"{CF_API}/zones?name={urllib.parse.quote(domain)}")
    if not body.get("success") or not body["result"]:
        raise RuntimeError(f"zone lookup failed for {domain}: {body.get('errors')}")
    return body["result"][0]["id"]


def fetch_hourly(zone_id, since, until):
    query = """
    {
      viewer {
        zones(filter: {zoneTag: %s}) {
          httpRequestsAdaptiveGroups(
            limit: %d
            filter: {datetime_geq: %s, datetime_lt: %s, requestSource: "eyeball"}
          ) {
            count
            dimensions { datetimeHour clientRequestHTTPHost }
            sum { visits edgeResponseBytes }
          }
        }
      }
    }
    """ % (
        json.dumps(zone_id),
        GRAPHQL_LIMIT,
        json.dumps(since),
        json.dumps(until),
    )
    body = cf_get(f"{CF_API}/graphql", {"query": query})
    if body.get("errors"):
        raise RuntimeError(f"graphql failed: {body['errors']}")
    zones = body.get("data", {}).get("viewer", {}).get("zones") or []
    if not zones:
        raise RuntimeError(f"no zone data returned in graphql response: {body}")
    rows = zones[0].get("httpRequestsAdaptiveGroups") or []
    if len(rows) >= GRAPHQL_LIMIT:
        raise RuntimeError("graphql result reached the limit; reduce LOOKBACK_HOURS")
    return rows


def main():
    end = datetime.now(timezone.utc).replace(minute=0, second=0, microsecond=0)
    start = end - timedelta(hours=LOOKBACK_HOURS)

    totals = {}
    failed = []
    for domain in DOMAINS:
        try:
            rows = fetch_hourly(zone_id_for(domain), start.strftime(TIME_FORMAT), end.strftime(TIME_FORMAT))
        except Exception as error:
            log(f"domain {domain} failed: {type(error).__name__}: {error}")
            failed.append(domain)
            continue
        log(f"domain {domain}: {len(rows)} hourly rows")
        for row in rows:
            hour = datetime.strptime(row["dimensions"]["datetimeHour"], TIME_FORMAT).replace(tzinfo=timezone.utc)
            host = (row["dimensions"].get("clientRequestHTTPHost") or domain).lower()
            sum_data = row.get("sum") or {}
            totals.setdefault(hour.strftime(TIME_FORMAT), {})[host] = {
                "requests": row.get("count", 0),
                "visits": sum_data.get("visits", 0),
                "edgeResponseBytes": sum_data.get("edgeResponseBytes", 0),
            }

    if failed:
        raise SystemExit(f"nothing written; failed domains: {', '.join(failed)}")

    out_dir = OUTPUT_DIR / "hourly"
    out_dir.mkdir(parents=True, exist_ok=True)
    hour = start
    while hour < end:
        key = hour.strftime(TIME_FORMAT)
        record = {"hour": key, "hosts": totals.get(key, {})}
        path = out_dir / f"{hour.strftime('%Y-%m-%dT%H')}.json"
        tmp = out_dir / f"{path.name}.tmp"
        tmp.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
        tmp.replace(path)
        hour += timedelta(hours=1)
    log(f"wrote {LOOKBACK_HOURS} hourly files to {out_dir}")


if __name__ == "__main__":
    main()
