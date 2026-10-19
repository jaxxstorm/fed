# Requirement-page loading

## 2026-10-19 investigation

- Production logged a requirement-page response taking 7,996 ms.
- The Ignis Class A network requirement's instruction/status projection encoded
  to 6,758,603 bytes. Gzip level 1 reduced that same projection to 820,689 bytes
  (88% less). This measures transfer size, not a production latency guarantee.
- Read-only query-plan inspection measured the observation queries at about
  134 ms combined. A remote diagnostic run also found that the detail projection
  fetched the observation summary twice. Network latency makes remote diagnostic
  timings unsuitable as estimates of ECS-to-RDS latency.

The change compresses browser GET JSON and static assets, reuses observation
summaries within one repeatable-read detail snapshot, and mounts collapsed
finding resources only when expanded. Each request still checks authorization
and reads current service state; the snapshot uses one freshness timestamp.
MCP streams and artifact downloads retain their existing transport.

Validation: PostgreSQL-backed assessment/web race tests, browser-response
compression/payload-preservation tests, production frontend build, and all 12
responsive guidance browser tests passed. Production rollout and end-to-end
latency verification remain pending.
