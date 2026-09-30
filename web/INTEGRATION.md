# Frontend integration contract

`App.tsx` owns the editable expression, angle unit, last submitted outcome, and
personal history. `CalculatorInput` receives values and event handlers through
props. `History` receives records, paging state, and callbacks; selecting a
record restores only the editor and angle unit. It does not submit or replace
the last submitted outcome.

`App` also owns the active tool selection. `useToolController` shares opening,
toggle, dismissal, and focus behavior across the header, tool bay, and collection
action. Escape ignores IME composition and leaves an active comic scene to handle
dismissal first. The controller restores the tool trigger on close, the editor
on history reuse or correction, and the submit button when a focused phone sheet
closes for submission. `CalculatorInput` renders the functions/keypad/history/
settings bay, docks it on desktop, and presents it as a phone sheet.

Calculation presentation lives under `src/features/calculation`.
`presentation.ts` formats values, mathematical errors, request failures, and
outcome announcements without owning state or issuing requests. `ResultView`
renders the submitted outcome and reports copy, retry, and correction actions
to `App`. Clipboard writes and their race guards remain in the root; display
rounding never changes the exact value copied or stored.

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

Core labels and mathematical error messages are Russian. Shared angle, outcome,
error, result, and request messages live in `messages.calculation` under
`src/i18n`; `messages.history` covers history title, loading, empty, and paging
states. Both catalogs cover these types, but the site has no language selector.
Each optional feature should supply a complete catalog before exposing one.
