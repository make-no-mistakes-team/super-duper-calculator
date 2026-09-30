# Client Experience

> Scope: Required Russian-first web client, with optional presentation features.

## Identity and visual direction

The console uses dark translucent panels, subtle gradients, and sharp borders.
Ordinary body copy uses Jura, main controls use Tiny5 pixel lettering, and
expressions, results, and mathematical syntax use PT Mono. All achievement
collection and ceremony copy uses Monocraft, including headings, names,
descriptions, status, counts, dates, and controls. Achievement surfaces use
rounded opaque windows and cards; this styling does not change the main console.
Expression entry and results lead; optional comments, achievements, and visual
reactions respond to calculations without delaying them.

The client uses React, TypeScript, and Vite. Fonts are served locally. Jura comes
from pinned `@fontsource/jura` 5.3.0; Monocraft uses official v4.2.1 Regular 400
and Bold 700 converted to WOFF2 without subsetting. `web/src/fonts.css` records
provenance, and `web/public/fonts/` contains both complete SIL OFL licenses.

## Default typing-first presentation

Provide:

- a directly editable expression that accepts typing and pasting;
- explicit degree notation through the required postfix `°`, including a keypad key;
- a prominent result or understandable error attached to its submitted expression.

Buttons insert the canonical syntax described by
[Calculation Engine](calculation-engine.md). The service evaluates all expressions.

A single exclusive tool bay contains Functions, Keypad, History, and Statistics,
in that order; settings open from the header into the same bay. It docks beside the
editor on desktop and becomes a bottom sheet on phones. Escape from the panel
or a tool trigger, and the close control, dismiss it and restore focus to the
active tool's trigger. Escape during IME composition does not dismiss the
panel. Phone keypad submission closes the sheet, reveals the answer, and
leaves focus on an available control.

The header also provides Achievements and a speaker control. Achievements opens
an independent native modal, not a section nested in History. The speaker shows
a slash when muted and has an accessible action label.

Do not reserve an empty result section; keep syntax help and limits with the
functions. Inserting a function wraps selected text or places the caret inside
empty parentheses. Retain function examples. The comma control has the short
accessible name “Запятая”, without an argument explanation or tooltip.

Opening syntax help scrolls it into view within the panel. Reduced-motion users
get an immediate scroll.

## Core interaction

- Enter submits the expression; the visible calculate action does the same.
- Editing and correcting an expression does not require closing a modal.
- A result remains associated with the expression actually
  submitted, even if the user has begun editing the next expression.
- A late response cannot overwrite a newer calculation's visible outcome.
- Explicit new submissions are new actions; uncertain network retries preserve
  their existing action ID.
- Full-value copying uses the canonical value.

On a network or service error, retain the input and display the failure.

## Result and error presentation

The real outcome is more prominent than commentary and remains available after
an effect finishes.

Mathematical errors use localized messages and source highlighting when a span
is available. For an end-of-input error, indicate the insertion point.
Offer “Go to error” only when a known span belongs to the unchanged editor
source. If the source has been edited, offer “Restore expression” instead.
Do not promise an automatic fix.

Service errors are distinct from expression mistakes. A joke must not replace
the information needed to correct an expression or retry a failed request.

Values follow the engine's canonical/rounded-display contract. Mathematical
decimal points remain `.` in both supported UI languages.

## History

History entries show source, outcome, and time. Selecting one restores its
unchanged expression and returns focus to the editor without submitting it.
History contains records only. Statistics shows total, success, and mathematical
error counts first, with division-by-zero attempts, operator/function usage,
longest expression, and parsed depth under expandable details.

The full ownership, durability, and paging rules are in
[History & Statistics](history-and-statistics.md). The same rules apply on a
phone and in a room.

## Achievement collection and new awards

The native collection modal has an opaque 16px-rounded window and opaque
12px-rounded cards, with two substantial card columns on desktop and one on
phones. Only the inner body scrolls; the outer dialog uses `overflow: clip`,
so focus and selected-card scrolling cannot move the header or close control.
Opening unmasks the final-size window over 300ms, with the header, close
control, and focus available immediately. Reduced motion opens it instantly.
Close and Escape restore the opener, falling back to the expression editor;
modal dismissal does not also close the tool bay. Loading and unavailable
states have their own retry action.

Names and icons are always visible. Locked icons are desaturated; earned cards
show color, earned status, and date. Keep the earned counter and one flat grid
ordered as earned, ordinary locked, then secret locked, preserving server
catalog order within each category. Do not add group headings. Every locked
card has the generic “Not earned” status (“Не получено”); secret cards have no
“Secret” badge or explanatory condition paragraph.

A locked secret description shows a fixed two-line ASCII pattern authored
independently of the real condition and its length. Two glyphs change at 4Hz
only while the description is visible, the collection is open, the tab is in
the foreground, and motion is allowed. Stop changes when closed, offscreen,
hidden, or under reduced motion. This is decorative fake cipher text, not
encryption. Keep the real condition out of rendered text, tooltips, and
accessible labels; expose one static concealed-condition label instead of
announcing changing glyphs.

Only an actual fresh secret grant gets a 600ms progressive decode: in its new
ceremony, or when a previously visible locked secret card becomes earned while
the collection remains open and its ID came from a fresh calculation POST
award. Ordinary achievements reveal their descriptions directly. Bootstrap,
quiet repair, reopening, reload, collection reads, and owner replacement never
decode old awards. Reduced motion reveals the earned description immediately.

Each genuinely new award from a calculation response gets a large pixel-art
icon, an earned heading, name, and unlocked description. A 1.25-second burst
precedes a 7.5-second readable card; hover, keyboard focus, and a background tab
pause the readable interval. Awards queue serially without stealing focus,
blocking typing, or intercepting clicks behind the spectacle. A new submission
does not clear the queue. The collection action opens the relevant earned card.
Bootstrap, collection reads, history, and reload do not replay awards; replacing
the browser identity resets the queue and deduplication state.

At widths up to 600px, the ceremony uses a compact horizontal 64px icon and
keeps the result readable. Error outcomes and short viewports place the card
after the result in document flow, without forced scrolling. The finite pixel
burst spans the viewport but never intercepts input.

Sound is independently persisted as `Preferences.soundEnabled`, defaulting to
`true`. Trusted click or Enter interaction activates the Web Audio context;
playback before a user gesture is not guaranteed. A new intentional calculation
dispatch from the calculate button, Enter, or keypad plays a quiet 65ms triangle
tick starting at 440Hz. It signals dispatch, not success. Typing, empty input,
Shift+Enter, IME composition, transport retries, and collection browsing stay
quiet. The tick shares the award cue's audio context and mute preference;
another dispatch replaces its active tick rather than queuing it. The original
short five-step award cue takes priority and stops any submit tick. Muting
stops both immediately, and disposal releases both. The settings label
`«Спецэффекты»` controls `Preferences.largeEffects` independently of sound and
humor. A basic earned notice remains visible when humor or special effects are
off. OS reduced motion separately reduces visual motion.

## Optional themes

If theme switching is included, provide at least two coherent themes and a
selector. Light and dark variants are a suitable starting point.

A theme may change visual character, but must preserve:

- readable results and errors;
- distinguishable controls and focus;
- the private/room/publication indicators;
- the same mathematical capabilities;
- usable layout in both languages and on phones.

Store the selected theme as a browser preference. Switching themes must not
reset progress or replay effects.
A theme does not silently enable more intense humor.

## Russian-first localization

Russian is the initial interface language, even if the browser's preferred
language is English. A previously selected supported language takes precedence
on subsequent visits.

The optional localization feature adds:

- an explicit Russian/English selector;
- complete interface labels, settings, help, empty/loading states, and errors;
- localized achievement names, descriptions, comments, and theatrical scenes;
- localized accessible names and status announcements.

Render system prose from stable IDs and translation catalogs. Persist history
and event identities independently of their translated labels. Both catalogs
must cover all enabled features; recognizable memes may use identical wording.

Each participant chooses their UI language and sees room events in that language.

Changing language must not:

- recalculate or alter an expression;
- change decimal syntax or function names;
- publish, unpublish, or resubmit a calculation;
- clear history, restart playback, or re-award achievements.

Without localization, show the Russian interface and omit the language selector.

## Personal and room context

The ordinary URL opens private mode. A room URL presents a clear invitation
explaining that new room-mode calculations are shared. Entry does not require
an account or mandatory name form.

Inside a room, display the room identity and publication state conspicuously.
Leaving returns private operation.

If the publication control is included, make its off state equally visible.
The user remains in the room and can watch and react. Theme or language changes
never switch publication back on.

Each tab supplies its own explicit calculation context; entering a room in one
tab cannot make a personal-mode tab publish.

## Optional “Show calculation”

Offer the localized control only for a supported successful result when the
feature is available. It opens the expression-reduction presentation defined in
[Visualization & Animation](visualization-and-animation.md).

Playback starts only on request. Show the result as soon as the service responds.

## Accessibility and phones

The core and every enabled audience-facing feature must work on a typical
modern phone as well as a laptop.

- No hover-only essential controls.
- Touch targets must be practical to operate.
- Long expressions and translated labels must not break the page width.
- Keep visible keyboard focus and meaningful control labels.
- Error and result information must not rely only on color.
- Respect reduced-motion preferences and provide immediate access to outcomes.
- Do not require sound, flashing effects, or device fullscreen.

Optional effects must not obstruct typing. Ordinary comments and scenes do not
build a backlog; genuinely new awards use the lossless serial queue above.
Operational effect limits are specified in [Fun & Chaos](fun-and-chaos.md).

## Acceptance

Exercise the full input/error/history/reuse path with keyboard and touch.
Verify that a stale response cannot replace the current result.

For each included presentation feature, switch it while an expression and
history are present. The mathematical and privacy state must remain unchanged.
For localization, also switch language while viewing an old error, an earned
achievement, and a room event.
