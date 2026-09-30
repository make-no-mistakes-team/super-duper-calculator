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
- `WEB_ASSETS_DIR` selects a built client directory containing `index.html`.
  When unset, Go uses `web` beside its executable (`bin/web` after `make build`).
  A missing or invalid client stops startup; keep the database outside this
  directory.
- `STATISTICS_ENABLED` defaults to `true`. Set it to `false` to disable
  `GET /api/statistics`; calculations and history remain available. See the
  [statistics response](specs/application-service.md#optional-statistics-response).
- `ACHIEVEMENTS_ENABLED` defaults to `true`. Set it to `false` to disable
  personal discoveries and reactions without disabling calculation or history.
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
make build       # bin/calculator and copied bin/web assets
make build-go    # Only the Go binary
make build-web   # Build web/dist and replace bin/web with fresh assets
make check       # Go build, vet, tests; web typecheck and build
make check-browser # Built client + Go + SQLite in Chromium
```

The strict TypeScript check covers the client, browser scenarios, fixtures, and
Playwright configuration. `make check` needs no browser installation. To
provision the browser check after `make setup`, run the standard Playwright
installer:

```sh
cd web
npx --no-install playwright install --with-deps chromium
cd ..
make check-browser
```

The installer downloads the pinned Chromium build and installs its system
dependencies; on Linux the system-dependency step may need sudo. Browser
installation is explicit, not part of `make setup` or `make check`.

`make check-browser` rebuilds the Go executable with CGO disabled and stages
the built client using the normal build tasks. Playwright starts a real Go
server for each test on an available loopback port, with a fresh temporary
SQLite file and isolated browser identities. It stops the server and removes
the temporary database on success or failure; it never opens the configured
development database.

The suite covers the scientific `17` flow, history restore/reuse/reload,
private histories, error correction, canonical clipboard values, angle
context and stale replies, failed/held collection refreshes, replacement
identities, keyboard/IME handling, scene dismissal, and mobile theme switching.
HTTP interceptions only delay real replies or simulate network failure; they
do not supply calculation or history data. Tests run serially with explicit
response and UI waits.

A failing check returns a nonzero exit status. The HTML report is in
`web/playwright-report/`; failure screenshots, traces, and server logs are in
`web/test-results/`. Both directories are ignored by Git. To inspect a report:

```sh
cd web
npx --no-install playwright show-report
```

CI runs both checks after locked dependency setup and Chromium provisioning,
with CGO disabled. Storage and browser checks use isolated database files.

`make dev` also stages `bin/web`; Vite still reloads browser changes. The Go
binary serves the staged client and API from any working directory. For a
one-process launch after `make build`:

```sh
APP="$PWD/bin"
(cd /tmp && HTTP_ADDR=127.0.0.1:8088 DATABASE_PATH="$APP/../data/calculator.sqlite" "$APP/calculator")
```

The binary does not load `.env`; export any required configuration. To serve
assets from another directory, set `WEB_ASSETS_DIR` to that directory. Running
the built application needs neither Node.js nor the source checkout: keep
`calculator` and the adjacent `web` directory together.

## Releases

Pushing a version tag matching `v*` triggers
[the release workflow](.github/workflows/release.yml). It runs `make setup`,
provisions Chromium, and runs `make check`, `make check-browser`, and
`make build` on the tagged source with CGO disabled, then
publishes `calculator-linux-amd64.tar.gz` to
[GitHub Releases](https://github.com/make-no-mistakes-team/super-duper-calculator/releases).
Publication requires successful checks, build, and packaging. Branch pushes
and pull requests do not publish releases.

The archive contains only the Linux/amd64 executable and its adjacent `web`
assets. It contains no `.env`, credentials, database, personal history, or
Node.js helper. Download the runnable archive rather than GitHub's automatic
source archives, then extract it into an empty directory:

```sh
mkdir calculator-release
tar -xzf calculator-linux-amd64.tar.gz -C calculator-release
cd calculator-release
HTTP_ADDR=127.0.0.1:8088 DATABASE_PATH="$PWD/state/calculator.sqlite" ./calculator
```

Open <http://127.0.0.1:8088>. The application creates its own private database.
Restart with the same database path and browser identity to retain personal
history. Node.js, Go, and development dependencies are not needed at runtime.

For a local archive on another OS or CPU architecture, build on the target
device and package the same two outputs:

```sh
make setup
make check
make check-browser # Provision Chromium first, as described above
make build
tar -czf calculator-local.tar.gz -C bin calculator web
```

Keep the executable and `web` directory together when copying or extracting
them. Rehearse the exact artifact on the presentation device without external
network access. See [Demo & Operations](specs/demo-and-operations.md) for the
browser sequence, restart/history checks, and recovery procedure, and the
[release policy](PLAN.md#release-policy) for publication rules.

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
