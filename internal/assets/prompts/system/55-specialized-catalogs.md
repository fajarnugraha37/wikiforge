{{BASE}}

OBJECTIVE
Create or deepen the whole-system specialized catalogs below using component wiki snapshots under `sources/components/`, the component manifest, and optional authoritative facts. Do not inspect or invent raw component implementation that is not represented in those sources.

SYSTEM SPECIALIZED PAGE CONTRACTS
{{SYSTEM_SUPPLEMENTAL_CONTRACTS}}

AGGREGATION RULES
- Preserve component identity, type, profile, and source-document links.
- Aggregate only evidence-backed rows. When no component provides an item, keep the page and state `Not Observed` or `Unknown`.
- Do not infer a runtime dependency from a build dependency, shared library, configuration relationship, deployment relationship, or contract relationship.
- Preserve direction for calls, events, file transfers, configuration, data ownership, provisioning, and deployment.
- Surface conflicting endpoint names, event semantics, authentication assumptions, database ownership, schedules, cloud resources, cryptographic mechanisms, and file formats.
- Never copy secret values, credentials, private keys, tokens, or production personal data.
- Link each aggregate row back to the component documentation that supports it.
- Use exactly the required catalog header and required section names.
- End every page with `## Knowledge Gaps` and `## Source References`.

STOP CONDITION
All specialized system pages exist, pass their structural contracts, preserve direction and component categories, link to supporting component pages, and distinguish Verified, Derived, Unknown, and Conflicting evidence.
