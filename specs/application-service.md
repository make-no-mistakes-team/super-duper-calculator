# Application Service

> Scope: Required application boundary, with optional feature contracts.

## Architecture and authority

One Go application owns calculation, anonymous browser identity, persistence,
and optional room events. The calculation engine remains an independent package.
PostgreSQL stores persistent data. The service computes results, achievement
awards, and statistical totals from validated input.

Production browser requests are same-origin HTTP/JSON. The development setup
must preserve this model through a proxy or equivalent arrangement.

The contracts below fix cross-boundary meaning. Internal database layout,
parser libraries, and component structure remain implementation choices.

## Database configuration

Use PostgreSQL in development, integration tests, and deployment. The Go service
reads its connection settings from `DATABASE_URL` and uses a bounded connection
pool. Keep credentials out of client assets and logs.

Store schema changes as versioned SQL migrations. Apply the same migrations in
every environment before the application accepts traffic. Database constraints
must enforce action identity and one-time achievement awards.

Local PostgreSQL runs in Docker Compose. Container storage, readiness, and
deployment requirements are defined in [Public Deployment](public-deployment.md).

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

Calculation records must be durable before a successful application response.
Any persistent counters or awards claimed by that response must also be
committed consistently with the action.

Optional statistics or fun failures may omit their extras, but must not prevent
the core calculation from being saved and returned. Do not claim an uncommitted
award. Derived state can be reconciled from authoritative history without
creating additional calculations or replaying old announcements.

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

## History response

Return `{ "items": [...], "nextCursor": null }`, or a non-null continuation
cursor when older owned records remain. The default page size is 50 and the
maximum is 100.

Records retain the submitted source and effective context. Restoring a record
does not submit a calculation. Locale-dependent dates, labels, and error
messages are rendered by the client.

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
| 404 | Missing resource, including a calculation not owned by this identity |
| 409 | Reused action ID with different input, or unavailable historical reduction |
| 413 | Expression or request work budget exceeded |
| 429 | Request rate limited; include retry guidance |
| 500/503 | Safe internal or availability error |

No response exposes stack traces, SQL, secrets, or arbitrary exception text.
Mutation endpoints require same-origin browser protection; public deployment
must not enable permissive credentialed cross-origin access.

## Acceptance

A real browser can calculate, recover from an expression error, reload owned
history, and reuse an expression with its angle context. Two browser identities
cannot read one another's personal records. Retrying an uncertain submission
does not duplicate its observable effects. Optional room or reduction failures
do not invalidate a saved calculation.
