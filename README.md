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

## Upstream attribution

This project is based on EasySub by [suyijun8182/easysub](https://github.com/suyijun8182/easysub). The upstream MIT license is retained in [LICENSE](LICENSE).
