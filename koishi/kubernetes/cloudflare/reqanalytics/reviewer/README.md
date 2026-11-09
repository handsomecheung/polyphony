# Cloudflare Request Analytics Reviewer (reqanalytics-reviewer)

Go web application providing a clean, interactive dashboard to inspect and visualize yesterday's Cloudflare request metrics aggregated by `reqanalytics-fetcher`.

## Features

- **Yesterday's Overview Cards**: Prominently displays the 4 key metrics for the full calendar day:
  - **Total Requests**
  - **Total Bandwidth**
  - **Total Visits**
  - **Active Hosts**
- **Domain Selector**: Filtering by root domain dynamically updates the summary metric cards, the 24-hour timeline bar chart, and the hosts table in sync.
- **24-Hour Traffic Trend Chart**: Responsive SVG bar chart displaying hourly distribution (00:00 - 23:00 UTC) with interactive hover tooltips.
- **Host Analytics Table**: Sortable, searchable table displaying requests (% share), visits, and bandwidth per host.
- **Self-contained**: HTML, CSS, and JS are embedded into the Go binary (`embed.FS`) without external CDN runtime dependencies.

## Local Development

Run directly with Go:

```bash
cd code
go run . -data-dir=../../data -port=8080
```

Open `http://localhost:8080` in your browser.

## Deployment

1. Build the container image using Kaniko:
   ```bash
   ./build.sh
   ```

2. Deploy the Kubernetes manifests (Deployment, Service, Ingress):
   ```bash
   ./deploy.sh
   ```

The service mounts the shared data directory `/mnt/coder-workspaces/private-workspace/repos/local/polyphony/koishi/kubernetes/cloudflare/reqanalytics/data` read-only.
