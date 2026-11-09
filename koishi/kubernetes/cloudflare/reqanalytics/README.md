# Cloudflare Request Analytics (reqanalytics)

Cloudflare Request Analytics collection and visualization suite for Koishi services.

## Architecture

- **[fetcher](./fetcher/)**: Hourly Python CronJob that queries Cloudflare's GraphQL Analytics API for Koishi domains and persists hourly request metrics to `../data/hourly/YYYY-MM-DDTHH.json`.
- **[reviewer](./reviewer/)**: Go web service providing an interactive, user-friendly dashboard to inspect and visualize the collected analytics data.
- **`data/`**: Shared storage directory holding JSON snapshots of hourly request metrics.
