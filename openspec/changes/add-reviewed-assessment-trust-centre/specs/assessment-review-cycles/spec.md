## ADDED Requirements

### Requirement: Separate claims and review authority
Fed SHALL distinguish agent-drafted implementation claims, human organizational adoption, agent evidence conclusions, internal review, and independently designated assessor conclusions. Human decisions SHALL identify actor, organization/capacity, timestamp, exact assessment/evidence revisions, procedures, sampling, conclusion, and notes. Agent tokens MUST NOT adopt organizational claims, record human review or attestation, or approve publication.

#### Scenario: Agent attempts attestation
- **WHEN** an agent token calls a human review or publication-approval operation
- **THEN** authorization fails and no human decision is recorded

#### Scenario: Internal review is not independent assessment
- **WHEN** a normal authorized user reviews an assessment
- **THEN** the result is labeled internal review unless that human has a separately administered service-specific independent-assessor designation

### Requirement: Change-aware review validity
Fed SHALL bind review decisions to their dependencies and preserve historical validity while separately computing current re-review needs. Changes to claims, cited evidence revisions, assertions, scope/profile, reporting period, evaluation settings, relevant issue disposition, or freshness SHALL yield explicit comparison reasons. Unrelated new evidence MUST NOT invalidate a review.

#### Scenario: Cited evidence changes
- **WHEN** the working assessment adopts a newer cited artifact revision
- **THEN** the previous review remains bound to the old assessment and the working assessment requires renewed review with a changed-input explanation

#### Scenario: Unrelated collection arrives
- **WHEN** collection adds evidence unrelated to any reviewed dependency
- **THEN** that review remains current unless another dependency or freshness condition changed

### Requirement: Reviewable gaps and package decisions
Fed SHALL permit human review of explicitly incomplete or negative assessments without converting them to supported conclusions. Package approval SHALL bind an immutable manifest and audience projection. Independent assurance and authorization labels MUST require their corresponding recorded authority evidence rather than technical pass status.

#### Scenario: Publishable incomplete assessment
- **WHEN** a reviewer accepts the accuracy of an assessment that documents missing operating records
- **THEN** the review can support publication of an explicitly incomplete report and cannot cause the missing assertions to become supported

#### Scenario: Disclosure changes after approval
- **WHEN** a prepared release's public projection is changed
- **THEN** a new release and approval are required before that projection can be published
