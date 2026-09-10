# History & Statistics

> Scope: Required personal history; optional statistics and achievement state.

## Personal history

The service stores each anonymous browser identity's calculation history in
PostgreSQL. The same personal history is available in private and room modes.
Room participation does not publish that history.

A record preserves:

- its identifier and deliberate action identifier;
- the original submitted expression;
- effective angle unit and mathematical semantics version;
- canonical result or structured mathematical error;
- server creation time;
- trusted calculation facts needed by enabled statistics or achievements.

The application record shape is defined in
[Application Service](application-service.md).

## What is recorded

Record both successful calculations and accepted mathematical failures,
including syntax and domain errors. Their status must be visually distinct.

Malformed HTTP requests, size/work-budget rejections, rate-limit responses, and
internal failures are not mathematical history entries. A network retry of the
same action is not another calculation.

Committed history must survive application and PostgreSQL restarts, database
container recreation with its data volume, and browser reopening.

## Identity boundary

The service determines ownership from the anonymous session cookie, not from a
history owner supplied by the browser.

- Different browser identities cannot retrieve one another's records.
- Tabs sharing a browser identity see the same personal history.
- Another device or a cleared identity cookie starts a separate history.

Render expressions, errors, and aliases as text.

## Browsing and reuse

Show newest records first, with stable ordering when timestamps coincide.
Use cursor pagination with the service's default and maximum page sizes.
Keep older records reachable.

Selecting a record:

1. Restores the original expression into the editor.
2. Restores its angle unit and visibly updates the angle indicator.
3. Does not submit, duplicate history, trigger humor, or publish anything.
4. Allows editing before a new deliberate calculation.

If a restored expression uses an unavailable extension, keep it readable and
editable and identify the unsupported feature. Display its stored result.

Theme, layout, and language changes do not mutate records. Render error text
and dates in the current UI language.

## Optional reduction replay

Reading an old record's reduction is a read-only action. It uses that record's
context, does not create another calculation, and cannot publish old history to
a room.

If a record's semantics version cannot be reproduced, retain its stored result
and report playback as unavailable.

## Optional personal statistics

Use a small set of metrics that follow from authoritative records:

- total accepted calculations, successes, and mathematical errors;
- division-by-zero attempts;
- operator and function usage;
- longest accepted expression;
- greatest parsed nesting depth.

Do not invent a valid syntax depth for an expression that never parsed.
Transport retries do not inflate metrics. If counters are materialized, they
commit consistently with their source records or can be rebuilt from them.

## Achievements and joke statistics

Personal achievement IDs and earned timestamps survive normal restarts with
the personal identity. Opening the collection displays earned achievements
without replaying their announcements.

Label humorous indicators as jokes and document how they are calculated.
Measured statistics must derive from recorded data.

The reaction catalog and eligibility rules belong to
[Fun & Chaos](fun-and-chaos.md).

## Room aggregates

Only successfully published calculations contribute to public calculation
statistics or collective calculation triggers. A private or erroneous
calculation must not leak through a live counter because its author has a room
open. Explicit public emoji reactions remain separate room activity.

Room aggregates do not expose personal history or raw malformed expressions.
Room presence and activity data are short-lived.

## Acceptance

Verify restart durability, separate-browser isolation, paging beyond the first
page, successful and erroneous records, and context-correct reuse.

An old record can be opened while in a room without being published. A new
calculation of its restored expression follows the current, visibly indicated
publication setting.

Statistics and achievements, when enabled, remain consistent after retries and
reloads. Their absence or failure does not prevent reading history.
