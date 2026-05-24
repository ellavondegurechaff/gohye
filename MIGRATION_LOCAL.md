# Local MongoDB to PostgreSQL Migration

This repo includes an automated local migration wrapper:

```powershell
.\scripts\sync-live-mongo-to-local-postgres.ps1
```

The script:

1. Loads secrets and local DB settings from `.env.migration`.
2. Runs `mongodump` against the MongoDB URI.
3. Stores a timestamped dump under `mongo-backups/`.
4. Runs the existing Go migrator against the dumped BSON files.
5. Writes logs under `migration-reports/`.

If `mongodump` is not installed, the script falls back to direct MongoDB mode and streams data through the Go migrator without creating a local dump.

## Setup

Copy the example env file:

```powershell
Copy-Item .env.migration.example .env.migration
```

Fill in:

```dotenv
MONGO_URI=...
MONGO_DB=hyejoo2
PG_HOST=localhost
PG_PORT=5432
PG_USER=root
PG_PASSWORD=root
PG_DATABASE=postgres
```

Do not commit `.env.migration`.

## Requirements

- Go installed and available as `go`.
- MongoDB Database Tools installed and `mongodump` available in PATH.
- Local PostgreSQL running and reachable with the `PG_*` settings.

## Useful Commands

Download only, without importing:

```powershell
.\scripts\sync-live-mongo-to-local-postgres.ps1 -DumpOnly
```

Run direct MongoDB to PostgreSQL migration without creating a dump:

```powershell
.\scripts\sync-live-mongo-to-local-postgres.ps1 -DirectMongo
```

Reset local app tables before import:

```powershell
.\scripts\sync-live-mongo-to-local-postgres.ps1 -ResetBefore
```

Use COPY mode for faster large imports:

```powershell
.\scripts\sync-live-mongo-to-local-postgres.ps1 -UseCopy
```

## Safety

The migration reads from live MongoDB and writes to the configured PostgreSQL target. Keep `PG_HOST=localhost` unless you intentionally want to write somewhere else.

If a MongoDB URI was shared outside your machine, rotate it or replace it with a temporary read-only migration user.
