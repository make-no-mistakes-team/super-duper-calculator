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

| Expression | Expected behavior |
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
| `sin(30°)` / `sin(-30°)` | Approximately `0.5` / `-0.5` |
| `sin(pi/2)` / `sin((30+60)°)` | Approximately `1` |
| `asin(1)` | Approximately `pi/2`, in radians |
| `asin(1)*180/pi` | Approximately `90` |
| `0^0` | `1` under the specified convention |
| `1/0` / `0^-1` | Division-by-zero error |
| `sqrt(-1)` / `ln(0)` | Domain error |
| `(-8)^(1/3)` | Real-domain error, not implicit complex arithmetic |
| `tan(90°)` / `tan(pi/2)` | Domain error; `tan(90°)` span `[0, 8)` |
| `1e308°` | Numerical overflow in the intermediate `x*pi` |
| `30°%` / `30%°` / `30°°` | Rejected stacked postfix operators |
| `exp(1000)` | Numerical overflow |
| `2+(` / `2+3 trailing` | Rejected syntax; no valid-prefix success |
| `unknown(1)` / `sin(1,2)` | Unknown identifier / wrong arity |
| `2pi` / `2(3+4)` | Rejected implicit multiplication |

Use appropriate tolerances for transcendental and decimal approximation.
Also cover negative zero, subnormal/underflow behavior, overflow in ordinary
operators, whitespace, and each input/work-budget boundary.
Check Unicode diagnostics in UTF-16 code units, not UTF-8 bytes. Degree
conversion shares postfix precedence with enabled factorial and percentage;
parentheses permit explicit combinations such as `(30%)°`.

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
- restoring an expression preserves its exact source, including any `°`, and
  does not submit or alter the stored outcome;
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
For schema 5, verify that `005_expression_angle_notation.sql` drops the obsolete
angle column while preserving history, outcomes, awards, scene state, and
discovery checkpoints. Do not convert historical expressions or run a legacy
evaluator. Requests require only `requestId` and `expression`; obsolete
`angleUnit` payloads return HTTP 400. Capabilities expose `binary64-v2` and `°`,
without angle-mode metadata.

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
4. Restore an unchanged expression and deliberately calculate it again.
5. Observe behavior when the service becomes unavailable.
6. Deliver responses out of order and confirm a stale result cannot take over.
7. Open, switch, and close the tool bay without losing editor or result state.
8. Submit from the phone keypad; confirm the sheet closes and focus remains on
   an available control.
9. Insert a function around selected text and check the caret and expression.
10. Use “Go to error” for a known span in unchanged source, then edit that source
    and use “Restore expression”; neither action claims to fix the mathematics.
11. Open Statistics independently of History and inspect its three counts and
    expandable details.
12. Inspect ordinary copy in Jura, main controls in Tiny5, and mathematical
    text in PT Mono using actual rendered fonts, not just CSS family names.
    Retain function examples; the comma control must have the short accessible
    name “Запятая”, no argument explanation, and no tooltip.

Use browser automation for critical flows and inspect the actual interface for
layout, motion, and interaction changes.

`make check-browser` builds the real application and runs the repository's
Chromium scenarios with per-test temporary SQLite files and browser identities.
After `make setup`, provision Chromium once:

```sh
(cd web && npx --no-install playwright install --with-deps chromium)
make check-browser
```

The installer downloads the pinned browser and installs its system dependencies;
on Linux this may require sudo. Installation is separate from `make setup` and
`make check`, which do not require a browser.

The HTML report is in `web/playwright-report/`; failure screenshots, traces,
and server logs are in `web/test-results/`. Both directories are ignored by Git.
Inspect the report with `(cd web && npx --no-install playwright show-report)`.
Network interceptions may delay real replies or simulate failure; they must not
replace calculation or history data.

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
Verify ordinary comments' 15-second cooldown and newest-only behavior, scene
suppression and the separate 120-second cooldown, safe dismiss/automatic
recovery, and unaffected real outcomes when effects are disabled.
Expiry during a transaction must be tied to the actual claim write phase, not
to the number of internal clock observations.

For awards, exercise the real collection and ceremony:

- Confirm all eight original 64px icons remain sharp at integer scale, names
  remain visible, and locked secret conditions are absent from descriptions,
  tooltips, and accessible labels until earned.
- Check one flat grid ordered as earned, ordinary locked, then secret locked,
  preserving catalog order within each category, without group headings.
  Keep the earned counter and generic locked status; omit the “Secret” badge
  and verbose concealed-condition copy.
- Inspect opaque 16px-rounded windows and 12px-rounded cards, two desktop
  columns and one phone column. Verify the 300ms final-size unmask leaves the
  header, close control, and focus immediately usable; reduced motion is
  instant. Scroll and focus cards, including the ceremony's selected card:
  only the inner body may scroll, and outer `overflow: clip` must keep the
  header visible. Check Close/Escape, opener restoration, and loading/
  unavailable retry.
- Verify actual Monocraft Regular 400 and Bold 700 rendering for all collection
  and ceremony copy, including headings, names, descriptions, status, counts,
  dates, and controls. Cover all 66 Russian letters, ASCII, and cipher glyphs.
  Check local Jura 5.3.0, official Monocraft v4.2.1 provenance in `fonts.css`,
  and both complete SIL OFL licenses.
- Observe the locked secret's fixed two-line ASCII pattern, independent of the
  real condition and its length. Verify two glyph changes at 4Hz only while
  visible, open, foregrounded, and motion-allowed; closed, offscreen, hidden,
  and reduced-motion descriptions remain static. Accessible text must stay
  static and must not announce fake cipher frames.
- Earn a fresh secret through the real calculation POST and observe its 600ms
  progressive decode in the new ceremony. Also grant it while its locked
  secret card is already visible in an open collection. Ordinary grants,
  bootstrap, quiet repair, collection reads, reopen, reload, and identity
  replacement must not decode. Reduced motion reveals earned text instantly.
- Deliver genuinely new awards in out-of-order calculation responses. The
  newest result must remain visible while every same-generation award queues
  exactly once. A subsequent Enter submission must not clear the ceremony.
- Verify a 1.25-second burst and 7.5-second readable interval, hover/focus/
  background pause, no automatic focus theft, and the collection action opening
  the matching earned card.
- Reload, read history, refresh the collection, retry an action, and replace
  identity; none may replay old awards or retain a previous owner's queue.
- Submit 25 accepted calculations, including mathematical errors, to earn
  `touch_grass`. Counts 50 and 100 supply comments, not additional awards.
- Observe the original five-step synthesized audio after a trusted gesture;
  muting must stop active voices and persist across reload. Sound defaults on
  independently of humor, `«Спецэффекты»`, and OS reduced motion.
- Observe the intentional calculate-button, Enter, and keypad dispatch tick:
  one quiet 65ms triangle voice starting at 440Hz, without waiting for success.
  Typing, empty input, Shift+Enter, IME composition, retries, and collection
  browsing must remain quiet. Rapid dispatches replace rather than queue
  ticks; the award cue preempts them and retains priority. Mute and disposal
  stop both voice sets in their shared audio context.
- Turn humor and special effects off separately: a basic earned notice must
  remain, and reduced motion must separately limit visual movement.

## Public rehearsal and CI

CI runs the critical deterministic engine and service checks, including the
[persistence scenarios](#service-and-persistence-behavior), and the real-browser
scenarios through `make check-browser`. Tagged release builds use the same
browser gate before publication.
Add regression coverage for discovered behavioral defects.

Public deployment checks use the real hosted version and the workload specified
in [Public Deployment](public-deployment.md).

The short manual checklist in [Demo & Operations](demo-and-operations.md) applies
only to the features actually included in the candidate.

## Acceptance

The mathematical corpus, persistence and isolation checks, and client scenarios
pass. Each included feature has been exercised in the application.
