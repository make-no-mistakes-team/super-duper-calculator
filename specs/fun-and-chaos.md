# Fun & Chaos

> Scope: Optional showcase, central to the calculator's intended character.

## Tone

The calculator may tease, roast, celebrate, and overreact to ordinary arithmetic.
Use numerical memes and comments tied to calculations. Most interactions remain
quiet; the catalog and cooldowns determine when a reaction appears.

## Separation from mathematics

Fun rules consume trusted outcomes and engine facts. They never change:

- input, mathematical interpretation, or canonical result;
- the distinction between a real error and a success;
- personal-history ownership or publication consent.

Use an authored catalog with stable rule and achievement IDs.

The service determines durable awards and collective eligibility. The client
renders events and may choose among authored variants without changing the
conditions or awarding itself progress.

## Event identity and localization

System reactions carry stable IDs and safe parameters. The service's `funEvents`
contract supports deduplication and expiry.

Russian copy is required for an included reaction. If English localization is
included, comments, achievements, controls, and theatrical reveals are also
available in English.

Localize each joke's intent. Recognizable phrases such as “Nice.” may use the
same wording in both languages. Each catalog must include every enabled reaction.

Language changes, history reads, replays, and reconnects never earn or repeat an
award. A collective event is rendered in each viewer's own language.

## Initial personal discovery catalog

The following optional rules define the reaction catalog. Wording may be refined
without changing trigger conditions.

| Stable ID | Condition | Intended reaction |
|---|---|---|
| `answer_found` | A successful canonical result is exactly `42` | The answer is known; the question remains unresolved |
| `six_seven` | A successful canonical result is exactly `67` | A brief “Сикс-севен!” (“Six seven!” in English) |
| `nice_number` | A successful result is exactly `69` | A brief, knowing “Nice.” |
| `result_found` | A successful result is exactly `404` | The result was found, contrary to expectations |
| `peer_review` | Three consecutive successful deliberate calculations have identical source and angle settings | Excessively thorough checking of arithmetic |
| `bracket_architect` | A successful expression has at least six levels of syntactic nesting | Architectural overachievement with parentheses |
| `scientific_method` | One successful expression uses at least three distinct required named functions | The calculator acknowledges actual scientific ambition |
| `touch_grass` | The identity reaches 25 accepted calculations | A friendly intervention about excessive calculator use |

These eight rules can supply personal achievements with localized names and
descriptions. An award is earned once per identity and survives ordinary
restarts. Eligibility uses actual values and parsed facts, not rounded display
strings or arbitrary substring matches.

The `six_seven` discovery is required for the planned Sprint 0 showcase.
Rehearse it with `60+7`. Inputs such as `167`, `67/0`, and `6*7` do not qualify,
nor does a different value whose display rounds to `67`. Use the existing
comment, achievement, and cooldown rules for this discovery.

The repeated-calculation rule counts deliberate actions, not transport retries.
The usage count includes accepted mathematical errors but excludes rejected
requests, history reads, and playback.

The touch-grass gag adds comments at 50 and 100 accepted calculations without
additional awards. These thresholds live in the personal discovery service's
configuration; the event carries the accepted count. Retries and collection
reads never emit a milestone comment.

### Backend rule interface

`discovery.New(discovery.DefaultConfig())` selects the eight rules and their
default thresholds. Configuration can change thresholds or select fewer rules.
`Catalog()` returns independent copies of their Russian/English names,
descriptions, and comments.

`Rules.Match(Input)` returns eligible IDs without granting awards or changing
state. Supply the current accepted record, its owner's unique accepted count
through that record, and the immediately preceding accepted records, newest
first. Exclude transport retries, rejected requests, reads, and later actions
from the snapshot. Evaluating the same snapshot does not advance progress.

## One theatrical incident

An included catastrophic gag is a brief, explicitly comedic in-application
incident, such as a fake calculator crash or blue-screen-style panel.

The initial deterministic trigger is three deliberate division-by-zero outcomes
within 60 seconds for the same identity, subject to the strong-effect cooldown.
It is a personal effect; one person must not force every room viewer into it.

Eligibility uses owned committed actions. Their timestamps may differ from
commit order; the three actions must fit one sixty-second interval. Actions
committed after the triggering record cannot qualify it retroactively.

Requirements:

- The real division-by-zero error remains available and understandable.
- The source, result/error state, and history are not destroyed.
- The effect can be dismissed immediately, including with Escape.
- It ends automatically within three seconds.
- A localized comedic reveal makes its nature clear.
- It does not invoke device fullscreen, imitate credential/security warnings,
  trap navigation, or interfere with operating-system controls.

Render the incident in the client while the service continues normally.

`ACHIEVEMENTS_ENABLED=false` disables personal awards, comments, and this scene.
The service grants awards after the core calculation commits; session bootstrap
quietly repairs missing awards from owned history. Retries never announce them.
Scene events expire after ten seconds; the client drops events already three
seconds old and shows a fresh scene for at most three seconds.

## Collective candidates

If rooms and collective reactions are included:

| Stable ID | Condition | Intended payoff |
|---|---|---|
| `shared_answer` | At least three distinct participants publish a successful result of `42` within 60 seconds | A brief “mathematical consensus” scene shared by the room |
| `peer_reviewed` | A public calculation has active reactions from at least three distinct participants other than its author | A comic peer-review badge; the numerical answer does not change |

`peer_reviewed` requires emoji reactions. `shared_answer` uses calculation events.

Room award/scene identity must prevent duplicate announcements after reconnect.
Room-level state may reset with an ephemeral room; personal awards remain
persistent. The room-wide strong-effect cooldown applies even if many
participants satisfy a condition simultaneously.

## Comedy pacing

Apply one shared presentation policy:

- No more than one prominent effect or achievement announcement at a time.
- Ordinary comments and achievement announcements share a per-view cooldown
  of at least 15 seconds.
- Large personal scenes have a per-identity cooldown of at least 120 seconds.
- Large collective scenes have a room-wide cooldown of at least 120 seconds.
- Strong eligibility and room-wide cooldowns are enforced by the service.
- Burst events are coalesced or suppressed, not queued for later sequential
  interruption.
- An earned achievement may appear quietly in the collection even when its
  announcement is suppressed.
- Reaction counter updates do not consume a full notification per update.

An actively requested reduction playback is not interrupted by a large gag.
A scene that cannot be shown while it is still relevant is dropped, not replayed
several calculations later.

Enforce input and request-rate limits separately from effect cooldowns.

## Settings and rehearsal

Provide the ability to disable humor and, independently, large theatrical
effects. These presentation settings do not alter mathematics or achievement
eligibility when the achievement feature is enabled. Earned progress remains
available without requiring its announcements to be shown.

Keep thresholds and catalog selection in configuration. Use the defaults above
unless they are updated in the catalog.

Use known expressions and, when needed, fresh anonymous contexts for rehearsal.
Automated checks control the clock; the public demo exercises real calculation,
history, and publication.

## Acceptance

Verify actual triggers from different categories, persistent one-time awards,
effect expiry, deduplication, and cooldown behavior under bursts.

Show that humor can be disabled without changing a result or error, and that
the theatrical incident always restores the interface. With localization,
exercise an earned achievement and a collective event in both languages.
