# Super Duper Calculator

A starting point for the Unnecessarily Advanced Calculator. The running starter
has a Russian React page and a Go process-liveness endpoint. PostgreSQL is
available for feature work; the Go server does not connect to it.

## Tools

Use Linux, macOS, or WSL2 with:

- Go **1.27.1**;
- Node.js **24.21.0 LTS** and npm **11.19.0** (`.nvmrc` pins Node);
- Docker with the Compose plugin (`docker compose` with `--wait` support);
- GNU Make.

On Linux, `docker info` must work for your user. After adding the user to the
Docker group, log in again or run `newgrp docker` in the terminal.

The frontend uses React 19.3.0, TypeScript 7.0.2, and Vite 8.3.0. Compose pins
PostgreSQL 18 Alpine by image digest.

## Start locally

With the tools installed and Docker running:

```sh
git clone https://github.com/make-no-mistakes-team/super-duper-calculator.git
cd super-duper-calculator
# If using nvm:
nvm install
nvm use
make setup
make db
make dev
```

`make setup` creates `.env` from `.env.example`, generates a local database
password, and installs the locked web dependencies with `npm ci`. It never
replaces an existing `.env`. `make env` creates only the environment file.

Open **http://127.0.0.1:5173**. The Go server listens on
**http://127.0.0.1:8080**; `GET /health/live` returns `{"status":"ok"}`. This
endpoint reports process liveness, not database readiness. Vite proxies `/health`
and `/api` to Go; business API routes are not implemented.

`make dev` builds and runs the Go binary alongside Vite. Ctrl+C stops both and
their child processes. Go changes require restarting `make dev`; Vite reloads web
changes. The database runs separately, so `make dev` also works without it.

### Local configuration and database state

`make` tasks load `.env`; exported shell variables take precedence. Keep `.env`
local: it is ignored by Git and created with owner-only permissions.

- `HTTP_ADDR` defaults to `127.0.0.1:8080`. Update `API_PROXY_TARGET` too if the
  server moves; its default is `http://127.0.0.1:8080`.
- PostgreSQL is exposed only on `127.0.0.1:54329`. Database name, user, and initial
  password come from `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`.
- `make db` waits for PostgreSQL readiness. `make down` stops and removes its
  container but preserves the named data volume. A later `make db` reuses it.
- `make db-reset` asks you to type `reset`, deletes the local data volume, and
  starts a fresh database. Changing `.env` credentials does not change an
  existing database; reset only when you intend to discard its data.

## Build and check

```sh
make build       # Separate bin/calculator and web/dist outputs
make build-go    # Only the Go binary
make build-web   # Only the web build
make check       # Go build, vet, tests; web typecheck and build
```

CI uses the same setup and checks, with PostgreSQL readiness checked through
Compose. The Go binary serves only the liveness endpoint; it does not serve the
web build.

## Repository map

- `cmd/calculator/`: standard-library HTTP starter.
- `contracts/types.go` and `web/src/contracts.ts`: shared data shapes.
- `internal/calculation/engine.go`: pure evaluator interface.
- `web/`: React/TypeScript client and Vite configuration.
- `scripts/`, `Makefile`, and `compose.yaml`: local lifecycle and checks.
- [`examples/`](examples/): static [success](examples/calculation-success.json),
  [error](examples/calculation-error.json), and [history](examples/history-page.json)
  payloads.
- [`PLAN.md`](PLAN.md), [`SPEC_INDEX.md`](SPEC_INDEX.md), and [`specs/`](specs/):
  product requirements for feature implementation.
