# Quality & Testing

> Scope: Required correctness baseline; additional checks follow enabled features.

## Verification priorities

Verify mathematical results, history durability and isolation, publication
boundaries, and the complete user flow.

Tests should detect behavioral defects. Changes to joke wording, component
structure, or internal wiring should not break them.

## Mathematical corpus

Keep one authoritative corpus aligned with
[Calculation Engine](calculation-engine.md). It must cover valid expressions,
invalid expressions, numerical boundaries, and any enabled extensions.

Representative core expectations include:

| Expression/context | Expected behavior |
|---|---|
| `2+3*4` | `14` |
| `(2+3)*4` | `20` |
| `8/4*2` | `4` |
| `2^3^2` | `512` |
| `-2^2` / `(-2)^2` | `-4` / `4` |
| `2^-3` | `0.125` |
| `.5+1.25e-3` | Approximately `0.50125` |
| `sqrt(81)+2^3` | `17` |
| `log(8,2)` | Approximately `3` |
| `sin(30)` in degrees | Approximately `0.5` |
| `sin(pi/2)` in radians | Approximately `1` |
| `asin(1)` in degrees | Approximately `90` |
| `0^0` | `1` under the specified convention |
| `1/0` / `0^-1` | Division-by-zero error |
| `sqrt(-1)` / `ln(0)` | Domain error |
| `(-8)^(1/3)` | Real-domain error, not implicit complex arithmetic |
| `tan(90)` in degrees / `tan(pi/2)` in radians | Domain error |
| `exp(1000)` | Numerical overflow |
| `2+(` / `2+3 trailing` | Rejected syntax; no valid-prefix success |
| `unknown(1)` / `sin(1,2)` | Unknown identifier / wrong arity |
| `2pi` / `2(3+4)` | Rejected implicit multiplication |

Use appropriate tolerances for transcendental and decimal approximation.
Also cover negative zero, subnormal/underflow behavior, overflow in ordinary
operators, whitespace, and each input/work-budget boundary.

The corpus must distinguish the retained binary64 value from rounded display.
For `0.1+0.2`, copying or reusing the full result must not silently use a value
that was rounded only for display.

If enabled, extensions add:

- `0!`, `5!`, `-3!`, `(-3)!`, `3!!`, and `171!`;
- percentage semantics for `200+10%` and `200*(1+10%)`;
- remainder signs, including `mod(-7,3)`;
- precedence such as `2^3!`;
- clear rejection when an extension is disabled.

Expected percentage results follow binary64 arithmetic, including intermediate
rounding.

## Service and persistence behavior

Run persistence and service integration tests on real database files in isolated
temporary directories. Use the application's
[opener and storage policy](application-service.md#database-configuration) and
versioned SQL migrations. Do not substitute in-memory databases or shared
developer files. Close the service and database handles before removing a test
directory.

Verify:

- successful and erroneous accepted calculations survive process restart and
  reopening the same database file, including an unclean exit after commit;
- a different anonymous browser cannot obtain another identity's history or
  reduction data;
- records beyond the first history page remain reachable in stable order;
- restoring an old degree-mode expression while currently using radians
  restores its meaning before the next calculation;
- retries and concurrent submissions with the same action ID produce one record
  and one set of durable effects;
- the same expression deliberately submitted again creates a new record;
- a reused action ID with changed input is rejected;
- a database failure returns a service error without claiming an unsaved record;
- a room publication failure does not invalidate an already saved result.

Distinguish transport/request errors from persisted mathematical outcomes.

Verify that migrations initialize an empty file and preserve existing records
on upgrade. Failed migrations must not delete the file or expose a partially
applied schema to business traffic.

Exercise inaccessible paths, real lock contention, and recovery using isolated
files. An inaccessible path must fail startup rather than select another file
or memory. Hold a write transaction on a separate test connection to verify
that lock waits remain bounded, failures preserve committed records, and
submissions can resume after the lock is released. Retrying an uncertain
action still uses its original ID and must not duplicate durable effects.

Check that new and replacement connections enforce declared foreign keys.
Verify rollback of a failed action's durable changes, canonical result
round-tripping, and restoration from a
[valid backup](application-service.md#backup-and-restore) into a separate file.
Follow the [file lifecycle](application-service.md#file-lifecycle) during recovery.

Check both [health endpoints](application-service.md#health), including failed
database queries and process liveness while storage is unavailable.

## Real client verification

Exercise the actual application with keyboard and touch:

1. Enter a scientific expression and calculate.
2. Correct a highlighted error.
3. Reload and retrieve history.
4. Restore an expression and calculate again with the correct context.
5. Observe behavior when the service becomes unavailable.
6. Deliver responses out of order and confirm a stale result cannot take over.
7. Open, switch, and close the tool bay without losing editor or result state.
8. Submit from the phone keypad; confirm the sheet closes and focus remains on
   an available control.
9. Insert a function around selected text and check the caret and expression.

Use browser automation for critical flows and inspect the actual interface for
layout, motion, and interaction changes.

## Optional presentation checks

For themes, switch while editor, result, and history state exists. Verify state
preservation, readable errors, focus, and phone layout.

For localization:

- a fresh browser starts in Russian even when its browser locale is English;
- a saved English selection survives reopening;
- errors, achievements, room events, and accessible labels are translated;
- both languages preserve identical mathematical syntax and behavior;
- changing locale does not submit, publish, reset, or re-award anything;
- enabled features have complete translation catalogs.

## Optional reduction checks

Verify reduction continuity and semantic equivalence, including functions,
negative intermediate values, precedence, and literal-only input.

Exercise actual highlight/collapse playback, skip, close, replay, interruption
by a new calculation, and reduced-motion behavior. Confirm no extra history,
statistics, awards, or room events result from playback.

## Optional room and humor checks

Use independent browser identities, not merely tabs sharing one cookie.

- New opted-in calculations reach other participants.
- Personal-mode calculations and old history do not appear in room data.
- Multi-tab presence does not multiply one participant.
- If publication control is enabled, off/on transitions and reload preserve its
  meaning without retroactive publication.
- Reactions replace/remove correctly and cannot be inflated by identical retries.
- Reconnect or stream reset restores state without duplicate entries or stale
  theatrical scenes.
- Shared counters and collective rules exclude private calculations.
- A configured collective rule uses the required distinct participants.

For humor, use a controlled clock to test cooldown boundaries and expiry.
Verify persistent one-time awards, burst coalescing, safe dismiss/automatic
recovery, and unaffected real outcomes when effects are disabled.

## Public rehearsal and CI

CI runs the critical deterministic engine and service checks, including the
[persistence scenarios](#service-and-persistence-behavior).
Add regression coverage for discovered behavioral defects.

Public deployment checks use the real hosted version and the workload specified
in [Public Deployment](public-deployment.md).

The short manual checklist in [Demo & Operations](demo-and-operations.md) applies
only to the features actually included in the candidate.

## Acceptance

The mathematical corpus, persistence and isolation checks, and client scenarios
pass. Each included feature has been exercised in the application.
