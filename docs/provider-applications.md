# Provider application setup

Enable connection management with `KSI_CONNECTIONS_ENABLED=true`. Set
`KSI_PUBLIC_URL` to the browser-facing HTTPS origin. Configure
`KSI_CONNECTION_KEY_ID` and `KSI_CONNECTION_KEYS` (a JSON map of key IDs to
base64-encoded 32-byte AES keys) before authorizing an application. Keep keys
outside the database and retain old key versions while rotating stored grants.

## GitHub App

1. Register a GitHub App in the intended owner account. Grant repository
   **Metadata: read** and **Contents: read**. Select only the repositories needed
   for assessment. Do not grant repository write permissions.
2. Set the user authorization callback to
   `https://YOUR_ORIGIN/connections/callback/github`. Enable user authorization
   for the app. Install the application separately, then copy its installation
   ID from the installation settings URL into the connection configuration.
   The callback requires the service-created state; an installation setup
   redirect by itself cannot activate a connection. The optional installation
   setup URL is `https://YOUR_ORIGIN/connections/setup/github`; it returns users
   to service selection without treating installation query parameters as proof
   of ownership.
3. Configure `KSI_GITHUB_APP_ID`, `KSI_GITHUB_APP_SLUG`,
   `KSI_GITHUB_CLIENT_ID`, `KSI_GITHUB_CLIENT_SECRET`, and
   `KSI_GITHUB_PRIVATE_KEY_FILE` on the server. The private key file must contain
   an RSA key of at least 2048 bits in PKCS#1 or PKCS#8 PEM format.
4. Start authorization from the connection. The browser user must have access
   to the installation. The service checks both user access and the App's
   installation/account identity before storing the binding.
5. Discover repositories, then select exact full commits and evidence paths.
   Branch names must be resolved to full commit hashes before collection.

Installation tokens are short-lived and repository-restricted for file reads.
Local disconnect does not uninstall the App. Remote removal is performed in
GitHub installation settings; existing evidence remains retained.

## Google Workspace

1. Enable **Admin SDK API** in a Google Cloud project. Configure the OAuth
   consent screen, audience and test users or publishing status appropriate to
   the organization. Follow administrator API access policies.
2. Register a **Web application** OAuth client with redirect URI
   `https://YOUR_ORIGIN/connections/callback/google_workspace`.
3. Configure `KSI_GOOGLE_CLIENT_ID` and `KSI_GOOGLE_CLIENT_SECRET` on the
   server. Enter the canonical Workspace customer ID in connection setup.
4. Have an authorized administrator consent to these scopes:
   - `https://www.googleapis.com/auth/admin.directory.customer.readonly`
   - `https://www.googleapis.com/auth/admin.directory.orgunit.readonly`
   - `https://www.googleapis.com/auth/admin.directory.group.readonly`
5. The service requests offline access and PKCE, verifies the customer through
   `customers/my_customer`, and retains the refresh token for coordinated
   renewal. Reauthorize with consent if no offline token is returned.

Discovery includes organizational units and groups only. Directory org-unit
responses are size-bounded and presented in local pages; groups use provider
pagination. Permission failures remain visible by resource kind. Snapshots
contain selected Directory metadata, not Gmail, Drive, membership, devices or
audit logs. Identity verification is not a statement of compliance or proof of
access to every resource kind.

## Deployment readiness

The provider catalog reports `server_adapter_available`. Missing application
configuration leaves agent-managed discovery available. Partially configured
applications fail startup with a redacted configuration error. Never paste
provider secrets into MCP parameters, source hints, browser storage or evidence.
Agent tokens need explicit `connect`; selecting scope also requires `edit`.

## Kubernetes

Set `KSI_KUBERNETES_ALLOWED_SERVERS` to a JSON array of exact HTTPS API server
URLs. For operator-installed contexts also set `KSI_KUBERNETES_ALLOWED_CONTEXTS`
to a JSON array of context names. Only trusted operator contexts may execute
installed authentication helpers. Uploaded kubeconfigs must contain one context,
one cluster and one embedded bearer-token or client-certificate identity. TLS
bypass, proxies, impersonation, executable/auth-provider plugins and file
references are rejected before client construction.

The cluster identity is the UID of its `kube-system` namespace. Configure that
UID as both account and cluster ID, and the exact allowed API server endpoint.
Read permission for `namespaces/kube-system` is required to establish identity.
Use `known_namespaces` when the identity can read specific namespaces but cannot
list all namespaces; those results remain explicitly partial.

Use a read-only ClusterRole for namespace metadata (`get`, optionally `list`),
and namespace-scoped Roles for `get/list` on pods and network policies needed by
the workload collector. Do not grant secrets access or write verbs. Scope
selection retains namespace UIDs so deleting and recreating a namespace does
not silently inherit the previous selection.

For uploaded credentials, create a Server Managed connection and select **Upload
access**. MCP `begin_connection_authorization` returns the same short-lived
browser handoff. The authorized browser submits the kubeconfig with CSRF and
single-use state. It is validated and identity-tested before encryption; it is
never returned through MCP or the connection metadata API. Start another handoff
to replace expiring credentials. Local disconnect deletes local credentials;
rotate/revoke the token or certificate separately at the cluster.

## AWS profiles and role trust

Profiles execute in the **server's** SDK environment. Populate
`KSI_AWS_ACCOUNT_PROFILES` for existing account/profile mappings, and use the
connection's profile field for new configuration. Run `aws sso login --profile
PROFILE` in that environment when sign-in expires. Access tests show the server
principal and verify the final STS account ID. Only `us-east-1` is supported.

For assume-role access, configure a target role ARN in the selected account.
Its trust policy must authorize the actual server principal, for example:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"AWS": "arn:aws:iam::SERVER_ACCOUNT:role/EvidenceServer"},
    "Action": "sts:AssumeRole",
    "Condition": {"StringEquals": {"sts:ExternalId": "OPERATOR_ASSIGNED_EXTERNAL_ID"}}
  }]
}
```

Give the server principal permission to assume that exact role. Grant the role
only enabled collector reads: EKS ListClusters/DescribeCluster, RDS
DescribeDBInstances/DescribeDBClusters, ElastiCache
DescribeCacheClusters/DescribeReplicationGroups, and GuardDuty
ListDetectors/GetDetector. Artifact-bucket permissions are configured separately.
For a role external ID, choose Server Managed and complete the authorization
handoff; it accepts the external ID, tests the role, and encrypts the binding.
No AWS access key upload or infrastructure write operation is used.

## Migration, key rotation and rollback

1. Stop worker processes and capture database/artifact backups and hashes of
   immutable evidence, source revisions, frozen exports and publication records.
2. Run `ksi-service migrate` with the configured `DATABASE_URL`.
3. With the same administrator identity settings used by the server, run
   `ksi-service import-connections`. In development use `KSI_DEV_IDENTITY=true`.
   This idempotently imports existing sources and environment mappings while
   preserving source IDs and existing record payloads. Retired bindings are not
   resurrected. Inspect the resulting connection modes before resuming workers:
   imported GitHub sources need a verified installation instead of global-token
   fallback.
4. Start the connection-aware server and workers. Set
   `KSI_CONNECTIONS_ENABLED=true` to expose setup routes and MCP tools. Workers
   enforce existing bindings even when that UI feature flag is disabled.
5. For encryption-key rotation, add the new key to `KSI_CONNECTION_KEYS`, retain
   old keys, and set `KSI_CONNECTION_KEY_ID` to the new ID. Restart the processes
   with that keyring and run `ksi-service rotate-connection-keys`. This rewraps
   grants without changing evidence timestamps or credential generations. Keep
   old keys until rotation succeeds and pending authorization attempts have
   expired (at least ten minutes), and retain recovery keys with encrypted
   backups. A generation conflict should be retried with both key versions
   available.
6. Compare immutable record/export hashes after migration and representative
   read-only smoke journeys. Discovery alone must not change assessment scope,
   evidence freshness, technical outcomes or human review.

Do **not** resume a pre-connection worker binary after retirement or rebinding:
older reconciliation ignores tombstones and older collectors use global access.
For rollback, stop workers first, retain the additive schema and immutable
records, and either roll back to a connection-aware compatible version or restore
a coordinated pre-change database/artifact backup with the matching old binary.
Disabling the UI flag is not a substitute for a connection-aware worker.
