# Specification Index

Specifications for the Unnecessarily Advanced Calculator.

## Start here

- [PLAN.md](PLAN.md): product identity, priorities, defaults, non-goals, and release
  gates.

## Product and implementation contracts

| Document | Authoritative scope |
|---|---|
| [Calculation Engine](specs/calculation-engine.md) | Scientific grammar, numerical semantics, errors, and reduction steps |
| [Application Service](specs/application-service.md) | HTTP operations, PostgreSQL persistence, identity, and integration |
| [History & Statistics](specs/history-and-statistics.md) | Durable personal records, reuse, and metrics |
| [Client Experience](specs/client-experience.md) | Hybrid/minimal presentation, themes, Russian-first localization, and UX |
| [Visualization & Animation](specs/visualization-and-animation.md) | Optional playback that highlights and reduces subexpressions |
| [Multiplayer](specs/multiplayer.md) | Shared rooms, automatic publication, optional publication control, and reactions |
| [Fun & Chaos](specs/fun-and-chaos.md) | Authored humor, personal/collective achievements, and effect limits |
| [Public Deployment](specs/public-deployment.md) | Public access, durability, safety, and fallback |
| [Quality & Testing](specs/quality-and-testing.md) | Behavioral correctness and verification |
| [Demo & Operations](specs/demo-and-operations.md) | Rehearsal, audience participation, and recovery |

## Reading rules

1. Read the plan before interpreting a subsystem's requirements.
2. “Must” inside an optional feature describes its behavior **if included**; it
   does not make that feature a core release requirement.
3. Mathematical meaning belongs to the engine specification. Shared application
   data belongs to the service specification. Other documents reference these
   contracts rather than redefine them.
4. Specifications and code-facing artifacts are English. The application starts
   in Russian; optional localization adds a complete English experience.
5. Internal library choices and exact visual styling remain implementation
   decisions where observable behavior is already fixed.
6. Behavior changes must update the authoritative specification and affected
   consumers together.
