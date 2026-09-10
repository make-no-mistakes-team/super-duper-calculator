# Public Deployment

> Scope: Highly desired optional public access. Independent of multiplayer.

## Deployment shape

Deploy one Go application with built web assets and PostgreSQL. Serve the
browser over HTTPS at a stable URL suitable for a QR code.

PostgreSQL may run in a container or as a managed service. Keep database access
on a private network. If rooms are enabled, the application host and proxy must
support long-lived SSE responses without buffering updates.

## Local development

Run PostgreSQL in Docker Compose and run the Go service and Vite on the host.
The development setup must provide:

- a pinned PostgreSQL image version, with the same supported major version in
  development, CI, and deployment;
- a named volume mounted at the data location required by the selected image;
- a `pg_isready` healthcheck and a startup command that waits for readiness;
- a `DATABASE_URL` for the Go service, with credentials in local environment
  configuration;
- versioned SQL migrations for creating and updating the schema;
- a database port bound to loopback when accessed from the host.

Document the Docker and Compose prerequisites, database startup, migration,
shutdown, and reset commands. Ordinary shutdown or container recreation must
retain data. Document destructive reset separately and identify what it deletes.

## Public paths

The ordinary URL opens the personal calculator. A room URL opens its invitation.

Direct navigation and refresh on an enabled room path must work. Missing API
operations must return API errors, including when SPA routing is enabled.

A public personal calculator is a complete useful deployment even if rooms are
not included. Room acceptance criteria apply only when that feature is enabled.

## Reproducibility and durability

The implementation must supply reproducible development, build, and deployment
instructions with its chosen commands and environment settings.

It must be possible to:

- start from a clean checkout;
- identify the running application version;
- update or restart the service;
- preserve the database across an ordinary restart or application replacement;
- distinguish an unavailable database from an empty new history.

Keep PostgreSQL data on persistent storage. Database connection or migration
failures must be reported without deleting history. Document backup and restore
procedures for the public database.

## Security baseline

- HTTPS for public browser traffic and Secure session cookies.
- Same-origin protection for browser mutations.
- No credentials or server secrets in client assets.
- Parameterized database access and text-safe rendering.
- A restricted mathematical language, never arbitrary code evaluation.
- Bounds on request bodies, engine work, feed retention, and connection counts.
- Separate limits for calculation submissions and reaction changes.
- Safe external errors, without stack traces, SQL, or internal addresses.

Limits must allow ordinary interactive use. Per-identity limits are useful,
but anonymous identities can be recreated; also bound aggregate server work.
IP-level limits must account for an entire classroom sharing one network.
Only trust forwarded client addresses from a configured trusted proxy.

## Audience readiness

Verify the hosted version from an external device. The initial rehearsal target
is 50 active room participants and a burst of 50 calculation submissions over
ten seconds. Adjust the documented target to the expected audience and measure
the selected deployment under that workload.

Under that workload:

- successful accepted calculations remain correctly persisted;
- overload returns controlled errors while preserving service availability and data;
- the UI remains usable;
- room updates and effects remain bounded;
- one source of repeated activity does not create an endless theatrical queue.

The calculator has priority over room presentation when resources are limited.

## Operational visibility

Provide a simple way to determine whether the service and its database are
available, which version is running, and whether failures are widespread.
Useful logs include request/action identifiers, outcome categories, and timing.

Exclude session secrets, database credentials, and full user expressions from
default logs.

Provide documented controls to disable room publication and large effects
independently of the calculator.

## Local or recorded fallback

The course permits local operation and video. Prepare a checked fallback that
shows real server-side calculation, errors, persistent history, and reuse.

The local fallback uses PostgreSQL through Docker Compose and a separate local
database. Before an offline demonstration, download the required images and
dependencies, apply migrations, and make client assets available locally.
Verify the core path with the public host and external network unavailable.

Public history is not synchronized to the fallback database. The fallback may
omit the room demonstration when audience networking is unavailable.

## Acceptance

For public hosting, open the URL on a phone, calculate, and reload history.
Verify that application and PostgreSQL restarts preserve committed records.
For container deployments, also recreate the database container with its
existing volume and confirm that history remains available.

If rooms are included: verify room deep links, multiple devices, reconnect,
the selected classroom workload, and the ability to disable shared features.

In either case, exercise the chosen fallback before treating the deployment as
demo-ready.
