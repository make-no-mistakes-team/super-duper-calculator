# Unnecessarily Advanced Calculator

> Documentation and code-facing artifacts: English.  
> Default application language: Russian. English UI localization is an optional feature.

## Product concept

A polished scientific calculator with an unexpectedly playful personality.

At first glance, it is a familiar, calm, attractive tool. Through ordinary use,
people discover comments, achievements, numerical jokes, escalating reactions,
and occasional theatrical effects. A shared room can turn that discovery into
a collective experience.

## Course requirements

The course assignment requires:

- a GUI client that submits an arithmetic expression as a string;
- calculation on a service, which may run locally;
- a defined, potentially limited operation set;
- a result or an understandable expression error;
- a database containing previous calculations;
- retrieval of the client's own calculations;
- clickable history that allows previous expressions to be reused;
- a public demonstration, either live or recorded.

The assignment also evaluates the technology stack and development environment.

## Scope and priorities

The required core satisfies the course assignment. Optional features extend its
presentation and audience experience.

### Required core

- A typing-first web client with an editable expression and on-demand keypad.
- Server-side scientific calculation.
- The mathematical baseline in [Calculation Engine](specs/calculation-engine.md).
- Clear syntax, domain, numerical, and service errors.
- Database-backed personal history, including accepted erroneous calculations.
- Anonymous browser identity; no accounts or cross-device history promise.
- Reuse of expressions and their calculation settings from history.
- Russian interface text on first use.
- Keyboard operation, readable results, and a usable phone layout.
- Reproducible local operation and meaningful correctness checks.

The full core path is:

**input → service → calculation/error → database → history → reuse.**

### Character and presentation showcase

These are optional for course compliance but central to the intended product:

- authored comments, numerical easter eggs, and personal achievements;
- a small escalating usage gag and one reversible theatrical incident;
- compact statistics derived from real calculation records;
- switchable visual themes;
- Russian/English localization with a language selector.

Themes and effect settings are independent. Neither changes the
supported mathematics or silently changes whether a calculation is public.
Exact colors, typography, and component composition remain design decisions.

### Highly desired audience features

These are optional and can be delivered independently where useful:

- a stable public URL for the personal calculator;
- a separate room link for the audience;
- shared activity, approximate presence, and emoji reactions;
- a small number of collective achievements or reactions.

A public personal calculator is useful without multiplayer. Inside a room,
new calculations are published automatically by default, after the user has
clearly entered the public context. A publication on/off control is a separate
optional room enhancement.

### Optional calculation playback

A user may request an animated explanation with a button such as
“Show calculation.” The animation highlights reducible subexpressions
(redexes), replaces them with their values, and continues until one number
remains.

Playback starts on request. The result remains available during the animation.

### First mathematical extensions

Factorial, percentages, and remainder are the first optional additions after
the scientific baseline is stable.

### Outside scope

- Symbolic algebra, equation solving, and function plotting.
- Arbitrary user-defined functions or execution of user code.
- Accounts, social profiles, chat, or cross-device synchronization.
- Collaborative expression editing.
- Competitive rounds, economies, or elaborate progression systems.
- Runtime LLM-generated jokes or an embedded chatbot.

## Product invariants

1. The service owns mathematical meaning and the authoritative result.
2. Humor, achievements, themes, language, and multiplayer never alter an answer.
3. History belongs to the anonymous browser identity, not to a room.
4. Entering a room never publishes old personal history.
5. A personal-mode calculation remains private even if another tab is in a room.
6. The calculator remains usable when optional systems are unavailable.
7. An effect cannot destroy input, trap the user, or conceal the real outcome.
8. UI localization does not change expression syntax or angle settings.
9. Optional features are either complete and usable or absent from the released
   surface; unfinished controls are not part of the demo.

## Technical baseline

- Client: TypeScript, React, and Vite.
- Service: Go.
- Persistence: SQLite.
- The engine is an independently testable package within the Go application.
- Ordinary application operations use same-origin HTTP/JSON.
- If rooms are implemented, server-sent events deliver server-to-client updates;
  calculations and reaction changes still use HTTP requests.
- Hosting provider and other libraries remain implementation choices within
  these storage and deployment constraints.

The application contract is defined in
[Application Service](specs/application-service.md). Implementation may choose
internal data structures and libraries, but independent clients must not invent
different mathematical, publication, or localization semantics.

## Default user experience

The ordinary URL opens a private calculator in Russian, using degrees.
A returning browser restores its saved preferences.

The user can type or paste an expression, calculate, read the outcome, and reuse
history without encountering a joke or animation that requires interaction.

A room URL clearly identifies the shared context before entry. New room-mode
calculations contribute to the shared experience; leaving restores private
operation.

## Release policy

Publish runnable versions through GitHub Releases tied to Git tags. Use GitHub
Actions to check and build the tagged source, package the Go executable with
the built web assets, and upload the archives as release assets.

Reuse the project's build and check commands and standard archive tools.
Do not create or extend a custom release system, including parallel release
manifests, source-revision stamp protocols, or frozen-demo bundle lifecycles.
Use GitHub's release features for publication and, when required, immutability.
An alternative release system requires explicit user approval.

Exclude `.env`, credentials, existing SQLite databases and sidecar files, and
personal history from release assets. A downloaded application initializes its
own database. GitHub's automatic source archives do not replace runnable
application packages.

Release publication does not replace correctness checks or presentation
rehearsal. The presentation and fallback requirements are defined in
[Demo & Operations](specs/demo-and-operations.md).

## Release gates

### Core ready

The required path works with a real service and database, survives an ordinary
restart, isolates different browser histories, and passes the critical
correctness scenarios. This satisfies the functional course requirements.

### Showcase ready

Every included extra has been exercised in the actual interface. Its failure or
disablement leaves the core usable. Jokes have controlled frequency, themes and
locale changes preserve state, and the chosen discoveries are reproducible.

### Audience ready

If public hosting or rooms are included, external phones can use the released
surface. Multi-client behavior, ordinary classroom bursts, and a local or
recorded fallback have been checked.

Once a demo candidate is selected, its feature set is frozen. Exclude incomplete
extras from that candidate.

## Demonstration shape

The demo should establish correctness before escalating:

1. Open the apparently ordinary scientific calculator.
2. Evaluate a scientific expression and reuse it from history.
3. Show a precise, recoverable error.
4. Reveal a numerical joke, an achievement, or an escalating reaction.
5. If available, invite the audience into a room and trigger a collective event.
6. Use theme switching, localization, or redex playback only when they strengthen
   the short narrative; showing every implemented feature is not required.

The exact rehearsal and fallback requirements are in
[Demo & Operations](specs/demo-and-operations.md).

## Specification ownership of facts

- [Calculation Engine](specs/calculation-engine.md): grammar, numerical meaning,
  errors, limits, and reduction semantics.
- [Application Service](specs/application-service.md): application operations and
  cross-boundary data.
- [History & Statistics](specs/history-and-statistics.md): durability, isolation,
  reuse, and derived metrics.
- [Client Experience](specs/client-experience.md): interaction, themes, modes,
  localization, and personal/public context.
- [Visualization & Animation](specs/visualization-and-animation.md): optional
  on-demand redex playback and motion behavior.
- [Multiplayer](specs/multiplayer.md): rooms, publication, reactions, and presence.
- [Fun & Chaos](specs/fun-and-chaos.md): reaction rules, achievements, and pacing.
- [Public Deployment](specs/public-deployment.md): delivery and public safety.
- [Quality & Testing](specs/quality-and-testing.md): verification obligations.
- [Demo & Operations](specs/demo-and-operations.md): demonstrable outcomes and
  recovery.

Subsystem documents refine this scope; they do not silently promote optional
features to core requirements.
