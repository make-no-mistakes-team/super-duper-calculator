# Local multiplayer API

One configured room shares successful calculations, emoji reactions, and
collective discoveries. The backend is available independently of the room UI.
Use this contract when integrating the frontend; enabling the server flag does
not add a room screen to the current client.

## Start and try it

After `make setup`, run:

```sh
ROOMS_ENABLED=true ROOM_CODE=demo make dev
```

The API is at `http://127.0.0.1:8080`; the development client is at
`http://127.0.0.1:5173`. Leave `PUBLIC_ORIGIN` unset for local HTTP. Make tasks
load `.env`; explicit environment variables override its values. The default
SQLite file is `data/calculator.sqlite` and survives restart. To use a separate
test database, add `DATABASE_PATH=data/room-demo.sqlite` to the command.

These commands require curl. `mktemp` creates a private cookie file representing
one browser identity. Use another cookie file for each independent participant.

```sh
room_cookie=$(mktemp)
curl -sS -c "$room_cookie" http://127.0.0.1:8080/api/session

curl -sS -b "$room_cookie" -H 'Content-Type: application/json' \
  -d '{}' http://127.0.0.1:8080/api/rooms/demo/join
```

Copy `viewId` from the join response. In another terminal, use that ID and the
same cookie file path to observe updates:

```sh
curl -N -b /path/to/cookie-file \
  'http://127.0.0.1:8080/api/rooms/demo/events?viewId=YOUR_VIEW_ID'
```

Publish a new calculation from the first terminal:

```sh
curl -sS -b "$room_cookie" -H 'Content-Type: application/json' \
  -d '{"requestId":"room-example-1","expression":"6*7","angleUnit":"deg","room":{"code":"demo","publish":true}}' \
  http://127.0.0.1:8080/api/calculations
```

Its response includes `publication.status: "published"` and a public `eventId`.
Submitting that exact request again returns the original result. Use a new
`requestId` for another deliberate calculation. Change `publish` to `false`
with a new request ID to save a private result while watching the room.

```sh
curl -sS -b "$room_cookie" -X PUT -H 'Content-Type: application/json' \
  -d '{"reactionId":"applause"}' \
  http://127.0.0.1:8080/api/rooms/demo/events/PUBLIC_EVENT_ID/reaction

curl -sS -b "$room_cookie" -H 'Content-Type: application/json' \
  -d '{"viewId":"YOUR_VIEW_ID"}' \
  http://127.0.0.1:8080/api/rooms/demo/leave
rm "$room_cookie"
```

Leaving returns HTTP 204. Deleting the temporary cookie file ends this test
identity's client session; it does not delete its saved history.

## Session and configuration

Call `GET /api/session` before other operations. All room endpoints require its
anonymous HttpOnly cookie; a missing or invalid cookie returns 401. The cookie
identifies the owner of a calculation or reaction; never send a participant ID
to claim another person's action. Browser requests use the same origin and
normal cookie credentials. Cross-origin mutations and streams are rejected.

`ROOMS_ENABLED=false` omits the room routes. `ROOM_CODE` is the single existing
room, not a password; anyone with its URL may join. Codes contain 1–32 lowercase
ASCII letters, digits, or hyphens. A different code returns 404.

| Setting | Default | Effect when false |
| --- | --- | --- |
| `ROOMS_ENABLED` | `false` | Room routes unavailable; new room calculation context rejected. |
| `ROOM_PUBLICATION_ENABLED` | `true` | Personal calculation still saves; requested public success returns `unavailable`. |
| `ROOM_REACTIONS_ENABLED` | `true` | Reaction mutations rejected; retained counts remain readable. |
| `ROOM_EFFECTS_ENABLED` | `true` | Collective announcements suppressed; event badges remain available. |

Read `/api/capabilities` to discover enabled server features. These flags describe
API support. The room interface must still be integrated. Changes to process
configuration require a restart.

## Join, snapshot, and leave

`POST /api/rooms/demo/join`, with `Content-Type: application/json` and body `{}`,
returns HTTP 200:

```json
{
  "viewId": "ac59e7734c810a5f0ef7d7c65bfae192",
  "snapshot": {
    "code": "demo",
    "epoch": "65031b13921475868f26c58a",
    "sequence": 1,
    "publicationEnabled": true,
    "participant": {"id": "opaque-public-participant-id", "alias": "Brave Otter"},
    "presence": 1,
    "aggregates": {"publishedCalculations": 0, "activeReactions": 0},
    "calculations": [],
    "myReactions": {}
  }
}
```

`snapshot.participant` is the requesting identity's public identity in this
room. Display its `alias` in the room header. On first room participation the
server randomly assigns an English adjective and noun, each starting with a
capital letter, for example `Brave Otter`. The name is unique within the room
and persisted for that room and anonymous identity. Other tabs, later joins,
and server restarts return the same name. Publishing or reacting through the
API before joining also establishes the participant's name. A new anonymous
identity receives a new name; the display alias returned by `GET /api/session`
is separate and must not be used as the room name.

Each join creates a view for one tab. `viewId` is an opaque 32-character string,
bound to the cookie. It is not the public participant ID and must not be shared
as room content. One identity counts once in `presence` even with several views.
A joined view without a live stream expires after 45 seconds; disconnecting a
stream starts a new 45-second grace period. Join again after expiry or restart.
`publicationEnabled` reflects the operator's room publication switch; when it
is false, new successful room-mode calculations remain personal and return
`publication.status: "unavailable"` if publication was requested.

`POST /api/rooms/demo/leave` with `{"viewId":"…"}` closes only that view and its
stream, returning 204. A missing, expired, or another identity's view returns
404. Other tabs stay joined. Leaving does not retract published calculations.

The snapshot's `calculations` are in increasing server `order`, with at most
100 entries. `myReactions` maps event IDs to the requesting identity's selected
reaction IDs. It is personalized snapshot data; do not broadcast it to other
participants. An individual entry has this shape:

```json
{
  "id": "e3f0ea96cd12490ab7e8e3dcf5eef541",
  "order": 12,
  "participant": {"id": "opaque-public-participant-id", "alias": "Brave Otter"},
  "expression": "6*7",
  "value": "42",
  "angleUnit": "deg",
  "createdAt": "2026-09-30T12:00:00Z",
  "reactions": {"applause": 2},
  "achievementIds": []
}
```

`value` is the canonical **string**, not a JavaScript number. `angleUnit` is
`deg` or `rad`. Times are RFC 3339 strings and may contain fractional seconds.
`reactions` maps allowed reaction IDs to positive integer counts; absent means
zero. `achievementIds` currently contains only `peer_reviewed` when earned.
Treat every public string as text. The public event ID differs from the private
calculation ID. Ordering gaps are allowed; use IDs for deduplication.

For each feed entry, show `participant.alias` as the author's name. To highlight
own calculations or implement a “Only mine” filter, compare the entry's
`participant.id` with `snapshot.participant.id`. Do not compare names or send an
owner ID to the server. Apply this filter locally to the retained room feed:
it shows only the requesting participant's entries among the latest 100 room
calculations, not every calculation they have ever published. Keep the complete
retained feed in client state so changing the filter does not require a rejoin.

`aggregates.publishedCalculations` is the persistent total of successfully
published calculations in this room, including entries already evicted from
the feed. `aggregates.activeReactions` counts current selections across retained
events; removing a reaction or evicting its event reduces this count. Both are
nonnegative JSON integers (`int64` and `int` respectively on the server).
Private calculations, mathematical failures, unavailable publications, and
repeated requests never increase the published total.

## Calculation publication

Send each deliberate calculation to the existing `POST /api/calculations`.
The added `room` object requires both `code` and boolean `publish`. Omitting
`room` means private, even when another tab joined a room. `publish: false`
saves privately without affecting public counts. Joining and viewing personal
history never publish old records.

The ordinary calculation response gains one of:

```json
{"status":"private"}
```

```json
{"status":"published","eventId":"e3f0ea96cd12490ab7e8e3dcf5eef541"}
```

```json
{"status":"unavailable"}
```

These are the values of the response's `publication` field, not complete
responses. Mathematical failures remain private, including with `publish: true`.
Personal persistence happens before optional publication. A committed public
event remains published even if the author's stream is disconnected.
`unavailable` never becomes a later publication when retrying the same action.

Retry with the same `requestId`, expression, angle unit, and exact publication
context. Changing any of them under the same ID returns 409
`REQUEST_ID_CONFLICT`. A new deliberate calculation gets a new ID. Persist the
publication switch per tab; rendering, reload, or reconnect must not silently
turn it on. A failed publication does not change that switch.

## SSE contract

Subscribe to `GET /api/rooms/demo/events?viewId=…`. The response is
`text/event-stream`, with `Cache-Control: no-store`, no proxy buffering, and a
comment heartbeat every 15 seconds. An SSE frame looks like:

```text
id: 65031b13921475868f26c58a:2
event: presence
data: {"participants":2}

```

The SSE `id` is `epoch:sequence`; it is distinct from a calculation's public ID
and `order`. Use named listeners (`addEventListener`) for these events:

| SSE event | JSON `data` |
| --- | --- |
| `snapshot` | The snapshot object shown above. Replace current room state. |
| `presence` | `{ "participants": 2 }`; replace approximate presence. |
| `calculation` | `{ "calculation": <entry>, "aggregates": {"publishedCalculations": 12, "activeReactions": 3}, "funEvents": [...] }`; append/deduplicate by entry ID and retain the last 100. |
| `reaction` | `{ "eventId": "…", "participantId": "…", "participant": {"id": "…", "alias": "Brave Otter"}, "reactionId": "applause", "reactions": {...}, "achievementIds": [...], "aggregates": {"publishedCalculations": 12, "activeReactions": 4}, "funEvents": [...] }`; replace counts and badges. `reactionId` is null on removal. |

In a `reaction` event, `participant` identifies the person changing or removing
the reaction. Its `id` equals the retained `participantId` field. Use its `alias`
when displaying who performed the action, including a removal.

Both mutation events carry complete authoritative `aggregates`; replace the
displayed counters rather than incrementing them or adding the payload values.

`funEvents` may be absent, `null`, or an array; treat absent/null as empty. Native EventSource
reconnects with `Last-Event-ID`. When creating a new subscription manually, send
the last applied cursor as `?cursor=epoch:sequence`; the header takes precedence.
Without a valid retained cursor, the server sends `snapshot`. Within an epoch,
the replay journal retains the latest 512 changes. A new server process changes
the epoch and clears view IDs; rejoin and replace state from its snapshot.

Never announce awards or play scenes from a snapshot. The server omits all
announcements from replay. Deduplicate live fun events by their `id` and also
discard them client-side once `expiresAt` has passed. Do not queue
scenes while hidden or disconnected. Replayed state updates still apply, even
when their announcement is suppressed. With an already current cursor there
may be no initial state frame; heartbeats keep the stream alive.

Only one stream is active per view; reconnecting the same view replaces its
previous stream. A slow reader with a full output queue is disconnected and
must resume. Calculate independently of the live connection; report a lost
stream separately from `publication.status`.

The room permits at most 64 simultaneous streams and 256 live/grace-period
views. Stream admission permits 10 attempts per second with a burst of 64;
each stream has a queue of 64 messages and a five-second write deadline.

## Reactions and discoveries

`PUT /api/rooms/demo/events/{eventId}/reaction` accepts exactly one selected
reaction or removal:

```json
{"reactionId":"applause"}
```

```json
{"reactionId":null}
```

Allowed IDs are `laugh`, `wow`, `applause`, and `thinking`. The frontend chooses
their glyphs and localized accessible labels. A repeated identical selection
does not add another count. A new selection replaces the old one, including
across tabs sharing the cookie. Only retained public events can be targeted.
Reaction admission allows two requests per second per identity with a burst
of six, and 20 per second room-wide with a burst of 100. At most 256 identities
may hold a reaction on one event. The ordinary JSON API limits also apply.

HTTP 200 returns:

```json
{
  "eventId": "e3f0ea96cd12490ab7e8e3dcf5eef541",
  "participant": {"id": "opaque-public-participant-id", "alias": "Brave Otter"},
  "reactionId": "applause",
  "reactions": {"applause": 1},
  "achievementIds": [],
  "epoch": "65031b13921475868f26c58a",
  "sequence": 3
}
```

On removal, `reactionId` is null. Apply authoritative counts from the response
and stream; do not add them together. If a reaction stream event's public
`participantId` matches the snapshot's own participant, also update
`myReactions` locally. This keeps the selected reaction consistent across tabs.
This response's cursor is useful for
comparing freshness, but do not advance the global stream cursor past other
unprocessed events just because one mutation responded first.

Three different identities publishing canonical `42` within 60 seconds can
produce `shared_answer`. The server permits at most one such scene per room
per 120 seconds. Three other identities reacting to an author's calculation
award that event `peer_reviewed`; the author's own reaction does not count.
The badge remains even if reactions are removed, until its event leaves the
100-entry feed. Personal/private calculations and failed publication never
contribute to these rules.

Collective announcements have the existing fun-event shape:

```json
{
  "id": "opaque-announcement-id",
  "ruleId": "shared_answer",
  "kind": "scene",
  "scope": "room",
  "params": {
    "triggeredBy": {"id": "participant-c", "alias": "Calm Robin"},
    "contributors": [
      {"id": "participant-c", "alias": "Calm Robin"},
      {"id": "participant-a", "alias": "Brave Otter"},
      {"id": "participant-b", "alias": "Bright Fox"}
    ]
  },
  "createdAt": "2026-09-30T12:00:00Z",
  "expiresAt": "2026-09-30T12:00:10Z"
}
```

For `shared_answer`, `params.triggeredBy` is the participant whose calculation
triggered the scene. `params.contributors` contains exactly three distinct
public participants whose published answers satisfied the rule, including the
triggering participant. Each participant has `id` and `alias`.

`peer_reviewed` uses `kind: "comment"` with these parameters:

```json
{
  "eventId": "e3f0ea96cd12490ab7e8e3dcf5eef541",
  "authorId": "participant-a",
  "author": {"id": "participant-a", "alias": "Brave Otter"},
  "triggeredBy": {"id": "participant-d", "alias": "Gentle Owl"}
}
```

`author` identifies the author of the awarded calculation; `authorId` equals
`author.id`. `triggeredBy` identifies the person whose reaction reached the
threshold. Display these roles distinctly when naming participants in an
announcement. Badge assignment is independent of announcement pacing. Enforce a
15-second view cooldown for ordinary comments in the client; the server enforces
the 120-second room cooldown for the large `shared_answer` scene. Translate
`ruleId` in the viewer's language; no server prose needs to be displayed verbatim.

## Errors and integration checks

JSON errors have shape `{"error":{"code":"INVALID_REQUEST","params":{}}}`.
Invalid JSON, extra fields, invalid reaction IDs, or malformed request fields
return 400. Disabled reaction mutations return 400 `UNSUPPORTED_CONTEXT`.
Missing identity returns 401; rejected origin returns 403 `INVALID_ORIGIN`;
unknown room, inaccessible view, or evicted event returns 404 `NOT_FOUND`.
Oversized requests return 413 `REQUEST_LIMIT`; rate limits return 429
`RATE_LIMITED`; temporary resource/storage failures return 503
`SERVICE_UNAVAILABLE`. SSE errors use JSON before the stream starts; after it
starts a transport failure closes the connection.

Check with independent cookie jars/browser profiles, plus multiple tabs sharing
one identity: publication on/off, reaction replace/remove, view expiry, one-tab
leave, 101 feed entries, reconnect with retained/stale cursors, server restart,
and calculation while SSE is unavailable. The collective rules need three
distinct 42 authors, or one calculation author plus three other reactors.
