# EasySub

EasySub is a self-hosted subscription, renewal, and account-retention manager.

## Storage

EasySub uses an **embedded SQLite database**. No MySQL server, database account, or external database setup is required.

On first start, the application creates and migrates the database at:

```text
DATA_DIR/easysub.db
```

`DATA_DIR` defaults to `data`. To use a different SQLite filename or an absolute database path, set `SQLITE_PATH`.

> Persist or mount the entire `DATA_DIR` in production. It contains the database, uploaded icons, encryption material, database configuration compatibility metadata, and local backups.

## Backup

Use the built-in JSON export/import functionality for portable backups. EasySub also supports local scheduled backups and WebDAV backups/restores. Keep an offline backup before moving or replacing the data directory.

## Docker deployment

GitHub Actions publishes a multi-architecture Docker image after every successful push to `main`:

```bash
docker pull ghcr.io/chunyu-zhou/easysub:latest
```

For server deployment, update `your-domain.com` in [Caddyfile](Caddyfile), then start the included Compose stack:

```bash
docker compose up -d
```

The `easysub_data` volume is persistent and holds the embedded SQLite database. Do not remove it unless you intentionally want to erase application data. Every `main` push also publishes `latest`, `main`, and a `sha-<commit>` image tag; version tags such as `v2.0.0` publish matching version tags.

## Application API and MCP

The Go service embeds the production Vue build and exposes the REST API under `/api`. Create an API token in **Settings → Security** and pass it with the following header:

```text
Authorization: Bearer <API_TOKEN>
```

The Streamable HTTP MCP endpoint is `/mcp`. It supports MCP initialization, tool discovery, and read-only tools for subscriptions, dashboard data, and currencies. The same bearer API token authenticates MCP requests.

## Upstream attribution

This project is based on EasySub by [suyijun8182/easysub](https://github.com/suyijun8182/easysub). The upstream MIT license is retained in [LICENSE](LICENSE).
