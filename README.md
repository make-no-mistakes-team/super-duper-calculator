# Super Duper Calculator

Working scientific calculator with a React client, Go calculation API, and
personal SQLite history. Enter an expression, calculate, and select an old
record to restore its expression and degree/radian setting.

## Tools

Use Linux, macOS, or WSL2 with:

- Go **1.26.3** or newer;
- Node.js **24.21.0 LTS** and npm **11.19.0** (`.nvmrc` pins Node);
- GNU Make.

The frontend uses React 19.3.0, TypeScript 7.0.2, and Vite 8.3.0. The SQLite
engine is included in the pure-Go dependency pinned in `go.mod` and `go.sum`.

## Start locally

```sh
git clone https://github.com/make-no-mistakes-team/super-duper-calculator.git
cd super-duper-calculator
# If using nvm:
nvm install
nvm use
make setup
make dev
```

`make setup` creates `.env` from `.env.example` if it is missing, downloads Go
modules, and installs locked npm dependencies. It never replaces an existing
`.env`. `make env` creates only the environment file.

Open **http://127.0.0.1:5173**. Try `sqrt(81)+2^3`, then press Enter or
**Вычислить**. Successful results and expression errors appear in personal
history; selecting a record restores it without calculating again. History is
kept for the browser's anonymous cookie and survives server restarts.

The Go server listens on **http://127.0.0.1:8080**. Vite proxies `/api` to Go.
`GET /health/ready` checks the database; `GET /health/live` checks the process.

`make dev` builds and runs the Go binary alongside Vite. Ctrl+C stops both and
their child processes. Go changes require restarting `make dev`; Vite reloads web
changes. Go opens the database at startup and closes it during shutdown. A
storage setup failure stops startup and leaves existing data intact.

### Local configuration and database state

`make` tasks load `.env`; exported shell variables take precedence. Keep `.env`
local: it is ignored by Git and created with owner-only permissions. Compare it
with `.env.example` when configuration changes.

- `HTTP_ADDR` defaults to `127.0.0.1:8080`. Update `API_PROXY_TARGET` too if the
  server moves; its default is `http://127.0.0.1:8080`.
- `DATABASE_PATH` defaults to `data/calculator.sqlite`. Relative paths resolve
  from the Go process's working directory; `make` starts it at the repository
  root. Use an absolute path on permanent local storage when deploying.
- Go creates missing database directories and the file with private permissions.
  The default data directory and `.sqlite` files, including their sidecars, are
  ignored by Git. Keep custom database locations out of version control too.
- Reopening the same file preserves committed data. Connection and durability
  settings are defined in [Application Service](specs/application-service.md#database-configuration).
- The first startup applies the SQLite schema migration. Later startups reuse
  the same schema and records.

To start with an empty database, stop the app, choose an unused `DATABASE_PATH`,
and restart. The previous database is retained. Deleting an old database is a
separate destructive operation: stop every process using it before removing the
selected file and any matching `-wal`, `-shm`, or `-journal` sidecars. Never remove
those files while the database is open.

Follow [Backup and restore](specs/application-service.md#backup-and-restore)
when copying or restoring data. Copying a live database file by itself can omit
committed data still in its WAL.

## Build and check

```sh
make build       # Separate bin/calculator and web/dist outputs
make build-go    # Only the Go binary
make build-web   # Only the web build
make check       # Go build, vet, tests; web typecheck and build
```

CI uses the same setup and checks with CGO disabled. Storage checks use isolated
database files.

After `make build`, run `./bin/calculator` from the repository root to serve the
built web app and API together at **http://127.0.0.1:8080**.

## Repository map

- `cmd/calculator/`: HTTP server, calculation API, history, and health checks.
- `internal/storage/`: database opening, configuration, and schema migration.
- `contracts/types.go` and `web/src/contracts.ts`: shared data shapes.
- `internal/calculation/engine.go`: pure expression evaluator.
- `web/`: React/TypeScript client and Vite configuration.
- `scripts/` and `Makefile`: local lifecycle and checks.
- [`examples/`](examples/): static [success](examples/calculation-success.json),
  [error](examples/calculation-error.json), and [history](examples/history-page.json)
  payloads.
- [`PLAN.md`](PLAN.md), [`SPEC_INDEX.md`](SPEC_INDEX.md), and [`specs/`](specs/):
  product requirements for feature implementation.
