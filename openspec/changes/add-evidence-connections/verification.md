# Verification — 2026-10-17

## Automated and migration results

- PostgreSQL-backed `go test -race ./...` passed after restarting PostgreSQL following the harness restart. Includes provider journeys, MCP/browser parity, credential/session revocation, lifecycle fences and immutable snapshot/export fixtures.
- `go vet ./...`, generated TypeScript contract check, frontend type/lint/unit checks, production frontend build and strict OpenSpec validation passed. The Playwright configuration formatting failure was corrected before the final lint run.
- Docker Chromium: **36/36 passed**, desktop 1440px, tablet 768px and mobile 390px. Includes all four setup forms, handoff recovery, partial discovery, cross-page selection, conflicts, renewal, retirement, history, accessibility and keyboard navigation. Screenshots in `frontend/test-results/` were inspected; input border visibility was corrected.
- Production container built as `fed:evidence-connections`, image `sha256:7d8810dbfd80b94c63bfa8427ef952f557686ab53fcdea13f194d3d4a992c927`.
- Migrated local database and imported 49 sources into nine connections (six AWS, one GitHub, two Kubernetes). Backup: `/private/var/folders/_9/zz5h76fj6sg42r_w3n0408mw0000gn/T/opencode/connections-rollout.5yvq38/before.dump`.
- All eight local artifact hashes survived migration. Existing 785 evidence revisions and 49 source revisions retain the exact hashes recorded in `compatibility-baseline.md`, checked again after live access tests. No local frozen exports existed; immutable export and publication preservation are covered by fixtures.

## Local deployment and live-provider blockers

The connection-enabled development server runs at `http://127.0.0.1:8080` in server mode. Workers are stopped during rollout verification. Existing artifacts remain in `.data/artifacts`.

Live access tests were issued through session/CSRF-protected browser endpoints:

| Provider | Result | Required operator action |
| --- | --- | --- |
| GitHub | Server adapter unavailable | Configure the GitHub App registration, private key, client credentials and installation; authorize in the browser. |
| Google Workspace | Server adapter unavailable | Configure the OAuth application and administrator offline consent. |
| AWS | After operator SSO renewal, all four mapped accounts passed STS identity tests and completed account/region/EKS discovery in us-east-1 | Complete live collector acceptance. |
| Kubernetes | Both imported EKS contexts passed identity/endpoint/CA tests and paginated namespace discovery after the profile propagation fix | Complete live collector acceptance. |

Audit and Log Archive accounts remain agent-managed without verified server access. An operator encryption keyring is also required before storing new server-managed grants. Setup instructions are in `docs/provider-applications.md`.

At 20:01 UTC, development discovery returned one cluster plus 31 namespaces; production returned one cluster plus 28 namespaces. The service now passes the account-mapped AWS_PROFILE to imported trusted kubeconfig helpers, including collector configuration, instead of relying on default credentials. Provider/checks PostgreSQL-backed race tests passed and the updated binary was deployed. Discovery did not select additional assessment scope.

Task 11.6 remains **blocked**, not complete: GitHub/Google live authorization requires operator registrations, and live collector acceptance remains outstanding. Mock-backed success is not substituted for live-provider verification. Do not archive until the blocked journeys have been completed and the baseline comparison repeated.
