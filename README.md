# Distributed Cron Scheduler

[![CI](https://github.com/kumar-ayush101/distributed-cron-scheduler/actions/workflows/ci.yml/badge.svg)](https://github.com/kumar-ayush101/distributed-cron-scheduler/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-multi--stage-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

A horizontally scalable cron job scheduler that executes each job **exactly once** across a cluster of stateless nodes. Jobs are coordinated with distributed locks — Redis when available, with automatic fallback to **PostgreSQL advisory locks** so the exactly-once guarantee holds even on infrastructure without Redis.

> **Live demo:** _<!-- Replace with your Render URL after deploying --> https://your-app.onrender.com_

---

## Highlights

- **Exactly-once execution** — every due job is wrapped in a distributed lock, with a re-read guard that prevents double-runs when nodes race.
- **Pluggable lock backend** — a single `Locker` interface with two implementations: Redis (`SET NX PX` + Lua compare-and-delete) and Postgres (`pg_try_advisory_lock`). Falls back automatically.
- **Layered architecture** — clean separation of config, models, repository, service, scheduler and HTTP layers.
- **Observability built in** — Prometheus metrics (`/metrics`), structured JSON logging (`slog`), request IDs, liveness (`/health`) and readiness (`/ready`) probes.
- **Single-binary deployment** — the Go server serves the REST API *and* the React dashboard from one origin, so you get one public URL.
- **Production hygiene** — graceful shutdown, connection pooling, input validation, pagination, parameterized queries, non-root container, CI with race detector.

---

## Architecture

```
                         ┌──────────────────────────────┐
      Browser  ───────▶  │   HTTP layer (chi-free stdlib) │
                         │  middleware: CORS · reqID ·    │
                         │  logging · metrics · recover   │
                         └───────────────┬────────────────┘
                                         │
                 ┌───────────────────────┼───────────────────────┐
                 ▼                        ▼                       ▼
          Service layer           Scheduler engine          /metrics /health
        (validation, rules)   (poll → lock → run → log)      /ready probes
                 │                        │
                 ▼                        ▼
          Repository layer  ◀────────  Locker interface
          (SQL, pagination)            ├── RedisLocker  (multi-node)
                 │                      └── PostgresLocker (advisory locks)
                 ▼
            PostgreSQL  ◀── jobs · job_history
```

Every node runs the same binary. Set `SERVE_FRONTEND=false` to run a node as a
pure worker. Nodes coordinate purely through the shared lock backend, so you can
add or remove nodes freely.

### Project layout

```text
.
├── cmd/server/              # Entrypoint: wiring + graceful shutdown
├── internal/
│   ├── config/              # Env-driven config (DATABASE_URL / REDIS_URL)
│   ├── models/              # Domain types, validation, cron parsing
│   ├── database/            # Pool, migrations, seed
│   ├── repository/          # Data access (parameterized, paginated)
│   ├── service/             # Business logic
│   ├── lock/                # Locker interface + Redis & Postgres impls
│   ├── scheduler/           # Polling engine with exactly-once guard
│   ├── metrics/             # Prometheus collectors
│   └── httpapi/             # Router, handlers, middleware
├── client/                  # React (Vite) dashboard
├── Dockerfile               # 3-stage: frontend → Go → tiny runtime
├── docker-compose.yml       # Local multi-node cluster
└── render.yaml              # One-click Render blueprint
```

---

## Exactly-once guarantee

When a job becomes due, every node could pick it up. To ensure only one runs it:

1. **Discover** — each node queries for `status = 'active' AND next_run_at <= now()`.
2. **Lock** — the node tries to acquire `cron:job:{id}`. Only one succeeds; the rest skip and increment `lock_conflicts_total`.
3. **Re-read guard** — the winner reloads the job inside the lock. If a peer already executed and rescheduled it, the winner backs off. This closes the race between discovery and lock acquisition.
4. **Execute & record** — run the job, write a `job_history` row (status, duration, node id), update counters, compute the next run.
5. **Release** — the lock is released safely. Redis uses a Lua compare-and-delete so a node can never delete a lock it no longer owns; Postgres releases the advisory lock with the session.

If a node crashes mid-execution, the Redis lock TTL (or the Postgres session ending) releases the lock automatically.

---

## API reference

Base path: `/api`. All responses are JSON. List endpoints are paginated with `?limit=&offset=` and return `{ items, total, limit, offset }`.

| Method | Path | Description |
| ------ | ---- | ----------- |
| `GET` | `/api/jobs` | List jobs (paginated) |
| `POST` | `/api/jobs` | Create a job `{ name, cron_schedule }` |
| `GET` | `/api/jobs/{id}` | Get a single job |
| `PATCH` | `/api/jobs/{id}` | Update name / cron / status (`active`·`paused`·`disabled`) |
| `DELETE` | `/api/jobs/{id}` | Delete a job |
| `POST` | `/api/jobs/{id}/run` | Schedule an immediate run |
| `GET` | `/api/jobs/{id}/history` | Execution history for a job |
| `GET` | `/api/history` | Execution history across all jobs (`?job_id=` optional) |
| `GET` | `/health` | Liveness probe |
| `GET` | `/ready` | Readiness probe (checks DB + lock backend) |
| `GET` | `/metrics` | Prometheus metrics |

### Example

```bash
# Create a job that runs every 5 minutes
curl -X POST http://localhost:8080/api/jobs \
  -H 'Content-Type: application/json' \
  -d '{"name":"Nightly backup","cron_schedule":"*/5 * * * *"}'

# Pause it
curl -X PATCH http://localhost:8080/api/jobs/1 \
  -H 'Content-Type: application/json' \
  -d '{"status":"paused"}'

# Run it right now
curl -X POST http://localhost:8080/api/jobs/1/run
```

---

## Configuration

All configuration is environment-driven. Discrete `DB_*` / `REDIS_*` variables
work locally; single connection strings work on managed platforms.

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `PORT` / `HTTP_PORT` | `8080` | HTTP listen port |
| `APP_ENV` | `development` | `production` switches logs to JSON |
| `DATABASE_URL` | — | Full Postgres URL (overrides `DB_*`) |
| `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`/`DB_SSLMODE` | localhost defaults | Discrete DB settings |
| `REDIS_URL` | — | Redis URL. **Omit to use Postgres advisory locks** |
| `POLL_INTERVAL` | `5s` | How often nodes scan for due jobs |
| `LOCK_TTL` | `30s` | Lock expiry (Redis) |
| `SEED_DATA` | `true` | Insert demo jobs on an empty DB |
| `SERVE_FRONTEND` | `true` | Serve the dashboard (`false` = worker-only) |
| `FRONTEND_DIR` | `./web` | Static assets directory |
| `NODE_ID` | hostname | Identifier shown in logs & history |

---

## Getting started

### Option A — single binary (API + dashboard)

```bash
# 1. Build the dashboard into ./web
cd client && npm install && npm run build && cd ..

# 2. Run Postgres (any instance works)
docker run -d --name pg -e POSTGRES_USER=cron -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=cron -p 5432:5432 postgres:15-alpine

# 3. Start the server
export DATABASE_URL="postgres://cron:secret@localhost:5432/cron?sslmode=disable"
go run ./cmd/server
# open http://localhost:8080
```

### Option B — multi-node cluster via Docker Compose

```bash
docker compose up --build                 # 1 api node + 2 workers + db + redis
docker compose up --build --scale worker=4 # scale workers to demonstrate locking
```

The dashboard is at `http://localhost:8080`. Watch the logs: for each tick only
one node logs `job executed` while the others skip on a lock conflict.

### Frontend dev server

```bash
cd client
cp .env.example .env   # set VITE_API_URL=http://localhost:8080
npm run dev            # http://localhost:5173, proxies to the Go API
```

---

## Deploying to Render (live link)

This repo ships a [`render.yaml`](./render.yaml) blueprint.

1. Push the repo to GitHub.
2. In Render: **New +** → **Blueprint** → select the repo.
3. Render provisions a free PostgreSQL database, injects `DATABASE_URL`, builds the Docker image and deploys the web service.
4. Your dashboard and API go live at `https://<service>.onrender.com`.

No Redis is required — the scheduler uses Postgres advisory locks by default.
To scale horizontally with Redis, uncomment the Key Value blocks in
`render.yaml`.

---

## Testing & CI

```bash
go test -race ./...     # backend unit + handler tests
cd client && npm run lint && npm run build
```

CI (GitHub Actions) runs three jobs on every push/PR:

- **backend** — `go vet`, build, and `go test -race`
- **frontend** — eslint + production build
- **docker** — validates the multi-stage image builds

> The module pins `GOTOOLCHAIN=local`; build and test with Go 1.25.

---

## Tech stack

Go 1.25 · PostgreSQL 15 · Redis 7 (optional) · React 19 + Vite · Prometheus · Docker · GitHub Actions

## License

MIT — see [LICENSE](./LICENSE).
