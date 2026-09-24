# Client Experience

> Scope: Required Russian-first web client, with optional presentation features.

## Identity and visual direction

The console uses dark translucent panels, subtle gradients, and sharp borders.
Cyrillic controls use pixel lettering; mathematical text stays in a readable monospace.
Expression entry and results lead; optional comments, achievements, and visual
reactions respond to calculations without delaying them.

The client uses React, TypeScript, and Vite. Exact typography, colors, layout
details, and animation libraries remain design choices.

## Default typing-first presentation

Provide:

- a directly editable expression that accepts typing and pasting;
- a compact, visible degree/radian control;
- a prominent result or understandable error attached to its submitted expression.

Buttons insert the canonical syntax described by
[Calculation Engine](calculation-engine.md). The service evaluates all expressions.

A single tool bay contains scientific functions, an arithmetic keypad, and
personal history. It docks beside the editor on desktop and becomes a bottom
sheet on phones. Escape and the close control dismiss it and restore focus to
the trigger. Phone keypad submission closes the sheet, reveals the answer,
and leaves focus on an available control.

Do not reserve an empty result section; keep syntax help and limits with the
functions. Inserting a function wraps selected text or places the caret inside
empty parentheses.

Opening syntax help scrolls it into view within the panel. Reduced-motion users
get an immediate scroll.

## Core interaction

- Enter submits the expression; the visible calculate action does the same.
- Editing and correcting an expression does not require closing a modal.
- A result remains associated with the expression and angle setting actually
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

Service errors are distinct from expression mistakes. A joke must not replace
the information needed to correct an expression or retry a failed request.

Values follow the engine's canonical/rounded-display contract. Mathematical
decimal points remain `.` in both supported UI languages.

## History

History entries show source, outcome, and time. Selecting one restores its
expression and angle unit and returns focus to the editor without submitting it.

The full ownership, durability, and paging rules are in
[History & Statistics](history-and-statistics.md). The same rules apply on a
phone and in a room.

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
- change the angle unit;
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

Optional effects must not obstruct typing or create a backlog of announcements.
Operational effect limits are specified in [Fun & Chaos](fun-and-chaos.md).

## Acceptance

Exercise the full input/error/history/reuse path with keyboard and touch.
Verify that a stale response cannot replace the current result.

For each included presentation feature, switch it while an expression and
history are present. The mathematical and privacy state must remain unchanged.
For localization, also switch language while viewing an old error, an earned
achievement, and a room event.
