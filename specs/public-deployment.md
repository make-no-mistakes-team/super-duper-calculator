# Public Deployment

> Scope: Highly desired optional public access. Independent of multiplayer.

## Deployment shape

Deploy the Go application with built web assets under the
[storage policy](application-service.md#database-configuration).
Serve the browser over HTTPS at a stable URL suitable for a QR code.

Keep the database directory outside release-specific build directories.
Configure a stable absolute `DATABASE_PATH` for hosting.

If rooms are enabled, the application host and proxy must support long-lived
SSE responses without buffering updates.

## Local development

Run the Go service and Vite on the host. Use the application's
[database configuration](application-service.md#database-configuration) and
[shutdown and reset procedures](application-service.md#file-lifecycle).

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

Document and rehearse the
[backup and restore procedure](application-service.md#backup-and-restore)
before the demonstration.

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

Operators must be able to check [health](application-service.md#health),
identify the running version, and assess whether failures are widespread.
Useful logs include request/action identifiers, outcome categories, and timing.

Exclude session secrets and full user expressions from default logs.

Provide documented controls to disable room publication and large effects
independently of the calculator.

## Local or recorded fallback

The course permits local operation and video. Prepare a checked fallback that
shows real server-side calculation, errors, persistent history, and reuse.

Use a separate local database file; public history is not synchronized to it.
Before going offline, prepare the Go executable, dependencies, and built client
assets, and verify schema initialization. Exercise startup, restart, and the
core path with the public host and external network unavailable.
The fallback may omit rooms when audience networking is unavailable.

## Acceptance

For public hosting, open the URL on a phone, calculate, and reload history.
Verify that application restart and replacement reopen the same file and
preserve committed records. Exercise backup and restore without mixing
the public database with the local fallback.

If rooms are included: verify room deep links, multiple devices, reconnect,
the selected classroom workload, and the ability to disable shared features.

In either case, exercise the chosen fallback before treating the deployment as
demo-ready.
