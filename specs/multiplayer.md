# Multiplayer

> Scope: Optional rooms, shared activity, and reactions.

## Purpose

Participants share calculations and react to results in a room.

Provide one preconfigured demo room.

The backend is enabled with `ROOMS_ENABLED=true`; it is off by default while
the room interface is integrated separately. See the
[room API guide](../docs/multiplayer-api.md) for local commands and wire payloads.

## Personal and room entry

- `/` opens the private calculator.
- `/room/{code}` opens a clear invitation to the specified room.
- The invitation explains that new room-mode calculations are shared.
- Joining uses the existing anonymous identity and a generated display alias.
- No account, email, or mandatory nickname form is needed.

Room entry uses the existing anonymous identity. Anyone with the link can join.

The same browser identity has one public participant identity within the room.
Count presence and collective achievement thresholds once per identity,
regardless of tab count.

## Default publication

Each new deliberate calculation in room mode requests publication automatically.

The calculation request carries its room and explicit publication intent as
defined in [Application Service](application-service.md).

- Opening personal history does not publish it.
- Restoring an old expression into the editor does not publish it.
- Calculating that expression again in room mode creates a new action that is
  eligible for publication.
- A personal-mode tab remains private even when another tab is in a room.
- Leaving room mode stops publication of subsequent calculations.

Previously published events are not retracted by leaving or disabling future
publication.

## Optional publication control

When included, provide a clearly labeled control for publishing **new
calculations**. It starts on for a newly entered room view.

When off:

- calculations still run and enter personal history;
- their expressions, results, and activity counts do not enter public room data;
- the user still sees the room and may deliberately react to public events;
- presence and explicit reactions remain public.

The state survives a reload of that room view. Theme, layout, language, or
connection recovery must not silently turn it back on. Another tab does not
silently change this tab's publication state. Private mode always overrides it.

Changing the control does not retroactively publish calculations made while it
was off, and does not retract an already submitted public action.

## Shared feed

Publish successful, grammar-validated calculations with:

- display alias;
- expression and result;
- angle context where needed to interpret the expression;
- server order/time;
- public achievement or reaction information.

The canonical result is authoritative. Clients format it without changing it.
Never insert raw syntax failures or arbitrary submitted text into the feed.
In the initial room scope, mathematical failures stay in personal history and
do not contribute public activity counters. Collective calculation rules use
successful published results.

Render all public strings as text. Public IDs support identity and deduplication
but are not displayed as the user's name. Private history IDs and session
secrets are never public event fields.

Keep the visible and retained feed bounded. The initial target is 100 recent
calculation events per room; older personal records remain in personal history.
An interface may show a smaller window while keeping recent entries reachable.

## Emoji reactions

If room reactions are included:

- offer a fixed set of emoji reactions;
- expose localized accessible labels for the reactions;
- allow one active reaction per participant per event;
- selecting another reaction replaces the previous one;
- selecting removal clears it;
- the server computes aggregate counts and validates the target event.

Repeated identical requests do not increment a count. Removed or expired feed
entries cannot be targeted to create invisible new activity.

Update reaction counts in place, without producing individual toasts or scenes.

## Presence and reconnection

Presence is approximate and reflects active room views, deduplicated by
participant identity. Closing or leaving one tab must not make another active
tab appear to have left. Explicit departure closes that view's subscription;
connection expiry handles abandoned views.

Use SSE for server-to-client updates. Snapshot, ordering, and resumption
contracts are defined by the application service.

- Replayed event IDs do not create duplicate feed entries or reactions.
- A missed retention window or a service restart produces a fresh snapshot.
- Reconnection does not replay expired scenes or re-announce earned awards.
- New participants receive a current snapshot with announcements suppressed.

The implementation retains the last 100 public calculations, their reactions,
and event badges in SQLite across restarts. Presence and replay state reset;
the new stream epoch requires a new join and snapshot. Persistent personal
history and personal achievements also survive restart.

## Collective discoveries

The collective rules in [Fun & Chaos](fun-and-chaos.md) derive events from shared
results and reactions.

Collective triggers:

- are calculated by the server;
- require distinct participant identities when their meaning is collective;
- consider only opted-in actions;
- have a room-wide cooldown;
- preserve the mathematical result and normal input;
- render in each viewer's selected UI language.

## Degradation and control

Calculation and persistence do not depend on an open SSE connection.

Distinguish “live updates disconnected” from “your calculation was not
published.” The author's subscription may be disconnected while the server
still publishes their opted-in calculation to other viewers.

If publication fails, return the saved result with publication `unavailable`.
Keep that action unpublished after recovery.

Provide controls to disable room publication and collective effects independently
of calculation. Enforce request limits and effect cooldowns separately.

## Acceptance

With at least two independent browser identities:

- join and exchange real calculation events;
- confirm private history and private-mode calculations remain private;
- verify angle context and event order;
- disconnect and reconnect without duplicates;
- continue calculating when live updates fail.

If reactions are included, verify replacement, removal, and distinct-user
counts. If publication control is included, verify off/on transitions, reload,
and absence of retroactive publication. Collective thresholds require the
actual number of independent identities configured by the rule.
