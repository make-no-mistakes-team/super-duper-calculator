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
action and optional `freshAwardIds` from the celebration hook. The root keys
the collection by `identity`, so description state cannot cross owners. It
scrolls to the selected earned card without replaying effects. Close and Escape
restore the opener or editor; Escape does not also dismiss the tool bay.
Loading/unavailable states use the root's refresh callback. The opaque window
has a 16px radius and opaque cards have a 12px radius. Only the inner body
scrolls: the outer dialog uses `overflow: clip` to prevent focus or selected-card
scrolling from displacing the header. Its 300ms unmask preserves final geometry
and immediately available header, close control, and focus. Reduced motion
opens instantly.

The server supplies localized names and required `DiscoveryDefinition.secret`;
do not duplicate that registry in the client. Show names and icons for every
card, keep the earned counter, and render one flat grid ordered as earned,
ordinary locked, then secret locked, preserving catalog order within each
category. Use two desktop columns and one phone column without group headings.
Locked cards use “Не получено”, with no “Secret” badge or verbose concealment
paragraph. Locked secret conditions never enter rendered text, tooltips, or
accessible labels.

`useAchievementCelebration` owns the award queue, deduplication, ceremony view,
and audio. Its options are `identity`, `catalog`, `soundEnabled`, `effectsEnabled`,
and `onOpenCollection`; it returns `enqueue`, `view`, `unlockAudio`,
`playSubmitSound`, `freshAwardIds`, and `active`.
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
click-intercepting spectacle. `active` suppresses competing ordinary notices
and scenes. Comments retain the 15-second newest-only cooldown; scenes retain
their separate 120-second durable cooldown. Counts 50 and 100 emit touch-grass
comments rather than new awards.

`Preferences.soundEnabled` defaults to `true` and persists with the other
browser preferences. The header speaker has an accessible action label and a
slash when muted. Trusted root pointer/keyboard capture calls `unlockAudio`
synchronously before asynchronous work. The hook owns one `AchievementAudio`
from `src/features/discoveries/achievementAudio.ts` and one shared Web Audio
context for the original five-step award cue and the quiet submit tick.
`App` calls `playSubmitSound` only in the intentional `CalculatorInput.onSubmit`
handler, before dispatch; the generic `submit` path and retry handlers never
play it. Button, Enter, and keypad dispatches produce a 65ms triangle voice
starting at 440Hz and falling to 330Hz. This signals dispatch, not success.
Typing, empty input, Shift+Enter, IME composition, transport retries, and
collection browsing stay quiet.

A new tick preempts the previous submit voice instead of queuing it. The award
cue stops submit audio and suppresses ticks while playing. `stop()` and
`dispose()` release both voice sets; mute stops both immediately, and disposal
closes the shared context. Audio before a gesture is not guaranteed.
`effectsEnabled` comes from `Preferences.largeEffects`, whose label is
`«Спецэффекты»`, independently of sound and humor. A basic earned notice
survives humor/effects being off; OS reduced motion separately limits motion.

## Artwork

The eight original transparent 64×64 PNGs live in
`web/public/achievements/{id}.png`, served as `/achievements/{id}.png`.
Editable ASE sources live in `art/achievements/{id}.ase`. Display at integer
scale, using `image-rendering: pixelated`; the main collection artwork is 128px.
`art/achievements/generate.py` reproduces the sprites with the pixel-art-python/
Pillow workflow and exports editable sources through LibreSprite. Keep IDs
aligned with the server catalog rather than substituting placeholder artwork.
