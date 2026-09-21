# Repository Operating Guide

## Scope

These instructions apply to this repository. More specific `AGENTS.md` files in subdirectories override them.

Before changing a component, read the relevant package/config files and the applicable service or module documentation for that component.

Code and documentation changes should be scoped to the user’s request. Do not expand the architecture, introduce new systems, or make opportunistic refactors unless explicitly requested.

## Repository Map

- `cmd/` — Go entry points for the HTTP server, SciFact importer, and offline evaluation.
- `internal/` — backend configuration, HTTP/SSE API, OpenRouter client, retrieval pipeline, and PostgreSQL store.
- `db/migrations/` — PostgreSQL and pgvector schema migrations.
- `web/` — Next.js frontend, API/SSE client state, deterministic mock transport, and UI tests.
- `scripts/` — dataset download helpers; downloaded files live under ignored `data/` directories.
- `docs/planning/` — backend and frontend implementation plans and the locked API contract.

## Sources Of Truth

- `SPEC.md` defines v0 product scope and locked architecture.
- `docs/planning/backend.md` defines backend behavior; `docs/planning/frontend.md` defines the HTTP/SSE contract and frontend behavior.
- `db/migrations/001_init.sql` defines the database schema; `.env.example` and `web/.env.example` define runtime configuration; `compose.yaml` defines local PostgreSQL.
- `internal/**` and `web/src/**` define current implementation behavior. Go `*_test.go` files and `web/src/lib/run-state.test.ts` define tested expectations.
- `go.mod`/`go.sum` and `web/package.json`/`web/package-lock.json` define dependencies. Regenerate lock and checksum files with their package managers rather than editing them manually.

The implementation and tests define current behavior. The user request defines intended behavior. If they differ, surface the discrepancy and resolve it explicitly.

Do not hand-edit generated files. Use the repository’s documented generation, migration, or build commands.

## Coding Guidelines

- For every implementation, bug fix, refactor, or code review, load and follow the `ponytail` skill at its default `full` level. Prefer the first simple solution that works; do not use it to skip correctness, security, accessibility, or verification.
- Think before coding. Surface assumptions, ambiguities, and tradeoffs instead of silently guessing.
- If multiple interpretations are plausible, say so. Prefer the simplest valid interpretation.
- Prefer the minimum code required to solve the task. Do not add speculative features, abstractions, configurability, or unnecessary defensive logic.
- Keep changes surgical. Touch only code directly related to the request and preserve the existing project style and structure.
- Do not refactor, reformat, rename, or clean up unrelated code unless explicitly asked.
- Remove only imports, variables, functions, or files made unused by your own changes. Mention unrelated dead code instead of deleting it.
- Every changed line should be traceable to the requested task. If a change cannot be justified by the request, leave it out.
- Define verifiable success criteria before implementation. Prefer tests or concrete checks over vague goals like "make it work."
- For bugs, reproduce the failure first when practical, then fix it and verify the regression is covered.
- For refactors, verify behavior before and after the change.
- For larger tasks, state a short implementation plan with a verification step for each stage.
- If the solution becomes significantly more complex than necessary, simplify it before finishing.
- Optimize for small diffs, predictable behavior, and correctness over speed. Use judgment for trivial tasks.

## Architecture Boundaries

Do not introduce new:

- services
- libraries
- databases
- queues
- architectural patterns
- abstractions
- background workers
- state management systems

without explicitly explaining why and asking first.

Implement only the architecture requested. If the requested design has problems, point them out before writing code rather than silently redesigning it.

## Documentation

Update the applicable README, service guide, or module documentation in the same change when modifying behavior, APIs, lifecycle, configuration, operations, or structure.

Keep durable architecture notes, workflows, and project-specific conventions in the appropriate documentation instead of growing this file unnecessarily.

## Commands

Run from the repository root unless noted:

- `go test ./...` — run all backend tests.
- `go test ./internal/pipeline` — run a targeted Go package test; replace the package path as needed.
- `go vet ./...` — run Go static checks.
- `cd web && npm run lint && npm test && npm run build` — lint, test, type-check, and build the frontend.
- `go run ./cmd/server` — start the API on `HTTP_ADDR`.
- `cd web && npm run dev` — start the frontend development server.
- `docker compose up -d --wait postgres` — start local PostgreSQL with pgvector.
- `docker compose exec -T postgres psql -U jevtrieval -d jevtrieval < db/migrations/001_init.sql` — apply the idempotent schema migration.
- `go run ./cmd/import-scifact` — import and embed SciFact documents; requires PostgreSQL and OpenRouter configuration.
- `go run ./cmd/eval` — run offline evaluation after query and qrels data are available.
- `./scripts/download-scifact.sh` — download SciFact data with `uv`; `./scripts/download-multihoprag.sh` downloads MultiHopRAG with `hf`.

Do not report a command as passing unless it actually completed successfully. If a command is known to be unavailable, flaky, environment-dependent, or currently empty, state that clearly.

## Commits

Commit messages must be specific, detailed, and production-level.

Use a precise Conventional Commit subject. Add a body when the change needs context, including the intent, affected behavior or scope, important implementation decisions, and verification.

Avoid generic messages such as `update docs`, `fix stuff`, or `misc changes`.
