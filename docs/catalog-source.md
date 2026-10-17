# Official catalog contract

Inspected source: FedRAMP/rules commit `a89d78477ddea98dc57eca24f5007794c2b4eeb6`, version `2026.10.13.01`, updated 2026-10-13.

| File | SHA-256 |
| --- | --- |
| `fedramp-consolidated-rules.json` | `fb48d677a85d77fb72735a325de2c1945a1e0ceab1d2e655292fe9ebe3e4e0b4` |
| `schemas/fedramp-consolidated-rules.schema.json` | `eb4710bee0b06d0f8f803302ac2e88b934ef2bc942d0b548738c7304f0cb418f` |

The complete payload passes its pinned Draft 2020-12 schema, including format checking. Fetch both files from `https://raw.githubusercontent.com/FedRAMP/rules/<commit>/` and retain the exact bytes. Fixtures should be extracted from this payload, with source pointers, rather than invented statements.

## Mapping

- `info`: version, update date, default artifact expectations.
- `FRD`: definitions and their source metadata. Preserve even when not individually normalized.
- `FRR.<document>.info`: name, web slug, status, global or type-specific effective metadata and subset definitions.
- `FRR.<document>.data.<all|20x|rev5>.<subset>.<id>`: individual rules. Join to `info.subsets` and type-specific `info.<type>.subsets` for types, paths, classes, and affected audiences.
- Rules have either `statement` and `force`, or `varies_by_class.<a|b|c|d>`. Preserve MUST NOT and SHOULD NOT, not merely their obligation category.
- `effective.date`: obtain/maintain/optional-adoption dates and grace `{default, until_next_assessment}`. Rule and class-level `effective_date` can override dates. Ongoing grace resolution needs assessment context; date alone may be insufficient.
- `related` is a relationship, not automatically a mandatory dependency. Class A's `FRC-CLA-MFR`, `FRC-CLA-RFR`, and `FRC-CLA-OFR` specifically enumerate required, recommended, and optional obligations in `following_information` and `related`.
- `KSI.<theme>.indicators.<id>`: common statement or class variants, controls, artifacts, terms, and updates. There is no KSI-wide `force` or uniform applicability object. Some Class B variants encode `**Optional:**` in their statement.
- `CTL`: control guidance; preserve raw data rather than misclassifying it as a KSI.

## Real-source fixture cases

- `FRC-CLA-MFR` references `KSI-SVC-SIN` and `CDS-CSO-UTC` despite their absence from some Class A subset metadata. Preserve the inclusion relationship.
- `FRC-CSX-VVK` has A/B/C/D variants but its subset lists B/C/D. Keep the conflict visible when not explicitly resolved by another rule.
- `KSI-SVC-PRR`: optional Class B text, Class C text, no explicit D variant. Do not invent a D fallback.
- `SDR-CSX-KMT`: Class B SHOULD and Class C MUST, with a pilot-dependent D statement.
- `FRC` dates differ for obtain (2026-07-04) and maintain (2027-01-01); grace depends on the next assessment.

Narrative citations resolve into `2026-markdown/providers/20x/rules/<web_name>.md` and `key-security-indicators/<web_name>.md`. The original JSON remains authoritative for machine fields. Conditional prose not implemented by a verified resolver must remain unresolved with its source explanation.
