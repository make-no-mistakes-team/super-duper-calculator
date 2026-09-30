# Super Duper Calculator

A calculator built to catch you off guard: memes, achievements, escalating
jokes, and the occasional theatrical meltdown. The surprises never change
the answer.

Go service, React/TypeScript client, and SQLite storage.

## Start locally

Use Linux, macOS, or WSL2 with Go **1.27.1**, Node.js **24.21.0**,
npm **11.19.0**, and GNU Make. SQLite is embedded; no database server is needed.

```sh
git clone https://github.com/make-no-mistakes-team/super-duper-calculator.git
cd super-duper-calculator
# With nvm: nvm install && nvm use
make setup
make dev
```

Open **http://127.0.0.1:5173**; the Go API runs on port **8080**.
Ctrl+C stops both servers. Restart `make dev` after Go edits; Vite reloads
client changes.

`make setup` installs locked dependencies and creates `.env` if missing.
The default database is `data/calculator.sqlite`; it survives restarts.
Keep configuration and data out of Git. See
[local configuration](specs/application-service.md#local-configuration) and
[backup and restore](specs/application-service.md#backup-and-restore).

To enable the multiplayer backend locally, run `ROOMS_ENABLED=true make dev`.
The default room code is `demo`. Room UI integration is separate; see the
[room API guide](docs/multiplayer-api.md) for a working curl walkthrough,
stream events, and the frontend contract.

## Build and check

```sh
make build         # bin/calculator and adjacent bin/web assets
make check         # Go build/vet/tests and TypeScript/web build
make check-browser # Real client, Go service, and SQLite in Chromium
```

Browser provisioning, scenarios, and failure reports are described in
[Quality & Testing](specs/quality-and-testing.md#real-client-verification).
`make check` does not require a browser installation.

## Releases

Version tags matching `v*` publish checked Linux/amd64 archives through
[GitHub Releases](https://github.com/make-no-mistakes-team/super-duper-calculator/releases).
The executable and `web/` assets run without Go or Node.js. See the
[release policy](PLAN.md#release-policy) and
[archive launch instructions](specs/demo-and-operations.md#offline-release-artifacts).

## Documentation

- [PLAN.md](PLAN.md): product scope and priorities.
- [SPEC_INDEX.md](SPEC_INDEX.md): subsystem contracts and requirements.
- [Frontend integration](web/INTEGRATION.md): module ownership and shared interfaces.
