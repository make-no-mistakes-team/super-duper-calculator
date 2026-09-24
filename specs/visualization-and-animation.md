# Visualization & Animation

> Scope: Optional, on-demand expression reduction playback.

## Intended experience

The user asks to see a calculation. The expression highlights a reducible
subexpression, replaces it with its value, and repeats until one number remains.

For example:

```text
2 + [3 * 4]  →  [2 + 12]  →  14
```

The example uses brackets to mark the active highlight. Animate each
replacement within the expression.

## Invocation and source of truth

Playback is explicitly requested with a localized action such as
“Show calculation.” Russian is the initial UI language.

The animation replays reduction data for a result already computed and saved
by the server.

The ordinary result is available without requesting or waiting for playback.

The reduction response is defined in
[Application Service](application-service.md). Its source, context, and final
value must match the selected calculation record.

## A meaningful step

For each server-provided step:

1. Show the current expression.
2. Highlight the supplied redex span.
3. Give the replacement a clear visual transition.
4. Collapse or reflow the surrounding expression naturally.
5. Continue with the next supplied expression.

The motion may use emphasis, movement, fades, or scale, but the active operation
and replacement must stay readable.

Each step resolves an operator or function application whose operands have been
evaluated. A literal-only input can immediately display its final value.

## Mathematical integrity

- Intermediate values come from the engine, not rounded result labels.
- Necessary grouping around a negative replacement must be preserved.
- Source spans are interpreted against that step's expression.
- Independent reductions follow the engine's deterministic order.
- The final displayed value equals the stored canonical result.

Playback runs in a presentation copy. It never rewrites the editable source,
changes the submitted expression, or replaces the record in personal history.

## Pacing and interruption

- Provide an immediate way to skip to the final result and to close playback.
- A completed playback can be replayed on request without a new calculation.
- Keep typical examples short enough for a brief demo.
- Large valid expressions must not require an unskippable long sequence.
- Starting another calculation or selecting another record cancels stale
  playback and prevents late data from attaching to the wrong expression.
- Background or closed views must not keep scheduling visible effects.

Playback does not trigger new calculation statistics, achievements, room
publication, or repeated jokes.

## Failure and unsupported records

Display syntax and evaluation errors in the normal error interface.

If reduction data is unavailable or inconsistent, keep the saved result and
editor available and report that playback is unavailable.

The same rule applies to history created under an unsupported semantics version.

## Presentation settings

Playback works within the current theme and language. Switching language only
changes its controls, not its mathematical contents or progress.

Reduced-motion users can inspect static steps or reveal replacements without
large movements. The final result and important text do not depend on seeing a
brief animation. The typing-first console can open playback explicitly.

Large comedic scenes must not compete with this presentation. The common
effect scheduler gives one strong effect the screen at a time.

## Acceptance

When included, demonstrate:

- arithmetic with precedence and parentheses;
- at least one named function;
- a reduction with a negative intermediate value;
- a successful one-number input;
- skip, close, replay, and interruption by a new calculation;
- unchanged input/history and no extra room events after playback;
- readable behavior on a phone and with reduced motion.

Verify the animation in the application and confirm that its final value matches
ordinary evaluation.
