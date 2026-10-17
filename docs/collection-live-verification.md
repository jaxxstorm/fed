# Local collection verification

The automatic-collection build was migrated and started locally on 2026-10-16
against service `dc0c7bef-7a80-4233-9ff2-cb69c6b0554f` (ignistechnologies).
Startup generated seven collector jobs without requiring assessment drafts.
The collection overview and KSI-SVC-SIN detail both returned HTTP 200.

Fresh stored collector evaluations included:

- Four RDS storage-encryption failures and three backup-retention failures.
- Two ElastiCache transit-encryption failures.
- Two EKS public-endpoint failures.
- Development Kubernetes workload observations: 213 ingress-isolation failures
  and 213 egress-isolation failures. These are workload counts, not independent
  cluster counts or proof of runtime reachability.

Every AWS resource observation was in us-east-1. Partition errors also retained
us-east-1 in their resource identifiers. No infrastructure changes were made.

Recorded gaps:

- Accounts `479958273743` and `620316221154`: credentials/account validation failed.
- Production cluster `production-9101335`: namespace enumeration failed.
- GitHub repository collection: worker has no GitHub token.
- Customer retention objectives and approved endpoint CIDRs remain unset.

These gaps prevent a claim of complete coverage. Batch-level error outcomes
coexist with usable resource observations and explicit per-target errors.

Verification before deployment: PostgreSQL-backed `go test -race ./...`,
`go vet ./...`, `make build` (including templ generation), and strict OpenSpec
validation passed. The OpenSpec checklist still contains incomplete verification
and compatibility work; this record does not mark the overall change complete.
