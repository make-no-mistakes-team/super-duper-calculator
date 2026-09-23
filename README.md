# Super Duper Calculator

Go service, React client, and file-backed storage for the Unnecessarily Advanced
Calculator.

## Tools

Use Linux, macOS, or WSL2 with:

- Go **1.27.1**;
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

Open **http://127.0.0.1:5173**. The Go server listens on
**http://127.0.0.1:8080**. `GET /health/ready` queries the database and returns
`{"status":"ok"}` when it is accessible. `GET /health/live` reports process
liveness. `GET /health/version` identifies the running build without querying
SQLite. Vite proxies `/health` and `/api` to Go.

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
- Set `PUBLIC_ORIGIN` to the browser-facing HTTPS origin when using a reverse
  proxy, without a path or trailing slash. This enables Secure session cookies
  and validates mutation origins. Leave it unset for local HTTP development.
- `DATABASE_PATH` defaults to `data/calculator.sqlite`. Relative paths resolve
  from the Go process's working directory; `make` starts it at the repository
  root. Use an absolute path on permanent local storage when deploying.
- Go creates missing database directories and the file with private permissions.
  The default data directory and `.sqlite` files, including their sidecars, are
  ignored by Git. Keep custom database locations out of version control too.
- Reopening the same file preserves committed data. Connection and durability
  settings are defined in [Application Service](specs/application-service.md#database-configuration).

On POSIX systems, the existing database directory, file, and SQLite sidecars must
belong to the application's user and have no group or other permissions.
Their paths cannot be symlinks. If startup rejects a path, stop the application
and correct its ownership or permissions; the service will not change them.
Startup errors identify the failed stage without exposing SQL or the path.

To start with an empty database, stop the app, choose an unused `DATABASE_PATH`,
and restart. The previous database is retained. Deleting an old database is a
separate destructive operation: stop every process using it before removing the
selected file and any matching `-wal`, `-shm`, or `-journal` sidecars. Never remove
those files while the database is open.

Follow [Backup and restore](specs/application-service.md#backup-and-restore)
when copying or restoring data. Copying a live database file by itself can omit
committed data still in its WAL.

### API sessions and limits

API clients call `GET /api/session` before calculating or reading history.
Missing or expired identity returns `401 SESSION_REQUIRED`; only the session
operation creates a replacement. The browser client does this on load.

Honor `Retry-After` on `429 RATE_LIMITED`. Saturation or a deadline returns 503;
retry an uncertain calculation with its original `requestId`. Health endpoints
remain outside the API budget. The
[runtime request policy](specs/application-service.md#runtime-request-policy)
defines rate, concurrency, connection, payload, timeout, and response bounds.

## Build and check

```sh
make build       # Separate bin/calculator and web/dist outputs
make build-go    # Only the Go binary
make build-web   # Only the web build
make check       # Go build, vet, tests; web typecheck and build
```

CI uses the same setup and checks with CGO disabled. Storage checks use isolated
database files.

After `make build`, run `./bin/calculator` from the repository root to serve
`web/dist` and the API together. The binary uses `HTTP_ADDR` and does not load
`.env`; export any required configuration before starting it.

## Repository map

- `cmd/calculator/`: HTTP server and health checks.
- `internal/storage/`: database opening and connection configuration.
- `contracts/types.go` and `web/src/contracts.ts`: shared data shapes.
- `internal/calculation/engine.go`: pure evaluator interface.
- `web/`: React/TypeScript client and Vite configuration.
- `scripts/` and `Makefile`: local lifecycle and checks.
- [`examples/`](examples/): static [success](examples/calculation-success.json),
  [error](examples/calculation-error.json), and [history](examples/history-page.json)
  payloads.
- [`PLAN.md`](PLAN.md), [`SPEC_INDEX.md`](SPEC_INDEX.md), and [`specs/`](specs/):
  product requirements for feature implementation.
