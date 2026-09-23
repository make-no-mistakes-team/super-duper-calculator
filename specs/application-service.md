# Application Service

> Scope: Required application boundary, with optional feature contracts.

## Architecture and authority

One Go application owns calculation, anonymous browser identity, persistence,
and optional room events. The calculation engine remains an independent package.
The service computes results, achievement awards, and statistical totals from
validated input.

Production browser requests are same-origin HTTP/JSON. The development setup
must preserve this model through a proxy or equivalent arrangement.

The contracts below fix cross-boundary meaning. Internal database layout,
parser libraries, and component structure remain implementation choices.

## Database configuration

Use SQLite in development, integration tests, and deployment through the
pure-Go `modernc.org/sqlite` driver, pinned to `v1.58.0`. Run one Go application
instance. The database requires persistent local storage with working file
locks and synchronization. Network filesystems and ephemeral disks are unsuitable.

`DATABASE_PATH` names the database file and defaults to `data/calculator.sqlite`.
At startup, call `storage.Open(ctx context.Context, path string) (*sql.DB, error)`
in `internal/storage` to open or create the file and configure SQLite.
Close the connection on shutdown. New installations use an empty file without
importing records.

Keep the database, its sidecar files, and its parent directory private to the
application's operating-system user, outside public assets and source control.
Opening or configuration failures stop startup. File access, locking, disk-full,
and migration errors must preserve existing files; never reset automatically or
fall back to memory.

On POSIX systems, the immediate database directory, existing database, and
existing SQLite sidecars must belong to the application's effective user and
have no group or other permission bits. Reject symlinks at those paths.
Validate existing paths without changing their permissions or contents.
On Windows, access remains governed by the operating system's ACLs; POSIX mode
checks do not establish ACL privacy.

The opener applies this policy:

- Enable `journal_mode=WAL` and verify that SQLite selected WAL.
- Enable `foreign_keys=ON` on every physical connection, including replacements,
  before starting a transaction.
- Set `synchronous=FULL` and `busy_timeout=5000` on every connection.
- Set `SetMaxOpenConns(1)` and `SetMaxIdleConns(1)` on the `database/sql` pool.

SQLite permits only one writer at a time. The single application connection
serializes database access. Keep transactions short, close result sets promptly,
and leave automatic checkpointing enabled. Evaluation, network calls, and SSE
delivery must happen outside transactions. Use request contexts to bound waits
for the Go connection; the busy timeout only bounds SQLite lock waits.
Return safe service errors for lock contention and I/O failures; retries must
be bounded.

WAL with `synchronous=FULL` synchronizes each transaction's commit before success
is reported, subject to the operating system and disk honoring synchronization.
Do not reduce durability settings to meet the audience workload.

References: [WAL](https://www.sqlite.org/wal.html),
[foreign keys](https://www.sqlite.org/foreignkeys.html#fk_enable),
[durability](https://www.sqlite.org/pragma.html#pragma_synchronous), and
[busy timeouts](https://www.sqlite.org/pragma.html#pragma_busy_timeout).

### Schema migrations

Store schema changes as versioned SQL migrations and apply the same migrations
in each environment before accepting business traffic. Record applied versions
and use transactional schema changes where SQLite supports them. Migrations
must initialize a fresh file and preserve existing records on upgrade.

Embed migrations from `internal/storage/migrations/NNN_name.sql`, numbered
consecutively from `001`. `PRAGMA user_version` records the applied version.
Apply pending SQL and version updates in one transaction; roll back the whole
upgrade on failure. Reject a database version newer than the application
supports. Add migrations for schema changes rather than editing an applied file.

Database constraints enforce identity-scoped action uniqueness, ownership
relationships, and one-time achievement awards. Preserve canonical result
strings losslessly; SQL affinity or formatting must not replace them with
rounded display values.

### File lifecycle

Restarts and application replacements reopen the same resolved database path.
Stop the old process before its replacement opens the file. Ordinary shutdown
closes the connection and retains the database.

To reset local data, stop the application, choose a new, unused `DATABASE_PATH`,
and restart. The previous files remain untouched. Never delete live database
or WAL files. If history is unexpectedly empty, check the resolved path.

### Backup and restore

Create live backups with the [Online Backup API](https://www.sqlite.org/backup.html)
or [`VACUUM INTO`](https://www.sqlite.org/lang_vacuum.html#vacuuminto).
Write each snapshot to a separate private path outside public assets.
Verify that the operation completed successfully and check the snapshot before
using it. An interrupted export may be incomplete.

Never copy the live main file alone: its
[`-wal` file](https://www.sqlite.org/wal.html#the_wal_file) may contain committed
data. Separating them can lose data or corrupt the database.
Copy files directly only after all database users close cleanly and checkpointing
completes. After a crash, or when a WAL remains, keep the file set together and
let SQLite recover and close it, or create a snapshot as above.
Never remove the WAL by hand.

Stop the application before restoring. Place a checked snapshot at a fresh
private path, without sidecar files from another database, and preserve the
original. Set `DATABASE_PATH` to the restored file and start a compatible
application version. Apply required schema migrations, then verify readiness,
retained records, and identity isolation before resuming traffic.

### Health

`GET /health/live` reports HTTP process liveness independently of SQLite.
`GET /health/ready` performs a real, bounded SQLite query. Success returns
HTTP 200 with `{"status":"ok"}`; an unavailable query returns
HTTP 503 with `{"status":"unavailable"}` and no internal error details.
Health checks neither establish an anonymous identity nor create business data.

`GET /health/version` reports build metadata independently of SQLite and API
admission limits: `version`, plus `revision`, `modified`, and `goVersion` when
available from the Go build. It does not expose configuration or session data.

### Runtime request policy

The initial single-process service uses these bounds:

| Resource | Limit |
|---|---|
| Aggregate API rate | 50 requests/second, with a burst capacity of 100 |
| Executing API handlers | 64 |
| API request deadline | 10 seconds |
| Buffered API response | 1 MiB |
| Calculation request body | 16 KiB |
| Accepted HTTP connections | 128 |
| HTTP header budget | 16 KiB |
| Header / request read timeout | 5 / 10 seconds |
| Response write / idle timeout | 15 / 60 seconds |
| Database startup deadline | 10 seconds |

All API routes share one rate and admission budget, including session creation
and history reads. Changing anonymous identities or source addresses does not
reset it. There is no per-IP quota that would treat the classroom's shared
address as one user. Health endpoints and static assets remain outside the API
budget; the connection and HTTP timeout bounds still apply to them.

Rate rejection returns HTTP 429 with `RATE_LIMITED` and a `Retry-After` delay in
whole seconds. Saturation or a request deadline returns HTTP 503 with
`SERVICE_UNAVAILABLE`. A timed-out handler retains its admission slot until its
work exits. Request contexts bound database-pool waits; SQLite's separate
`busy_timeout=5000` remains unchanged.

Buffer API responses so a panic, timeout, or response-size failure cannot expose
partial data or cookies. A recovered panic returns a safe HTTP 500 response.
Log status, failure category, and elapsed time without request bodies, session
cookies, raw exceptions, or SQL. Startup errors identify the failing stage
without printing the underlying database exception or configured path.

The current API guard handles finite JSON responses. Room subscriptions require
their own bounded streaming policy when rooms are enabled; do not wrap SSE in
the buffered response guard. Measure the selected deployment against the
audience workload before treating these bounds as public-readiness evidence.

## Browser identity

`GET /api/session` establishes or resumes an opaque anonymous session cookie.
It returns a display alias and the session's personal achievement state if that
feature is enabled, but not the secret identity token.

The cookie must be unpredictable, persistent across ordinary browser restarts,
HttpOnly, SameSite-protected, and Secure on HTTPS deployments. A persistent
expiry of at least the planned public deployment lifetime is required.

Personal records are always scoped from this server-validated cookie. A
client-supplied identity, history owner, or display alias cannot grant access to
another person's data. Missing identity creates a new anonymous context through
the session operation; it does not expose a global history.

An absent, invalid, or expired cookie on calculation or history requests returns
HTTP 401 with `SESSION_REQUIRED`, without creating a session or a calculation.
The client calls `GET /api/session` before retrying. Session bootstrap also
checks browser origin metadata, since it can create persistent state.
JSON API responses use `Cache-Control: no-store`.

Accounts, password recovery, and cross-device synchronization are out of scope.

## Capabilities

`GET /api/capabilities` reports:

- `semanticsVersion`;
- supported operators and function arities;
- supported angle units and default `deg`;
- the advertised input budgets;
- optional feature availability: operation extensions, statistics,
  achievements, themes, minimal presentation, localization, reduction playback,
  rooms, room reactions, and room publication control.

Availability controls which features the UI offers. The server validates every
request independently.

## Operations

| Method and path | Purpose |
|---|---|
| `GET /health/live` | Check HTTP process liveness |
| `GET /health/ready` | Query SQLite to check database readiness |
| `GET /health/version` | Identify the running build without database access |
| `GET /api/session` | Establish or resume anonymous identity |
| `GET /api/capabilities` | Discover supported behavior |
| `POST /api/calculations` | Evaluate and durably record an accepted calculation |
| `GET /api/history?limit=50&cursor=...` | Page through personal history |
| `GET /api/calculations/{id}/reduction` | Optional read-only reduction data for an owned record |
| `GET /api/statistics` | Optional personal statistics |
| `POST /api/rooms/{code}/join` | Optional room entry and initial snapshot |
| `POST /api/rooms/{code}/leave` | Optional room departure |
| `GET /api/rooms/{code}/events` | Optional SSE subscription and resumption |
| `PUT /api/rooms/{code}/events/{eventId}/reaction` | Optional reaction selection or removal |

Identifiers and cursors are opaque to the client. History ordering and paging
must not depend on comparing localized timestamps.

## Calculation request

A private request has this form:

```json
{
  "requestId": "client-generated-unique-action-id",
  "expression": "sqrt(81)+2^3",
  "angleUnit": "deg"
}
```

An intentional room action additionally carries:

```json
{
  "room": {
    "code": "demo",
    "publish": true
  }
}
```

`requestId`, `expression`, and `angleUnit` are required. If `room` is present,
its code and explicit Boolean `publish` are required.

Omitting `room` always means private calculation, regardless of room membership
in another tab. The room UI sends explicit publication intent on each request.

`publish: false` is the basis of the optional room publication control. Joining
or watching a room by itself does not publish an expression.

## Calculation record and response

An accepted request returns a record with:

- `id`, `requestId`, and server `createdAt` in UTC;
- the original `expression`;
- `context`: effective `angleUnit` and `semanticsVersion`;
- `outcome`: either `{ "kind": "success", "value": "<canonical value>" }` or
  `{ "kind": "error", "error": <mathematical error> }`;
- optional trusted calculation facts for enabled presentation features.

A mathematical error has `code`, `stage`, safe `params`, and a nullable `span`
with `start` and `end` offsets as defined by the engine contract.

The response contains `calculation`, optional newly earned `achievements`, and
`publication` with status `private`, `published`, or `unavailable`.
`published` means the event entered the room's bounded event stream; delivery
to connected screens is asynchronous. `private` means no public contribution
was made. Mathematical failures remain in personal history. The response status
does not change the room view's publication setting for subsequent calculations.

When humor is enabled, the response may also contain `funEvents`. Each event has
an `id`, `ruleId`, `kind` (`comment` or `scene`), `scope` (`personal` or `room`),
safe `params`, and server `createdAt`/`expiresAt` timestamps. Expired or previously
seen events are not replayed. Collective events use the same representation
inside the room stream.

Personal achievement entries contain a stable achievement `id` and `earnedAt`;
the calculation response reports newly earned entries, while session bootstrap
returns the existing collection without requesting new announcements.

An accepted syntax or domain error is still a calculation record and uses
HTTP 200. Request failures use the error responses below and create no record.

## Durable submission and repeated actions

Commit calculation records in short transactions before returning success
or publishing room events. Any persistent counters or awards claimed by the
response must also be committed consistently with the action.

Optional statistics or fun failures may omit their extras, but must not prevent
the core calculation from being saved and returned. Do not claim an uncommitted
award. Derived state can be reconciled from authoritative history without
creating additional calculations or replaying old announcements.
Isolate optional SQL work using separate short transactions, or savepoints only
for errors that permit rollback to that savepoint.

If saving the calculation fails, return a service error. If room fan-out fails
after persistence, return the saved result with publication `unavailable`.

Each deliberate calculation action gets a new `requestId`, even when its
expression is unchanged. A retry of the same action reuses the ID.

Within an anonymous identity:

- the same ID and the same expression, angle setting, and publication context
  return the original record without additional history, awards, or events;
- reuse of that ID with different semantic input returns HTTP 409;
- records from another identity cannot be retrieved by guessing its request ID.

A retry offered after an uncertain network outcome must preserve the action ID.

While rooms are unavailable, reject a new room-context action with HTTP 400
(`UNSUPPORTED_CONTEXT`). A valid room context added to an already saved private
action is a changed input and returns HTTP 409. Optional module delivery and
failure-isolation checks apply when those modules are enabled.

## History response

Return `{ "items": [...], "nextCursor": null }`, or a non-null continuation
cursor when older owned records remain. The default page size is 50 and the
maximum is 100.

Records retain the submitted source and effective context. Restoring a record
does not submit a calculation. Locale-dependent dates, labels, and error
messages are rendered by the client.

The service validates that a continuation cursor identifies an owned record.
Malformed or foreign cursors return HTTP 400 without disclosing whether another
identity owns the referenced record. Historical records with absent optional
facts remain readable; history never invokes the current evaluator.

## Optional statistics response

`GET /api/statistics` requires a valid session and returns metrics from that
owner's complete history. An empty history returns:

```json
{
  "totalCalculations": 0,
  "successes": 0,
  "mathematicalErrors": 0,
  "divisionByZeroAttempts": 0,
  "operators": {},
  "functions": {},
  "longestExpression": null,
  "maxParsedDepth": null
}
```

Operator and function maps count recorded occurrences, not source-text matches.
`longestExpression`, when present, contains `calculationId`, `expression`, and
`length` in UTF-16 units; ties keep the earliest accepted record.
`maxParsedDepth` is null until parsed facts exist, and can be zero for a literal.
Parse failures and historical records without facts supply no usage or depth;
their outcomes and expression lengths still count.

Reads reconstruct the metrics without changing history or issuing awards.
Invalid metric data returns a safe 503; calculation and history remain independent
of this derivation. `STATISTICS_ENABLED` defaults to true. When false, capabilities
advertise statistics as unavailable and this route returns 404; actions still
enter history and appear in statistics when it is re-enabled.

## Optional reduction response

This is a read-only operation on a successful, owned calculation. It never adds
history, changes statistics, awards an achievement, or publishes a room event.

The response includes:

- `calculationId`;
- `initialExpression`;
- `steps`, an ordered array;
- `finalExpression`, the stored canonical result.

Each step has:

- `before`: the full expression shown at the start of the step;
- `span`: the half-open UTF-16 range of its redex in `before`;
- `replacement`: the evaluated value, with grouping if needed;
- `after`: the expression after the replacement.

The first `before` equals `initialExpression`. Each `after` equals the next
`before`, and the final `after` equals `finalExpression`. Literal-only inputs
may have no reduction steps; their canonical final value is still returned.

Generate reduction data on demand using the record's mathematical context.
Verify the final value against the stored outcome. If an older semantics
version cannot be reproduced, return `REDUCTION_UNAVAILABLE` and retain the
historical result.

## Optional room data

Room snapshots identify the room, its current stream epoch and sequence,
approximate presence, recent public events, and reaction aggregates.

A successful public calculation event contains only:

- a public event ID and order;
- a public participant ID and display alias;
- the validated mathematical expression;
- canonical result and relevant angle context;
- server time and applicable public achievement IDs.

Private history identifiers, identity cookies, and raw rejected input are not
room data. In the initial room scope, mathematical failures remain personal:
they do not produce feed entries or public activity counters.

SSE events use stable event IDs, an epoch, and increasing sequence numbers.
Reconnect uses `Last-Event-ID`. Replay retained events or send a fresh snapshot.
Restoring a snapshot must suppress old joke and achievement announcements.

Reaction requests set one allowed reaction ID or `null`; they do not increment
a client-supplied counter. Detailed room behavior is in
[Multiplayer](multiplayer.md).

## Localization boundary

Codes, achievement IDs, reaction IDs, and numeric syntax do not depend on
language. User-facing system prose must not be persisted or broadcast as the
only representation of an outcome.

The client renders Russian or English text from stable IDs and safe parameters.
The same collective event can therefore appear in different languages on
different devices. Original expressions and display aliases are user data,
not translation keys.

## Request and service errors

Errors outside accepted mathematical outcomes use:

```json
{
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "params": {}
  }
}
```

| HTTP status | Meaning |
|---|---|
| 400 | Invalid request shape or unsupported context |
| 401 | Session required; establish or resume identity through `GET /api/session` |
| 403 | Browser origin rejected |
| 404 | Missing resource, including a calculation not owned by this identity |
| 409 | Reused action ID with different input, or unavailable historical reduction |
| 413 | Expression or request work budget exceeded |
| 429 | Request rate limited; include retry guidance |
| 500/503 | Safe internal or availability error |

No response exposes stack traces, SQL, secrets, or arbitrary exception text.
Mutation endpoints require same-origin browser protection; public deployment
must not enable permissive credentialed cross-origin access.

Unknown API routes return the same JSON error envelope with HTTP 404, including
when built client assets are present. An uncertain timeout or lost response may
follow a commit; retry the original `requestId` rather than creating a new action.

## Acceptance

A real browser can calculate, recover from an expression error, reload owned
history, and reuse an expression with its angle context. Two browser identities
cannot read one another's personal records. Retrying an uncertain submission
does not duplicate its observable effects. Optional room or reduction failures
do not invalidate a saved calculation.
