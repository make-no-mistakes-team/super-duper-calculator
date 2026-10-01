# Frontend integration contract

`App.tsx` owns the editable expression, last submitted outcome, and
personal history. `CalculatorInput` receives values and event handlers through
props. `History` receives records, paging state, and callbacks; selecting a
record restores only the unchanged source in the editor. It does not submit or replace
the last submitted outcome.

`App` also owns the active tool selection. `useToolController` shares opening,
toggle, dismissal, and focus behavior across the header settings and tool bay.
Escape ignores IME composition and leaves an active comic scene to handle
dismissal first. The controller restores the tool trigger on close, the editor
on history reuse or correction, and the submit button when a focused phone sheet
closes for submission. `CalculatorInput` renders the exclusive Functions/Keypad/
History/Statistics bay, plus header-selected Settings, docks it on desktop, and
presents it as a phone sheet. History contains records only; Statistics shows
three top counts and expandable metric details.

`CalculatorInput` receives `effectsEnabled` and a `consoleRef` for personality
measurement. Active-tool dismissal and focus restoration happen immediately;
its outgoing panel is retained only as an inert, accessibility-hidden,
click-through 190ms visual frame. Effects-off and reduced motion remove it
immediately. Desktop disclosure is lateral; the phone sheet rises from below.
Buttons use press compression and elastic rebound without delaying handlers.

Calculation presentation lives under `src/features/calculation`.
`presentation.ts` formats values, mathematical errors, request failures, and
outcome announcements without owning state or issuing requests. `ResultView`
renders the submitted outcome and reports copy, retry, and correction actions
to `App`. Clipboard writes and their race guards remain in the root; display
rounding never changes the exact value copied or stored.
“Go to error” is available only for a known span in unchanged source; after an
edit the action is “Restore expression”. Error spans and editor selection use
half-open UTF-16 code-unit offsets.

All HTTP requests go through `src/api.ts`. Its relative `/api/...` paths keep
the browser on the page origin and send the anonymous session cookie. Go owns
calculation, durable records, and capability flags. Feature components should
consume `src/contracts.ts` types, receive server data from the root, and report
user actions upward. They should not add another evaluator or API client.

Calculation requests require `requestId` and `expression`; optional room context
remains explicit. Mathematical context contains only `semanticsVersion`,
currently `binary64-v2`. Trigonometry and inverse outputs use radians, while
required postfix `°` converts `x*pi/180`. The keypad inserts `°`; capabilities
advertise it among the operators. There is no angle-mode state, metadata, or
restoration.

Optional `CalculationFacts.normalizedExpression`, `structureIdentity`, and
`trigWithDegrees` are trusted parser metadata, not client-generated source
matches. Their meaning and durable evidence contract are in
[Application Service](../specs/application-service.md#calculation-record-and-response).
Keep older records readable when those fields are absent; do not reparse them
to create discovery eligibility.

The root loads capabilities once and passes them to controls and history.
History keeps stored outcomes readable while warning when an old expression
uses a disabled extension. A failed capability request leaves the core keypad
available and shows a warning.
Optional controls should mount only after their complete server behavior is
enabled and integrated. For rooms, keep the current room code and publication
choice in `App` state for that tab. Supply room context explicitly on each new
calculation request; never infer it from the shared session cookie or storage.

Core labels and mathematical error messages are Russian. Shared outcome,
error, result, and request messages live in `messages.calculation` under
`src/i18n`; `messages.history` covers history title, loading, empty, and paging
states. Both catalogs cover these types, but the site has no language selector.
Each optional feature should supply a complete catalog before exposing one.

## Typography

`src/fonts.css` defines local Jura body fonts and Monocraft achievement fonts.
Ordinary copy uses `--body` (Jura), main controls use `--controls` (Tiny5), and
expressions, results, and mathematical syntax use `--mono` (PT Mono).
Collection and ceremony copy consistently use `--achievements` (Monocraft):
headings, names, descriptions, status, counts, dates, and controls. Keep their
rounded opaque surfaces scoped to achievements; the main console retains its
own styling. Function examples remain in the Functions bay. The comma control
uses the short accessible label “Запятая” without an argument explanation or
tooltip.

Jura is pinned to `@fontsource/jura` 5.3.0, with local upright 500/600/700 Latin
and Cyrillic subsets. Monocraft comes from the official v4.2.1 release's
Regular 400 and Bold 700 files, converted to WOFF2 without subsetting or outline
changes. Both retain the complete source character map, including all 66
Russian letters. `src/fonts.css` records the release source, commit, archive
checksum, conversion tools, and copyright. `public/fonts/Jura-LICENSE.txt`
and `public/fonts/Monocraft-LICENSE.txt` contain the complete SIL OFL 1.1
licenses. Font files and licenses are served locally.

## Collection and ceremony

`AchievementCollection` is an independently controlled native dialog opened
from the header, with optional `selectedId` for the ceremony's collection
action, optional `freshAwardIds` from the celebration hook, and `effectsEnabled`.
The root keys the collection by `identity`, so description state cannot cross owners. It
scrolls to the selected earned card without replaying effects. Close and Escape
restore the opener or editor; Escape does not also dismiss the tool bay.
Loading/unavailable states use the root's refresh callback. The opaque window
has a 16px radius and opaque cards have a 12px radius. Only the inner body
scrolls: the outer dialog uses `overflow: clip` to prevent focus or selected-card
scrolling from displacing the header. Its 280ms unmask and elastic settling
preserve final layout geometry and immediately available header, close control,
and focus. Cards disclose with staggered icon squash/stretch. Reduced motion
or effects-off opens instantly.

`exitSnapshot.ts` captures only the last painted collection or ceremony frame.
The real dialog/state releases immediately, including opener focus restoration.
The 190ms snapshot is inert, `aria-hidden`, and click-through; strip duplicate
IDs, live award identifiers, and interactive semantics. It starts from the
current painted transform even if closing interrupts entrance, preserves
scroll positions, and runs no cipher/decode renderer. Clear it on replacement,
resize, tab hiding, reduced-motion changes, or unmount.

The server supplies localized names and required `DiscoveryDefinition.secret`;
do not duplicate that registry in the client. Show names and icons for every
card, keep the earned counter, and render one flat grid ordered as earned,
ordinary locked, then secret locked, preserving catalog order within each
category. Use two desktop columns and one phone column without group headings.
Locked cards use “Не получено”, with no “Secret” badge or verbose concealment
paragraph. Locked secret conditions never enter rendered text, tooltips, or
accessible labels.
Earned cards have an award-accent gradient, bright solid border, and unfiltered
color artwork; locked cards have a dark page-background canvas, muted dashed
border, and grayscale artwork at 55% brightness. Apply dimming only to artwork,
keeping names, status, descriptions, and concealed text readable in both themes.

`useAchievementCelebration` owns the award queue, deduplication, ceremony view,
and audio. Its options are `identity`, `catalog`, `soundEnabled`, `effectsEnabled`,
and `onOpenCollection`; it returns `enqueue`, `view`, `unlockAudio`,
`playSubmitSound`, `playButtonSound`, `freshAwardIds`, and `active`.
The root renders `view` outside the tool bay and result. Every mounted,
same-session-generation POST response merges and enqueues `achievements`
outside the newest-visible-result guard. `funEvents` remain ordinary comments
and scenes. New submissions do not clear awards, and awards awaiting catalog
data are retained. Identity replacement resets the queue and seen IDs;
bootstrap, history, refresh, retry, and reload never replay old ceremonies.

`freshAwardIds` is a cumulative snapshot of the existing queue's `seen` IDs for
the current identity, not a separate transient event feed. Pass it to the
identity-keyed collection. Shared `AchievementDescription` owns cipher and
decode state in both collection cards and ceremonies. Collection props provide
`text`, `locked`, `concealed`, `active`, the inner body's `rootRef`,
`freshAwardIds`, `awardId`, and `className`; locked secret `text` is `null`.
The ceremony passes its earned text with `freshCeremony` set only for secret
awards.

The concealed description is a fixed two-line ASCII pattern independent of the
real condition or its length, with two glyph changes every 250ms. Run it only
when visible, open/active, foregrounded, and motion-allowed; stop while closed,
offscreen, hidden, or under reduced motion. It is decorative fake cipher text,
not encryption. Hide visual frames from assistive technology and expose a
static concealed-condition label.

Decode over 600ms only for an actual fresh secret grant: its new ceremony, or
a collection card that was visibly locked and concealed before becoming earned
while the same collection remains open, with its ID in `freshAwardIds`.
Consume grants even when closed or offscreen so later browsing cannot replay
them. Bootstrap, quiet repair, refresh, reopen, reload, and owner replacement
show earned text directly; ordinary awards never decode. Decode motion runs
only while visible and foregrounded; closing or reduced motion completes it
immediately. Accessible earned text is the real description, not changing
glyphs.

Each ceremony has a 1.25-second burst and a 7.5-second readable card, with
hover/focus/background pause, no focus theft, and no typing blockage or
click-intercepting spectacle. `active` includes pending awards, even while
catalog data is unavailable, and suppresses ordinary personality and scenes.
Scenes retain their separate 120-second durable cooldown. Counts 50 and 100
emit touch-grass comments rather than new awards.

`CalculatorPersonality` replaces the separate discovery notice. Props are
`identity`, newest-visible accepted `calculation`, `speechEligible`,
`submittedAt`, `events`, `catalog`, `enabled`, `effectsEnabled`, `suppressed`,
`loading`, and `anchorRef`. The root passes every newest accepted record,
including quiet retries and programmatic submissions, for once-per-ID
observation. This updates recovery and successful-expression repeat context
even when `speechEligible` is false. Bootstrap, history reads, and superseded
responses do not enter this path.

`App.submit` captures `submittedAt` with `Date.now()` before awaiting session
bootstrap or the POST, and sets `speechEligible` from deliberate submission
intent. Personality checks the same browser clock: the timestamp must be finite,
not in the future, less than 15 seconds old, and strictly after the latest
return to visibility. Do not use the Go record's `createdAt` as request freshness.
The initial foreground epoch is `-Infinity`, allowing a first submission that
awaits bootstrap. Loading, replacement accepted records, transport failures,
and quiet retries preserve active speech and its original expiry. Observe new
outcomes once, but do not replace the bubble, restart its timer, or queue speech.
Visibility changes clear speech and reaction state; quiet, stale, or suppressed
records are consumed without replay on resume or later prop changes. Server
comment events also retain their own timestamp, expiry, and deduplication checks.

`phraseCatalog.ts` supplies exactly 200 stable-ID/context-tagged authored RU/EN
entries across 16 contexts; the current Russian UI uses `ru`. All seven
mathematical error codes select truthful pools, including shared `math_error`
for identifier, arity, overflow, and protocol-declared unsupported errors.
The authoritative mapping and copy constraints are in
[Fun & Chaos](../specs/fun-and-chaos.md#ordinary-personality).
Context selection follows outcome/facts, with 32 recent phrase IDs and recent
context avoidance; skip an exhausted eligible pool. Eligible speech has a 70%
chance, a five-second minimum start gap, and 4.5-second display before
retraction. There is one bubble, never a deferred comment queue.

Personality is identity-keyed, humor-controlled, foreground-only, and suppressed
by active/pending awards, modals, or comic scenes. Its pixel face reflects real
loading, success, error, and recovery. Idle SVG motion uses CSS only: a 6.4-second
hover with a 3px lift and gentle tilt, sparse blinking on a 17.3-second cycle,
and a brief down-right glance on a separate 19.1-second cycle. Nested eye-lid
and eye groups keep blinking and glancing independent. Idle selectors require
`data-mood="idle"` and `data-minimal="false"`; computation reactions override
them. Effects-off and reduced motion cancel all idle animation. Existing
foreground, priority, offscreen, and safe-placement removal cancels the loops
without JavaScript scheduling. The reserved face envelope also contains hover.
The portal is click-through and does not
move focus or announce every joke as live status. Effects-off/reduced motion
simplify feedback. `data-speech-protected` marks critical regions: measure
console-edge candidates against editor, outcome, submit, header, toolbar,
open tools, notices, priority surfaces, and visual viewport bounds. Randomize
among safe slots, avoid the previous slot when possible, include bubble motion
envelopes and actual tail clearance, and skip when none fit. Resize, scrolling,
panel mutations, and protected-surface animation trigger remeasurement.
Full conditions and copy requirements are in
[Fun & Chaos](../specs/fun-and-chaos.md#ordinary-personality).

`Preferences.soundEnabled` defaults to `true` and persists with the other
browser preferences. The header speaker has an accessible action label and a
slash when muted. Trusted root pointer/keyboard capture calls `unlockAudio`
synchronously before asynchronous work. The hook owns one `AchievementAudio`
from `src/features/discoveries/achievementAudio.ts` and one shared Web Audio
context for the original five-step award cue and replaceable ordinary cues.
`CalculatorInput.onSubmit(trusted)` derives trust from the actual button/Enter
event. `App` calls `playSubmitSound` only when it is trusted, before dispatch;
the generic `submit` path and retry handlers never play it. Button, Enter,
and keypad dispatches produce a 65ms triangle voice starting at 440Hz and
falling to 330Hz. This signals dispatch, not success.
Typing, empty input, Shift+Enter, IME composition, transport retries, hover,
focus, disabled controls, and synthetic/programmatic activation stay quiet.

`ButtonCue` is `'open' | 'close' | 'edit' | 'toggle'`; `AchievementAudio.playButton`
and the hook's `playButtonSound` use differentiated motifs. Root trusted
click capture reads `data-button-cue`, with `panel` using `aria-expanded` and
`none` suppressing a duplicate submit cue. Trusted checkbox/radio change capture
uses `toggle`; a handled trusted Escape uses `close`. Summary disclosure uses
open/close. One deliberate activation produces one cue, including keyboard
activation; native typing and programmatic retries never do.

All signal amplitudes are four times their preceding levels (about +12dB).
Current output gains are `.8` for submit, `.448` for open/close/toggle,
`.384` for edit, and `.9` for the award. Envelopes and waveforms differ:
verify native waveform peak and 10ms RMS, fanfare strength, and clipping
headroom rather than comparing output gains alone.

A new ordinary cue replaces the previous ordinary voice instead of queuing it.
The award cue stops ordinary audio; active or pending ceremonies suppress
button and submit cues. `stop()` and `dispose()` release both voice sets;
mute stops all immediately, and disposal closes the shared context.
Audio before a gesture is not guaranteed.
`effectsEnabled` comes from `Preferences.largeEffects`, whose label is
`«Спецэффекты»` and caption is
«Живые анимации, праздник достижений и шуточные сцены.».
It remains independent of sound and humor. A basic earned notice
survives humor/effects being off; OS reduced motion separately limits motion.

## Artwork

The 22 original transparent 64×64 PNGs live in
`web/public/achievements/{id}.png`, served as `/achievements/{id}.png`.
Editable ASE sources live in `art/achievements/{id}.ase`. Display at integer
scale, using `image-rendering: pixelated`; the main collection artwork is 128px.
`art/achievements/generate.py` reproduces the sprites with the pixel-art-python/
Pillow workflow and exports editable sources through LibreSprite. Its optional
`--contact-sheet` lays out the whole generated catalog with four columns and
a row count derived from the number of sprites, showing native 64px and nearest
128px views rather than assuming eight icons. Keep IDs aligned with the
[server catalog](../specs/fun-and-chaos.md#personal-discovery-catalog) rather
than substituting placeholder artwork.
