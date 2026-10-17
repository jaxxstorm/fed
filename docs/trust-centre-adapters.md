# Vendor-neutral trust-centre adapter contract

The service runs without a trust-centre account. Its own IDs identify services, evidence, assessments, and immutable snapshots. External URLs can be registered without fetching them and remain unverified references.

Optional adapters advertise independent capabilities: list evidence, retrieve evidence, and publish an assessment bundle. Unsupported operations return `integration_unavailable`. The core must not infer a vendor capability merely because an adapter is configured.

An adapter receives a destination configuration reference and an immutable bundle or external reference. Secrets are resolved within the integration layer, never embedded in evidence or bundles. External object IDs and receipts remain in destination mappings. Changing destinations creates new mappings and preserves previous attempts.

Publication is tracked separately from preparation and export. Only a confirmed external receipt can record publication. Downloading a bundle records export, not publication. Live vendor connectors are follow-up work based on verified APIs; no vendor-specific connector is needed for the initial release.
