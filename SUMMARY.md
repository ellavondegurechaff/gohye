# GoHYE Project Summary

This repository contains **GoHYE**, a K-pop card-collection Discord bot with a Go admin API and a Next.js admin dashboard.

The project is currently a monorepo with three main runnable parts:

- **Discord bot**: root Go app started from `main.go`.
- **Admin backend**: Go Fiber API in `backend/`.
- **Admin frontend**: Next.js dashboard in `frontend/`.

The codebase appears to have started from `github.com/disgoorg/bot-template`; the Go module names still use that upstream name.

## Top-Level Structure

```text
gohye/
├── main.go
├── go.mod
├── go.sum
├── config.example.toml
├── Dockerfile
├── README.md
├── bottemplate/
├── backend/
├── frontend/
├── MDFILES/
├── skipped_cards.log
└── Trading System
```

## Main Applications

### Discord Bot

The Discord bot lives at the repository root and uses the shared code in `bottemplate/`.

Important areas:

- `main.go`: bot entry point.
- `bottemplate/bot.go`: main bot wiring and runtime dependencies.
- `bottemplate/commands/`: slash command implementations.
- `bottemplate/database/`: PostgreSQL access, schema initialization, models, and repositories.
- `bottemplate/economy/`: economy, auctions, claims, effects, crafting, pricing, and transactions.
- `bottemplate/services/`: Spaces/S3 image handling, quests, search, profile images, and leaderboard images.
- `bottemplate/migration/` and `bottemplate/cmd/migrate/`: migration tooling.

The bot includes command groups for admin tools, cards, economy, social comparison, and system/profile features.

### Backend API

The admin backend lives in `backend/` as a separate Go module:

```text
backend/
├── main.go
├── go.mod
├── handlers/
├── services/
├── middleware/
├── config/
├── models/
└── utils/
```

It uses **Fiber v2** and connects back to the root Go module through:

```go
replace github.com/disgoorg/bot-template => ../
```

The backend provides admin-facing APIs for:

- Discord OAuth authentication.
- Card management.
- Collection management.
- Sync operations.
- User/admin access checks.
- Dashboard data.
- Uploads and image-related workflows.

### Frontend Dashboard

The frontend lives in `frontend/` and is a **Next.js 15** app using React 19, TypeScript, Tailwind 4, Radix UI, Zustand, Zod, TanStack Table, and Framer Motion.

Key areas:

- `frontend/src/app/`: App Router pages and layouts.
- `frontend/src/app/(auth)/login`: login flow.
- `frontend/src/app/(dashboard)/dashboard`: admin dashboard.
- `frontend/src/app/api/`: API proxy routes to the Go backend.
- `frontend/middleware.ts`: dashboard route protection using the `gohye_session` cookie.
- `frontend/next.config.ts`: redirects, backend rewrites, and image domains.

Dashboard sections include cards, collections, import tools, and sync tools.

## Core Technologies

- **Go 1.23** with toolchain `go1.24.2`.
- **Discord** via `disgo`.
- **PostgreSQL** via `pgx` and **Bun ORM**.
- **DigitalOcean Spaces / S3-compatible storage** via AWS SDK v2.
- **Fiber v2** for the admin backend.
- **Next.js 15**, **React 19**, **TypeScript**, and **Tailwind 4** for the dashboard.
- **chromedp** for generated profile/leaderboard imagery.
- **MongoDB driver** for legacy migration paths.
- **fuzzy search** via `github.com/sahilm/fuzzy`.

## Configuration

The main configuration template is `config.example.toml`.

Expected local config:

```text
config.toml
```

Major config sections:

- `[log]`: logging level, format, and source options.
- `[bot]`: Discord bot token and development guilds.
- `[db]`: PostgreSQL host, port, credentials, database, pool size, and fast initialization.
- `[web]`: backend host, port, session key, OAuth, admin access, and rate limiting.
- `[spaces]`: DigitalOcean Spaces credentials, region, bucket, and card root path.

The frontend expects a backend URL such as:

```text
GO_BACKEND_URL=http://localhost:8080
```

usually in `frontend/.env.local`.

## Common Commands

Run the Discord bot:

```bash
go run .
```

Run the Discord bot with a config file:

```bash
go run . --config config.toml
```

Sync Discord commands:

```bash
go run . --sync-commands=true
```

Recalculate card prices:

```bash
go run . --calculate-prices=true
```

Run the backend:

```bash
cd backend
go run .
```

Run the frontend:

```bash
cd frontend
npm run dev
```

Build the frontend:

```bash
cd frontend
npm run build
```

Run frontend lint:

```bash
cd frontend
npm run lint
```

Run migration help:

```bash
go run ./bottemplate/cmd/migrate --help
```

## Discord Command Areas

The command registration is centralized under `bottemplate/commands/`.

Current command categories include:

- **Admin**: database test/init, delete card, gift, duplicate fixes, economy analysis, image management.
- **Cards**: summon, search, claim, forge, level up, limited cards, collection views.
- **Economy**: balance, daily, work, shop, liquefy, auction, trade, fuse, price stats, inbox.
- **Social**: wish, has, miss, diff.
- **System**: help, version, metrics, inventory, effects, profile, quests.

Many commands include Discord component handlers for buttons, selects, pagination, shop flows, claiming, forging, and similar interactions.

## Data And Assets

Important data and support files include:

- `bottemplate/cmd/migrate/cards.json`
- `bottemplate/cmd/migrate/collections.json`
- `MDFILES/*.md`
- `MDFILES/*.js`
- `MDFILES/*.py`
- `skipped_cards.log`

Runtime card images are expected to live in DigitalOcean Spaces and be served through the configured CDN/domain.

## Docker

The root `Dockerfile` builds the Discord bot binary only.

Current note:

- `go.mod` requires Go 1.23 with toolchain 1.24.2.
- The Dockerfile should be checked for Go version alignment before relying on it for deployment.

## Current Gaps And Notes

- `README.md` still appears to describe the upstream bot template rather than the full GoHYE bot/backend/frontend system.
- There are no obvious Go test files currently present.
- The Docker setup covers the bot, not the backend and frontend.
- Some planning files in `MDFILES/` may describe earlier ideas that differ from the current Next.js dashboard implementation.
- `bottemplate/handlers/handlers.go` has a TODO-style message handler stub.

## Quick Mental Model

GoHYE is best understood as:

1. A Discord card-collection bot backed by PostgreSQL.
2. A shared Go domain layer in `bottemplate/`.
3. A Fiber admin API that exposes management operations.
4. A Next.js dashboard that talks to the backend and manages cards, collections, imports, and sync workflows.

