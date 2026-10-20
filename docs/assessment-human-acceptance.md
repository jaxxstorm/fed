# Human walkthrough: reviewed assessment reports

Automated tests cannot complete this acceptance gate. The examples below contain
synthetic evidence and simulated reviewer actions, not evidence or approval for
Ignis. Guidance-source comprehension acceptance remains a separate gate.

Generate examples with PostgreSQL configured for the test harness:

```sh
KSI_ACCEPTANCE_OUTPUT=/path/to/private/review-directory \
  go test ./internal/mcp -run TestReviewedReleaseBrowserMCPJourneys -count=1
```

Each `AFC-CSO-INB` and `KSI-CNA-RNT` folder contains `assessment.zip` and
`public.html`. Extract the archive and open `index.html` for the restricted
report. Open `public.html` separately for the selected public disclosure. The
example public page's live application links are not functional offline.

Ask the reviewer to explain in their own words:

1. What does the report say the service actually does? Which operating period and
   sample does the evidence cover?
2. What remains unproven? Does accepting this incomplete report mean the
   requirement is compliant?
3. In the technical example, how do the access gap, contradictory evidence, and
   open issue affect their understanding?
4. What information appears publicly, and what requires restricted access? What
   happens to copies already downloaded if a release is withdrawn?

Record the actual reviewer's responses, the exact package hashes reviewed, date,
and any requested changes. Do not infer acceptance from test results, a successful
deployment, or a general instruction to continue implementation. After changes,
regenerate the examples and repeat only the affected acceptance checks.

## Current acceptance record

Pending. No genuine human comprehension or disclosure-preview acceptance has
been recorded for this change. No production disclosure or publication is
approved by these fixture journeys.

Verified walkthrough artifacts (archive SHA-256):

- `AFC-CSO-INB`: `3df19a8f34d386eab97a088892dd0d432db9e887b6d1ac7968da0c00494b0e77`
- `KSI-CNA-RNT`: `9ed2aaf498a187268ab40df5538e893d7d02088cd73f13e0cea52286c6ba6668`

The artifacts are in the local OpenCode temporary directory under
`assessment-acceptance-final/`. Each folder includes `public.html`,
`assessment.zip`, and the extracted `report/index.html`. Every entry in the
archives' `SHA256SUMS` files was verified. The final assessment, MCP, web, and
storage race suites passed; human acceptance remains pending.
