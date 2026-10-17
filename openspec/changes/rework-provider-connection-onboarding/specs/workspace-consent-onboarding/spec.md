## ADDED Requirements

### Requirement: Distinct application readiness and administrator prerequisites
Workspace setup SHALL distinguish Fed operator OAuth client registration from customer administrator consent. It SHALL describe Admin SDK enablement in the application Google Cloud project, consent audience, tenant API policies, exact redirect URI and the existing customer/org-unit/group read-only scopes.

#### Scenario: Missing application registration
- **WHEN** the OAuth client or callback origin is not configured
- **THEN** Connect displays the missing operator configuration and does not redirect into an unusable authorization flow or request client secrets from the customer

### Requirement: Consent-first verified customer discovery
Fed SHALL allow initial Workspace setup without a manually entered customer ID, request offline consent using PKCE and single-use state, resolve the canonical customer through the authorized Directory API, and show that identity for confirmation before connection finalization.

#### Scenario: Initial connection
- **WHEN** an administrator grants the required consent and confirms the verified customer
- **THEN** Fed finalizes one service-owned connection with encrypted renewable credentials and offers Directory resource discovery

#### Scenario: Reconnect to another customer
- **WHEN** a reconnect returns a different customer from the existing connection
- **THEN** Fed rejects the substitution and preserves the original tenant binding and evidence history

### Requirement: Recoverable consent and renewal
Fed SHALL preserve setup progress and distinguish cancellation, denied consent, missing customer identity permission, tenant policy restrictions, missing offline grant and expired/revoked grant. Refresh responses omitting a refresh token SHALL preserve the existing valid token.

#### Scenario: Incomplete or denied authorization
- **WHEN** consent cannot establish verified renewable access
- **THEN** setup shows an actionable retry or administrator-policy step and does not claim the connection is ready

#### Scenario: Partial Directory access
- **WHEN** verified customer access allows groups but denies org-unit discovery
- **THEN** group results remain available and org-unit coverage is explicitly partial rather than implying complete access

### Requirement: Honest Workspace coverage
The onboarding journey SHALL describe its org-unit/group metadata coverage and SHALL NOT imply Gmail, Drive, users, device or audit-log collection. Consent and discovery SHALL NOT update assessment completion, human review or evidence freshness.

#### Scenario: Successful consent only
- **WHEN** a Workspace connection is authorized
- **THEN** the UI reports verified access and the next discovery step while evidence and assessment state remain unchanged
