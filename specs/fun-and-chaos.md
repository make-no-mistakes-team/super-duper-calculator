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
| `peer_review` | Three consecutive successful deliberate calculations have identical source and mathematical context | Excessively thorough checking of arithmetic |
| `bracket_architect` | A successful expression has at least six levels of syntactic nesting | Architectural overachievement with parentheses |
| `scientific_method` | One successful expression uses at least three distinct required named functions | The calculator acknowledges actual scientific ambition |
| `touch_grass` | The identity reaches 25 accepted calculations | A friendly intervention about excessive calculator use |

These eight rules can supply personal achievements with localized names and
descriptions. An award is earned once per identity and survives ordinary
restarts. Eligibility uses actual values and parsed facts, not rounded display
strings or arbitrary substring matches.

The server catalog supplies the following names and required `secret` flags,
in this stable order:

| Stable ID | Russian name | English name | `secret` |
|---|---|---|---|
| `answer_found` | Главный вопрос | The Big Question | `true` |
| `six_seven` | Мем года | Meme of the Year | `true` |
| `nice_number` | Тонкий намёк | A Subtle Hint | `true` |
| `result_found` | Потерянный сигнал | Lost Signal | `true` |
| `peer_review` | Всё под контролем | Under Control | `false` |
| `bracket_architect` | Внутренний мир | Inner World | `false` |
| `scientific_method` | Исследователь | Explorer | `false` |
| `touch_grass` | Снаружи тоже жизнь | Life Outside | `false` |

Names and icons stay visible in the collection. A locked secret's condition is
not rendered in its description, tooltip, or accessible label. All locked cards
use the generic “Not earned” status, without a “Secret” badge or a verbose
concealment paragraph. Earned cards reveal the description and date. Keep the
earned counter and a flat grid ordered as earned, ordinary locked, then secret
locked, preserving catalog order within each category without group headings.
The collection window, cards, and ceremony use rounded opaque surfaces and
Monocraft for all copy, scoped separately from the main console.

Locked secret descriptions use one fixed two-line ASCII pattern independent
of every real condition and its length. Change two glyphs at 4Hz only when
visible in an open collection, in the foreground, with motion allowed; stop
when closed, offscreen, hidden, or under reduced motion. The decorative fake
cipher is not encryption. Assistive technology receives a static concealed
label, never the changing glyphs.

A 600ms progressive decode applies only to a fresh secret grant: its new
ceremony, or a previously visible locked secret card becoming earned while
the collection stays open, with its ID present in fresh POST awards. Ordinary
awards show their description directly. Bootstrap, quiet repair, collection
reads, reopening, reload, and identity replacement never trigger decoding.
Reduced motion shows the earned text immediately. Collection opening instead
uses a 300ms final-size unmask, with header, close control, and focus available
immediately; reduced motion skips it. Only the inner body scrolls, while outer
`overflow: clip` keeps the header fixed even during programmatic card scrolling.

The `six_seven` discovery is required for the planned Sprint 0 showcase.
Rehearse it with `60+7`. Inputs such as `167`, `67/0`, and `6*7` do not qualify,
nor does a different value whose display rounds to `67`. Its ordinary comment
follows comment pacing; its genuinely new award follows the ceremony queue.

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
`Catalog()` returns independent copies of their required secret flags and
Russian/English names, descriptions, and comments.

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

Separate new-award ceremonies from ordinary humor:

- No more than one prominent effect or achievement announcement at a time.
- Ordinary comments have a per-view cooldown of 15 seconds and use only the
  newest visible calculation response. Coalesce or suppress comment bursts.
- New-award ceremonies consume `POST /api/calculations` response `achievements`,
  not comment `funEvents`. Queue every valid same-generation response's awards,
  including responses too old to replace the visible result.
- Deduplicate awards by ID per identity and reset on identity replacement.
  New submissions never clear the queue. Bootstrap, history, collection reads,
  retries, and reload do not replay awards.
- Each award shows its original pixel-art icon, earned heading, name, and
  unlocked description. A 1.25-second visual burst precedes a 7.5-second readable
  card; hover, focus, and background tabs pause the readable interval.
- The ceremony does not steal focus, block typing, open a modal automatically,
  or intercept clicks behind its spectacle. Its collection action opens the
  matching earned card.
- Large personal scenes have a per-identity cooldown of at least 120 seconds.
- Large collective scenes have a room-wide cooldown of at least 120 seconds.
- Scene eligibility and durable cooldowns are enforced by the service.
- Scenes are suppressed rather than queued for later interruption. A ceremony
  takes precedence over ordinary comments and comic scenes.
- Reaction counter updates do not consume a full notification per update.

An actively requested reduction playback is not interrupted by a large gag.
A scene that cannot be shown while it is still relevant is dropped, not replayed
several calculations later.

Enforce input and request-rate limits separately from effect cooldowns.

## Settings and rehearsal

Provide independent humor, special-effects, and sound preferences.
`Preferences.largeEffects` has the settings label `«Спецэффекты»`;
`Preferences.soundEnabled` defaults to `true` and is persisted by the header
speaker control, whose muted state shows a slash. These settings do not alter
mathematics or award eligibility. A basic earned notice is still shown when
humor or special effects are off.

The award cue is an original short five-step Web Audio synthesis. Activate
audio only from a trusted user gesture; playback before interaction is not
guaranteed. A new intentional button, Enter, or keypad calculation dispatch
plays a quiet 65ms triangle tick starting at 440Hz in the same audio context.
This is dispatch feedback, not a success sound. Typing, empty input,
Shift+Enter, IME composition, retries, and collection browsing remain silent.
An active submit tick is replaced by the next dispatch, never queued. The
award cue takes priority, preempts a submit tick, and suppresses ticks while
playing. Persistent mute stops both; disposal releases both voice sets and the
shared context. OS reduced motion is a separate visual constraint, not a sound
or humor toggle.

Keep thresholds and catalog selection in configuration. Use the defaults above
unless they are updated in the catalog.

Use known expressions and, when needed, fresh anonymous contexts for rehearsal.
Automated checks control the clock; the public demo exercises real calculation,
history, and publication.

## Acceptance

Verify actual triggers from different categories, persistent one-time awards,
effect expiry, deduplication, comment/scene cooldown boundaries, and lossless
award queuing under bursts and out-of-order responses.

Show that humor can be disabled without changing a result or error, and that
the theatrical incident always restores the interface. With localization,
exercise an earned achievement and a collective event in both languages.
