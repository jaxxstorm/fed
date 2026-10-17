## ADDED Requirements

### Requirement: Modern original operator workspace
The application SHALL provide a React/TypeScript operator frontend built with Bun and Vite, served by the Go service in production. It SHALL use an original cohesive visual system with readable typography, consistent components, accessible status colors and responsive navigation inspired by the clarity of the provided references without copying their branding or layout.

#### Scenario: Operator opens a service
- **WHEN** an operator selects a service
- **THEN** the workspace presents stable navigation, profile context, clear page hierarchy and a service-progress link without artifact IDs crowding navigation

### Requirement: Complete operator route coverage
The workspace SHALL cover service selection/onboarding, overview, requirements, evidence, activity/collection, sources, review, settings and exports/history. Each page SHALL explain its relevant completed work, missing work, improvement opportunities and next action. Existing requirement, artifact, run and export deep links SHALL continue to resolve appropriately.

#### Scenario: Navigate from overview to an affected rule
- **WHEN** the operator follows a progress or attention link
- **THEN** the destination retains service context and relevant filters and makes the next useful action apparent

#### Scenario: Direct URL reload
- **WHEN** a bookmarked requirement or collection-run URL is loaded directly
- **THEN** the correct authenticated view loads and API/download/MCP URLs are not intercepted by the frontend fallback

### Requirement: Accessible responsive presentation
The workspace SHALL support keyboard operation, visible focus, semantic labels, text/icon status indicators, reduced motion and layouts at 390px, 768px and 1440px widths, targeting WCAG 2.2 AA. Large datasets SHALL be paginated and technical identifiers SHALL wrap or expand without breaking layout.

#### Scenario: Many artifacts on a narrow screen
- **WHEN** a requirement with hundreds of artifacts is viewed at mobile width
- **THEN** navigation and primary actions remain usable, evidence is paginated and identifiers do not cause page-wide horizontal overflow

### Requirement: Reproducible frontend delivery and browser authentication
Release builds SHALL reproducibly build the frontend from a pinned lockfile and package assets with the Go service without requiring Bun/Node in the production runtime. Browser APIs SHALL use existing sessions, capabilities and CSRF protection; generated TypeScript API contracts SHALL be checked against the Go browser API schema.

#### Scenario: Authenticated mutation
- **WHEN** the browser saves a source or review
- **THEN** the server enforces permissions, CSRF and revision checks through the existing shared domain services

#### Scenario: Missing release assets
- **WHEN** a release build lacks the real compiled frontend bundle
- **THEN** the build fails rather than shipping an empty placeholder application
