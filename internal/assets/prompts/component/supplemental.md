{{BASE}}

COMPONENT CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}} (`{{PROFILE_ID}}`)
- Repository: {{REPOSITORY}}
- Scoped working directory: {{SCOPE}}

OBJECTIVE
Create or deepen every specialized catalog below. These pages are first-class documentation contracts, not optional notes.

SPECIALIZED PAGE CONTRACTS
{{SUPPLEMENTAL_CONTRACTS}}

EVIDENCE AND APPLICABILITY RULES
- Inspect repository evidence specifically for each page.
- When evidence exists, document concrete identifiers, directions, protocols, mechanisms, limits, failures, security, ownership, and source paths.
- When no verified evidence exists, still create the page, preserve every required section, and state `Not Observed` or `Unknown` precisely. Do not fabricate entries merely to fill a catalog.
- A page with no observed entries must explain what evidence was searched and what remains unknown.
- Do not expose secret values, credentials, private keys, tokens, personal data, or live production identifiers. Document only logical secret references, stores, consumers, rotation evidence, and access boundaries.
- Distinguish internal service calls, external parties, managed cloud services, build-time dependencies, in-memory events, and runtime broker messages.
- Distinguish business data concepts from physical database structures.
- Distinguish database programmable objects from application methods with similar names.
- Distinguish authentication from authorization while documenting their end-to-end interaction in the merged page.
- Distinguish file transport/storage from file format/schema while documenting both in the merged file page.
- Do not duplicate prose already owned by a core page. Link to the canonical detailed page instead.

CATALOG RULES
- Preserve the exact required table header for every specialized page.
- Use one row per evidence-backed item. Use explicit `Unknown` cells instead of guesses.
- Give stable IDs to endpoints, events, jobs, flows, and rules where the repository provides no existing identifier.
- Include direction, producer/caller, consumer/callee, protocol or mechanism, failure behaviour, and evidence wherever relevant.
- End every page with `## Knowledge Gaps` and `## Source References`.

STOP CONDITION
Every specialized page exists, satisfies all required headings, table, diagram, front matter, links, and evidence rules, including honest Not Observed/Unknown results where the repository contains no applicable implementation.
