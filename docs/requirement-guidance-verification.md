# Requirement guidance verification

Verified against the running local service on 2026-10-17.

- Audited all **291 requirement detail endpoints**: 183 not started, 71 not applicable, 26 needing scope decisions, 3 in progress, 5 failing and 3 blocked.
- Every requirement returns nonempty explanations and actions. Every applicable or unresolved requirement includes a source plan, assertion-level evidence instructions and the investigation steps supplied by `RuleInstructions`.
- Recorded collection recovery is grouped by error and execution environment. Findings and missing evidence remain distinct from access errors. Historical errors are not cleared merely by testing a connection.
- Reviewed application guidance is identified separately from generic investigation guidance; instructions do not invent control-specific evidence requirements beyond the official statement and configured assertions.
- Shared presentation tests cover status precedence, next-action completeness and grouped blocker diagnostics. PostgreSQL-backed assessment/web race tests passed.
- Desktop/tablet/mobile workflow checks passed. The source-edit checks initially collided with concurrent contract regeneration in the Vite development server; all three passed when rerun after the build finished.
- Built and deployed the updated embedded frontend and server at `http://127.0.0.1:8080`.

The audit validates guidance availability, not compliance or evidence sufficiency. Existing assessment outcomes and immutable evidence are retained.
