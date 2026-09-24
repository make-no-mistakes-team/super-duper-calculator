# Demo & Operations

> Scope: Local solo demonstration of the released calculator. The audience
> segment is omitted unless rooms are implemented, selected, and verified.

## Narrative

Start with the calculator. Show a correct result, recoverable error, and saved
history before the optional personal reaction. Keep the browser on the local
service during the fallback; it uses its own database.

Prepare a short core sequence. Public rooms and a language switch are not part
of this local solo candidate.

## Short core and character sequence

1. Open the default Russian interface.
2. Calculate `sqrt(81)+2^3` and obtain `17`.
3. Open personal history; restore the saved expression and calculate it again.
4. Reload the page and confirm the saved records remain.
5. Submit a malformed expression such as `sqrt(81` and show a recoverable error.
6. With personal discoveries enabled and a fresh anonymous identity, calculate
   `60+7` and show the real result `67` alongside “Сикс-севен!”.
7. If the current candidate includes another complete personal capability,
   demonstrate it once. Finish with the calculator usable.

The `six_seven` reveal needs a fresh anonymous browser context, humor enabled,
and no active announcement cooldown. Keep `67` visible during the reaction.

Suitable additional reveals include:

- `6*7` for the `42` discovery;
- `23*3` for the `69` discovery;
- `sqrt(81)+abs(-2)+ln(e)` for the scientific-function achievement;
- opening statistics and the achievement collection from history;
- switching between violet and amber themes while preserving input;
- three deliberate `1/0` submissions for the dismissible comic incident.

Only use a reveal if its complete feature is in the released candidate.

## Offline release artifacts

Obtain the local candidate from GitHub Releases under the
[release policy](../PLAN.md#release-policy). Before going offline, download a
runnable archive for the presentation device's OS and CPU architecture.
Keep the executable and built web assets together.

The downloaded build must start without the repository, development tools,
a Node runtime, or external network access. Initialize a separate local
database; public history is not synchronized to it.

## Rehearsal

Rehearse the selected artifact on the presentation device with external
network access unavailable. Walk through the browser sequence, then restart
the local process with the same SQLite file and browser identity. Confirm that
saved results and errors remain available and that a restored expression can
be calculated again.

For the browser presentation, use a fresh anonymous context for a first
achievement and wait out any previous announcement cooldown. Check the
visual error and the click-to-reuse history action; API checks alone do not
establish browser behavior.

## Candidate checklist

For every candidate:

- scientific result and angle-mode behavior;
- understandable syntax and domain errors;
- committed records retained after restart and reopening the same file;
- private history and clickable reuse;
- a usable laptop and phone core layout;
- the copied and rehearsed local fallback on the presentation device.

For each included extra:

- jokes and achievements trigger without spamming;
- theme switches preserve state;
- disabling achievements leaves ordinary calculation and history available.

Exclude unfinished features from the candidate.

Leave at least 15 seconds between discovery announcements. The comic incident
has a separate 120-second per-identity cooldown and ends within three seconds.
Settings can independently suppress comments and large effects; awards still
appear in the collection.

## Recovery

If the public host or network fails, leave its process and database alone.
Start the downloaded release on the presentation device using the rehearsed
local configuration and database. Ordinary restarts must retain its history.
Use the documented service and presentation controls to disable optional
features if needed. Do not remove or copy a live SQLite file alone; follow the
[backup and restore](application-service.md#backup-and-restore) procedure.

## Acceptance

On the presentation device, exercise startup, restart, and the browser sequence
with the exact candidate artifact. Record its GitHub release, tag, source
commit, and actual capability flags.
Freeze the candidate only after the required checks pass; optional unfinished
work is excluded, not treated as complete.
