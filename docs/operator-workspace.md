# Operator workspace

The React/TypeScript workspace is served by the Go binary. Bun is used for
development and builds; the production runtime remains Go-only.

## Build and run

```sh
make build
```

This regenerates the browser OpenAPI schema/types, installs pinned frontend
dependencies, builds Vite assets and embeds them into `bin/ksi-service`. Set the
existing database, identity and artifact configuration before running it. A
backend-only `go build` works with the placeholder, but the browser returns a
clear 503 until assets are built. Release/container builds require `index.html`.

For frontend development, run the Go backend on port 8080 and run `bun run dev`
inside `frontend/`. Vite proxies `/api` and `/auth` to Go. `/gallery` is an explicitly
labeled illustrative design gallery, available on the development server.

## Chromium in Docker

Install the frontend dependencies first, then start the matching Playwright image:

```sh
docker compose -f compose.browser.yaml up -d --wait
```

From `frontend/`:

```sh
bun run test:docker
```

Tests connect to Chromium over the loopback WebSocket endpoint. The browser
container reaches Vite through `host.docker.internal`; no local Chromium install
is required. Screenshots and reports are in ignored `frontend/test-results/`.

The read-only live-service suite expects the Go service on port 8080 and the
configured `ignistechnologies` assessment. It exercises navigation and evidence
inspection without changing that service's records:

```sh
bunx playwright test --config playwright.live.config.ts
```

Stop the browser when finished:

```sh
docker compose -f compose.browser.yaml down
```

## Contracts and checks

`cmd/ui-contract` derives response schemas from Go types. `bun run contracts`
regenerates `frontend/openapi.json` and TypeScript bindings;
`bun run contracts:check` detects drift. The frontend view types are checked
against generated response contracts during TypeScript compilation.

Run `bun run lint`, `bun run test`, `bun run build`, and `bun run test:docker`.
Backend integration tests use `KSI_TEST_DATABASE_URL` with isolated schemas.
The presentation cache test asserts a fixed eight-query load and no additional
status queries across the catalog, and compares cached results with the existing
domain projection for findings, review, expiry and reusable documentary support.

## Status semantics

Passing required requirements are divided by applicable required requirements.
Unresolved applicability, optional and recommended work are shown separately.
Evidence completion and human review remain independent of technical findings.
The UI never reports a live external-agent session or ETA without telemetry.

Progress uses one repeatable-read transaction and a common reference time.
Browser mutations use the existing authorization, CSRF, revision and idempotency
contracts. Evidence previews request exact revisions. Plain text/Markdown and
JSON previews are rendered as escaped content; raw HTML is never executed.

## Deployment and rollback

Build the image with the repository Dockerfile, which compiles frontend assets
before the Go binary. Existing requirement, run, evidence-download and export
URLs remain valid; protocol and operational endpoints are registered separately.

This change requires no schema migration. Roll back by deploying the previous
compatible binary/image, retaining the same database and artifact storage.
Evidence revisions, reviews, source records and immutable exports remain intact.
