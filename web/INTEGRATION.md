# Frontend integration contract

`App.tsx` owns the editable expression, angle unit, last submitted outcome, and
personal history. `CalculatorInput` receives values and event handlers through
props. `History` receives records, paging state, and callbacks; selecting a
record restores only the editor and angle unit. It does not submit or replace
the last submitted outcome.

`App` also owns the active tool selection. `CalculatorInput` renders the
functions/keypad/history bay, docks it on desktop, and presents it as a phone
sheet. It handles dismissal and focus when the sheet closes.

All HTTP requests go through `src/api.ts`. Its relative `/api/...` paths keep
the browser on the page origin and send the anonymous session cookie. Go owns
calculation, durable records, and capability flags. Feature components should
consume `src/contracts.ts` types, receive server data from the root, and report
user actions upward. They should not add another evaluator or API client.

The root loads capabilities once and passes them to controls and history.
History keeps stored outcomes readable while warning when an old expression
uses a disabled extension. A failed capability request leaves the core keypad
available and shows a warning.
Optional controls should mount only after their complete server behavior is
enabled and integrated. For rooms, keep the current room code and publication
choice in `App` state for that tab. Supply room context explicitly on each new
calculation request; never infer it from the shared session cookie or storage.

Core labels and mathematical error messages are Russian. Existing translation
catalogs live under `src/i18n`; each optional feature should own stable message
IDs and supply a complete catalog before a language selector is exposed.
