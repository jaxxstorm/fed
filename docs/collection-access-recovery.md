# Collection access recovery

The Ignis network requirement exposed three presentation/collection problems:

- An empty AWS cluster selection means discover clusters. The connection-aware
  Kubernetes runner passed that discovery into the legacy collector before
  matching verified standalone Kubernetes endpoints, producing duplicate
  missing-context gaps for already connected clusters.
- Earlier account/region access failures remained the latest observations for
  their partition keys even after later collection produced resource-level
  observations with different keys.
- The requirement had actual network findings, but its failing headline was
  replaced by the first access-recovery instruction.

The runner now discovers EKS names before identity matching. Reuse still requires
cluster-wide scope, successful Kubernetes observations, and an AWS DescribeCluster
response with the exact expected account/region/name ARN and matching endpoint.
Namespace-only, failed, and mismatched sources do not suppress collection gaps.

Successful EKS enumeration and verified Kubernetes-source matching now produce
new immutable access observations at the original partition keys. These have zero
resource-coverage counts and an unknown control outcome. They supersede errors
only at those exact keys; partial enumeration does not emit a success marker.
No earlier evidence is updated or deleted. A new collection is required for the
live status to reflect recovered access.

Access-only observations do not influence the resource technical outcome.
Pure access gaps are labeled **Evidence blocked**. When actual findings exist,
the **Failing** headline explains a finding and access gaps remain separate.

Validation includes PostgreSQL-backed race tests, immutable-history/access
supersession checks, partial-pagination tests, endpoint identity mismatch and
namespace-only checks, and presentation precedence tests.
