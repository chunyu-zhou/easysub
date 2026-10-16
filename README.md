# EasySub

EasySub is a self-hosted subscription, renewal, and account-retention manager.

## Storage

EasySub uses an **embedded SQLite database by default**. No MySQL server, database account, browser setup wizard, connection check, or manual initialization is required.

On every startup, the application automatically creates the data directory when needed, opens the SQLite database, applies migrations, and seeds the initial administrator account and built-in defaults. On first start, it creates the database at:

```text
DATA_DIR/easysub.db
```

`DATA_DIR` defaults to `data`. To use a different SQLite filename or an absolute database path, set `SQLITE_PATH`. This is an optional deployment setting, not an installation step: after the container starts, open the application directly and sign in.

> Persist or mount the entire `DATA_DIR` in production. It contains the database, uploaded icons, encryption material, database configuration compatibility metadata, and local backups.

## Backup

Use the built-in JSON export/import functionality for portable backups. EasySub also supports local scheduled backups and WebDAV backups/restores. Keep an offline backup before moving or replacing the data directory.

## Docker deployment

GitHub Actions publishes a multi-architecture Docker image after every successful push to `main`:

```bash
docker pull ghcr.io/chunyu-zhou/easysub:latest
```

Start the included Compose stack:

```bash
docker compose up -d
```

The application is available directly at `http://your-server-ip:8000`. The `easysub_data` volume is persistent and holds the embedded SQLite database. Do not remove it unless you intentionally want to erase application data. Every `main` push also publishes `latest`, `main`, and a `sha-<commit>` image tag; version tags such as `v2.0.0` publish matching version tags.

### First-run account and security configuration

On an uninitialized database, EasySub creates the administrator account from environment variables. **Set these values before the first start.** Changing `ADMIN_USERNAME`, `ADMIN_PASSWORD`, or `ADMIN_EMAIL` later does not modify an existing account.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADMIN_USERNAME` | `admin` | First administrator username |
| `ADMIN_PASSWORD` | `admin123` | First administrator password — change this in production |
| `ADMIN_EMAIL` | `admin@example.com` | First administrator email address |
| `JWT_SECRET` | generated and persisted | Secret used to sign access/refresh JWTs and encrypt stored secrets such as WebDAV credentials |
| `DATA_DIR` | `data` | Persistent directory containing SQLite, uploaded icons, backups, and the generated JWT secret |
| `SQLITE_PATH` | `easysub.db` | SQLite filename under `DATA_DIR`, or an absolute database path |
| `LISTEN` | `:8000` | HTTP listener address |
| `MCP_ENABLED` | `true` | Enables the `/mcp` endpoint |

If `JWT_SECRET` is omitted, EasySub generates a cryptographically random secret and stores it as `DATA_DIR/jwt_secret`. This is safe as long as the whole `DATA_DIR` is persisted. Set an explicit, long random `JWT_SECRET` when managing secrets outside the container. **Do not change it after users have started using the application**: existing login tokens and encrypted WebDAV/notification secrets will no longer be readable.

The included [`compose.yaml`](compose.yaml) documents the generation commands immediately above `JWT_SECRET` and exposes these variables directly. Before running it, replace at least these placeholder values:

```yaml
ADMIN_USERNAME: easysub-admin
ADMIN_PASSWORD: replace-with-a-long-unique-password
ADMIN_EMAIL: admin@example.com
JWT_SECRET: replace-with-a-long-random-secret
```

#### Generate `JWT_SECRET`

`JWT_SECRET` must be a high-entropy, private, stable value. Generate it once, copy the complete output into `JWT_SECRET` in [`compose.yaml`](compose.yaml), and keep it in a password manager or secret-management system.

On Linux, macOS, or inside a Docker-capable shell:

```bash
openssl rand -hex 32
```

On Windows PowerShell:

```powershell
[Convert]::ToHexString((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

Both commands generate 32 random bytes encoded as 64 hexadecimal characters. Do not use an ordinary password, an example value from documentation, or a value committed to a public repository.

Do not rotate `JWT_SECRET` during normal upgrades. Changing it invalidates every active login token and makes existing encrypted WebDAV and notification credentials unreadable. If `JWT_SECRET` is intentionally omitted, EasySub instead generates and persists one at `DATA_DIR/jwt_secret`; this only remains safe when the complete data directory or Docker volume is retained.

The Compose file persists `/app/data` in the `easysub_data` Docker volume, so the SQLite database and generated secret survive container recreation. The container runs as a non-root `easysub` user; if you replace the named volume with a host bind mount, ensure that the mounted directory is writable by the container user. After signing in for the first time, change the administrator password under **Settings → Account**. For a public deployment, place the service behind a TLS-terminating reverse proxy or tunnel; the bundled Compose file intentionally exposes plain HTTP on port `8000` only.

### SQLite startup troubleshooting

EasySub does not have a database installation wizard. A `503` response that says **embedded SQLite is unavailable** means the application could not create, open, or migrate its local database. Inspect the container logs:

```bash
docker compose logs --tail=100 app
```

For the bundled named volume, recreate the application container without deleting the volume:

```bash
docker compose pull && docker compose up -d --force-recreate
```

Do **not** run `docker compose down -v` unless intentionally deleting every subscription, backup, uploaded icon, and generated secret. If a host directory is mounted instead of `easysub_data`, correct its write permissions, then recreate the container.

## Application API and MCP

The Go service embeds the production Vue build and exposes the REST API under `/api`. Create an API token in **Settings → Security** and pass it with the following header:

```text
Authorization: Bearer <API_TOKEN>
```

The Streamable HTTP MCP endpoint is `/mcp`. It supports MCP initialization, tool discovery, and read-only tools for subscriptions, dashboard data, and currencies. The same bearer API token authenticates MCP requests.

## Upstream attribution

This project is based on EasySub by [suyijun8182/easysub](https://github.com/suyijun8182/easysub). The upstream MIT license is retained in [LICENSE](LICENSE).
