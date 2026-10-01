# Fun & Chaos

> Scope: Optional showcase, central to the calculator's intended character.

## Tone

The calculator may tease, roast, celebrate, and overreact to ordinary arithmetic.
Use numerical memes and comments tied to calculations. Ordinary personality is
frequent when space and pacing allow; large theatrical effects remain occasional.
Tease the expression or situation, not the person.

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

## Personal discovery catalog

The server supplies 22 definitions, including ten secrets, in the order below.
Every definition has a Russian and English name, description, and congratulatory
comment, plus original pixel artwork. This table defines conditions and secrecy.
Wording may be refined without changing trigger conditions.

| Stable ID | Russian name | English name | Condition | `secret` |
|---|---|---|---|---|
| `answer_found` | Главный вопрос | The Big Question | Successful canonical result exactly `42` | `true` |
| `six_seven` | Мем года | Meme of the Year | Successful canonical result exactly `67` | `true` |
| `nice_number` | Тонкий намёк | A Subtle Hint | Successful canonical result exactly `69` | `true` |
| `result_found` | Потерянный сигнал | Lost Signal | Successful canonical result exactly `404` | `true` |
| `peer_review` | Всё под контролем | Under Control | Three consecutive successful deliberate calculations with identical source and mathematical context | `false` |
| `bracket_architect` | Внутренний мир | Inner World | Successful expression with at least six levels of syntactic nesting | `false` |
| `scientific_method` | Исследователь | Explorer | One successful expression uses at least three distinct required named functions | `false` |
| `touch_grass` | Снаружи тоже жизнь | Life Outside | The identity reaches 25 accepted calculations, including mathematical errors | `false` |
| `second_wind` | Второе дыхание | Second Wind | The next accepted calculation after a mathematical error succeeds | `false` |
| `alternate_routes` | Другой маршрут | Another Route | The same canonical result through three distinct parsed operator/function structures | `false` |
| `quiet_after_storm` | Тишина после бури | Quiet After the Storm | Successful canonical zero with at least eight operations | `true` |
| `paper_tiger` | Бумажный тигр | Paper Tiger | Successful canonical one using at least three distinct required named functions | `true` |
| `gaining_altitude` | Набираем высоту | Gaining Altitude | Five consecutive successful numeric results, each strictly greater than the previous result | `false` |
| `mirror_room` | Зеркальная комната | Mirror Room | Three consecutive successes `x`, `-x`, `x`, where `x` is nonzero | `true` |
| `trouble_collector` | Коллекционер неприятностей | Trouble Collector | Accepted syntax, division-by-zero, and domain errors have all occurred | `false` |
| `unscathed` | Без единой царапины | Unscathed | Ten consecutive accepted successes; a mathematical error breaks the streak | `false` |
| `grand_scale` | Большой размах | Grand Scale | Successful result with absolute value strictly greater than `1e12` | `false` |
| `last_pixel` | Последний пиксель | The Last Pixel | Successful result with `0 < abs(value) < 1e-12` | `false` |
| `parallel_worlds` | Параллельные миры | Parallel Worlds | The same canonical result from genuine direct trigonometric calculations with and without degree notation | `true` |
| `time_loop` | Петля времени | Time Loop | Repeat a normalized successful expression at least seven days after its earliest stored success | `false` |
| `fourth_wall` | Четвёртая стена | The Fourth Wall | The whole trimmed, case-insensitive expression is `привет` or `hello`; its mathematical error remains unchanged | `true` |
| `unexpected_tail` | Незваный хвост | An Unexpected Tail | A computation with at least one operation produces canonical `0.30000000000000004`; a bare literal does not qualify | `true` |

An award is earned once per identity and survives ordinary restarts. Eligibility
uses canonical values and trusted parsed facts, never rounded display strings.
Retries and rejected requests neither advance progress nor break streaks.
Accepted mathematical errors break success streaks; they still count toward
usage and the recorded error categories.

Required named functions are `sqrt`, `abs`, `exp`, `ln`, `log`, `sin`, `cos`,
`tan`, `asin`, `acos`, and `atan`; extension `mod` does not count toward the
three-function rules. For example, `sqrt(16)/abs(-4)+ln(1)` qualifies for
`paper_tiger`. `2+2`, `2*2`, and `2^2` supply three routes to `4`; whitespace,
redundant grouping, and changing only numeric leaves do not supply a new
operator/function structure. `sin(90°)` and `sin(pi/2)` illustrate
`parallel_worlds`: degree conversion must occur in a direct `sin`, `cos`, or
`tan` argument, not elsewhere in an unrelated expression.

Names and icons stay visible in the collection. A locked secret's condition is
not rendered in its description, tooltip, or accessible label. All locked cards
use the generic “Not earned” status, without a “Secret” badge or a verbose
concealment paragraph. Earned cards reveal the description and date. Keep the
earned counter and a flat grid ordered as earned, ordinary locked, then secret
locked, preserving catalog order within each category without group headings.
The collection window, cards, and ceremony use rounded opaque surfaces and
Monocraft for all copy, scoped separately from the main console.
Earned cards use an award-accent gradient, bright solid border, and unfiltered
color artwork. Locked cards use a darker page-background canvas, muted dashed
border, and grayscale artwork at 55% brightness. Dim the artwork, not the whole
card: names, status, ordinary conditions, and concealed text remain readable in
both violet and amber themes.

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
uses a 280ms final-size unmask with elastic settling. Header, close control,
and focus are available immediately; reduced motion or effects-off skips it.
Closing releases the native modal and restores focus immediately; a 190ms
inert, accessibility-hidden, click-through snapshot retracts the painted frame
when motion and effects allow. It runs no cipher or decode work.
Only the inner body scrolls, while outer `overflow: clip` keeps the header fixed
even during programmatic card scrolling.

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

`discovery.New(discovery.DefaultConfig())` selects all 22 rules and their
default thresholds. Configuration can change thresholds or select fewer rules.
`Catalog()` returns independent copies of their required secret flags and
Russian/English names, descriptions, and comments.

`Rules.Match(Input)` returns eligible IDs without granting awards or changing
state. `Input` contains `Calculation`, the owner's `AcceptedCount` through that
record, prior `State`, and indexed `Evidence`. Match before calling
`State.Advance(record)` exactly once for a new accepted action. State keeps
bounded streaks, recent numeric values, source-repeat progress, recovery, and
seen error categories; it does not retain a previous-history window.

`Evidence` contains at most three parser structure identities for the same
canonical result, direct-trig variant bits, and the earliest success timestamp
for the normalized expression. Scope indexes by owner and semantics version.
Exclude retries, rejected requests, reads, and later actions from the snapshot.
Evaluating the same snapshot does not advance progress. Persist state, evidence,
checkpoint, and grants together as specified in
[History & Statistics](history-and-statistics.md#achievements-and-joke-statistics).

## Ordinary personality

`web/src/features/discoveries/phraseCatalog.ts` contains exactly 200 individually
authored entries, each with a stable ID, context tags, and Russian/English text.
It is an offline catalog, not a model call or a template expanded per request.
Its 16 contexts cover ordinary and complex expressions, functions, nesting,
repeats, zero, negative, huge and tiny results, mathematical errors, recovery,
and occasional philosophy or hype. All seven protocol mathematical error codes
have a truthful context: `SYNTAX_ERROR` selects `syntax`, `DIVISION_BY_ZERO`
selects `division_zero`, and `DOMAIN_ERROR` selects `domain`.
`UNKNOWN_IDENTIFIER`, `WRONG_ARITY`, `NUMERIC_OVERFLOW`, and
`UNSUPPORTED_FEATURE` select the shared `math_error` pool of 12 entries.
`UNSUPPORTED_FEATURE` is declared by the protocol but not currently emitted by
the engine. Generic error comments make no syntax, domain, result-size, or
successful-result claims. Existing contexts retain at least ten entries each;
philosophy has 19. Select from actual outcomes and facts; do not claim
complexity, trigonometry, or past activity without evidence.

Observe each newest-visible accepted calculation once, even when a quiet retry
or programmatic submission cannot speak. Observation updates error/recovery,
successful-expression repeat context, and calculation deduplication independently
of speech permission. Bootstrap and history reads do not supply observations;
superseded responses do not replace the visible outcome. New submissions,
accepted outcomes, transport failures, and quiet retries preserve an active
bubble until its original 4.5-second deadline. They neither replace its text
nor extend its timer; outcomes observed during speech are not queued to speak
later. The face continues to reflect the current calculation.

Speech requires a deliberate submission timestamp captured in the browser
before awaiting session bootstrap or the calculation POST. Using the same
browser clock, require a finite, nonfuture timestamp less than 15 seconds old
and strictly after the most recent return to visibility. Server calculation
creation time does not establish this freshness. The initial foreground epoch
is unbounded in the past so a first submission awaiting bootstrap remains
eligible. Hidden tabs discard speech; a delayed pre-resume response cannot
speak on return. Consumed quiet, stale, or suppressed outcomes are not replayed.

Keep Russian grammatical and natural, including short jokes. Favor brief
comments; philosophy and hype are rare. Teasing must avoid personal insults,
profanity, and hostility toward protected groups. Suitable Russian examples
include «Получился ноль. Это тоже результат.» and
«Число найдено. Смысла пока нет.».

The pixel face responds to real loading, success, error, and recovery states;
it never adds a fake computation wait. Between reactions, it gently hovers
within its reserved safe bounds, blinks occasionally (including a double blink),
and briefly glances down-right toward the editor. Computation and result
reactions take priority over idle motion, which resumes when the reaction ends.
Idle motion is silent and stops with effects-off or reduced motion. Hidden,
offscreen, unsafe, or priority-suppressed faces run no idle loops.
Speech emerges from a console edge,
squashes and stretches into place, then retracts with its tail. Reduced motion
and effects-off use simple feedback. Stop nonessential activity in hidden tabs.

Choose positions randomly from measured safe console-edge slots, avoiding the
last slot when another is available. Protect the editor, result/error content,
submit control, header, toolbar, open tools, notices, and priority surfaces.
Check the full animated bubble envelope and actual tail clearance, not just
the final text rectangle. Keep both inside the visible viewport, using visual
viewport bounds when a software keyboard reduces it. Decorative space may be
used on phones, but critical content may not be covered. Re-measure after
resize, scroll, and panel changes; reposition or dismiss if safety changes.
When no safe slot exists, skip speech.

Personality is identity-scoped and follows the humor preference. Speech is
click-through, does not steal focus, and is not a stream of automatic
screen-reader announcements. The real outcome remains the accessible status.

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
- An eligible deliberate, newest-visible accepted outcome has a 70% chance
  of ordinary speech. Starts are at least five seconds apart, and a bubble
  stays for 4.5 seconds before retracting. Show one bubble at a time, with no
  queue for blocked, stale, or burst responses.
- Remember the last 32 catalog phrase IDs and recent contexts to avoid repeats.
  Skip an exhausted eligible pool rather than forcing a repeat.
- Catalog commentary and server discovery comments share one personality
  surface. Active or pending awards, modals, comic scenes, and hidden tabs
  suppress it; discarded comments never return after the interruption.
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
- Scenes are suppressed rather than queued for later interruption. A ceremony,
  including awards pending catalog data, takes precedence over ordinary
  comments and comic scenes.
- Reaction counter updates do not consume a full notification per update.

An actively requested reduction playback is not interrupted by a large gag.
A scene that cannot be shown while it is still relevant is dropped, not replayed
several calculations later.

Enforce input and request-rate limits separately from effect cooldowns.

## Settings and rehearsal

Provide independent humor, special-effects, and sound preferences.
`Preferences.largeEffects` has the settings label `«Спецэффекты»`;
its caption is «Живые анимации, праздник достижений и шуточные сцены.».
`Preferences.soundEnabled` defaults to `true` and is persisted by the header
speaker control, whose muted state shows a slash. These settings do not alter
mathematics or award eligibility. A basic earned notice is still shown when
humor or special effects are off.

The award cue is an original short five-step Web Audio synthesis. Activate
audio only from a trusted user gesture; playback before interaction is not
guaranteed. A new intentional button, Enter, or keypad calculation dispatch
plays a 65ms triangle tick falling from 440Hz to 330Hz in the same audio context.
This is dispatch feedback, not a success sound. Typing, empty input,
Shift+Enter, IME composition, retries, hover, focus, disabled controls, and
synthetic/programmatic activation remain silent.

Deliberate collection/tool browsing and controls have differentiated
cues: rising navigation open, falling close, a short editing/keypad/copy tick,
and a toggle motif. Trusted root click/change/Enter guards cover pointer and
keyboard activation without duplicating the submit cue or checkbox toggle.
All cues use four times the preceding signal amplitude (about +12dB).
Output gains are `.8` for submit, `.448` for open/close/toggle, `.384` for edit,
and `.9` for the award. Keep the fanfare stronger than ordinary cues without clipping.
Validate native waveform peak and 10ms RMS levels, including overlapping fanfare
voices; output-gain ordering alone does not prove the audible hierarchy.
All cues use one lazy shared audio context. Ordinary bursts replace or drop
the current cue; they never queue late sound. An award preempts ordinary audio,
and active or pending ceremonies suppress both button and submit cues.
Persistent mute stops all voices immediately; disposal releases voices and
the shared context. OS reduced motion is a separate visual constraint, not a
sound or humor toggle.

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
