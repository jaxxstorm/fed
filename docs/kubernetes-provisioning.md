# Kubernetes connections managed by deployment

`ksi-service provision-kubernetes` imports dedicated read-only Kubernetes
credentials. It is an operator command, not an HTTP or MCP endpoint. It requires
the database owner login and an existing eligible administrator resolved through
`KSI_OIDC_ISSUER` and either `KSI_ADMIN_SUBJECT` or `KSI_ADMIN_EMAIL`. It does not
bootstrap an administrator or allow runtime database credentials to provision.

Supply `KSI_CONNECTION_KEYS`, `KSI_CONNECTION_KEY_ID` and the exact HTTPS endpoint
allowlist in `KSI_KUBERNETES_ALLOWED_SERVERS`. Deliver the JSON array in
`KSI_PROVISION_KUBERNETES` through a deployment secret, not command-line arguments:

```json
[
  {
    "service_id": "existing-service-id",
    "name": "Production Kubernetes",
    "cluster_id": "kube-system-namespace-uid",
    "server": "https://cluster-api.example",
    "kubeconfig": "single-context kubeconfig with embedded read-only credentials",
    "select_cluster": true
  }
]
```

The provider validates TLS, the endpoint allowlist, the data-only credential
format and the live kube-system UID before persisting credentials. Stable IDs
make reruns idempotent. Changed credentials increment the connection generation
and revision atomically; in-flight collections using the old generation cannot
commit. Secrets are encrypted with the existing connection keyring and excluded
from metadata, history and audit records. Each update records
`connection.operator_provisioned` with the administrator identity.

`select_cluster` defaults to false. When explicitly enabled it discovers the
cluster and selects cluster-wide Kubernetes collection in the existing service
scope, preserving all existing scope entries. Scope changes feed the existing
collection planner. It does not declare evidence complete or remediate workloads.
Existing source selections are preserved on rerun, including disabled sources.
Disconnected/deleted connections are not silently restored. Operations are
transactional per connection; rerun after a partial multi-connection failure.

Dedicated service-account tokens do not rotate automatically. Replace the token
through its owning IaC stack and rerun provisioning to fence the previous grant.
Retain encryption keys required by historical backups. No schema migration is
needed for this command.
