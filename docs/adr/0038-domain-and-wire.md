# ADR 0038: The domain is not the wire: no transport tags on domain structs

**Status**: accepted 2026-10-20 (the user's ruling in the second architecture review).

## Context

The second architecture review (reports/architecture-review-2.html, candidate 4) proposed that the
character sheet's derived block be defined once as the shape the API serves, with JSON tags on
`charactersvc`'s structs, because a new derived field today touches the domain struct, the handler's
DTO, a `rest/wire` DTO, the mapping function and the TypeScript type. The precedent offered was
[ADR 0018](0018-transactions-and-undo.md) step 11's rule for the roll results (W5, W6: the journal
payload and the HTTP answer are one Go struct).

The user refused: "clean architecture not to be challenged. conflating domain and wire is a big
NO". This ADR records the line so that no later review proposes it again.

## Decision

1. **Domain structs carry no transport tags.** An entity or value of `internal/domain` (the services'
   entities, the derived block, the warnings) has no `json` tag and is never serialised as an
   answer. `docs/clean_architecture.md` §4 stands as written.
2. **One mapping per answer, on the transport side.** The shape the API serves is a DTO in
   `internal/rest/wire` (when several handler packages serve it) or in the handler's package, with
   one mapping function from the domain type. An answer two endpoints share (the sheet's derived
   block for `GET /api/character` and the creation draft) shares that one mapping, never a second
   copy.
3. **The action log's payload records are the exception, and the only one.** A roll result's
   record (`combatsvc`, `castsvc`) is stored JSON in the log, so it carries its tags where it is
   written; serving it as the HTTP answer unchanged (ADR 0018 step 11) is reading stored data, not
   tagging the domain. The exception does not extend to anything that is not a stored payload.

## Consequences

- The cost named by the review is accepted: a new served field is one field on the domain struct,
  one on its DTO and one on the TypeScript type, kept equal by the DTO's tests.
- A refactor that reduces the mapping does so on the transport side (one shared mapping, one
  DTO), never by moving tags into the domain.
- Not covered: the web's types, which mirror the DTOs and are the web's own.
