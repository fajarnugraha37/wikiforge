{{BASE}}

COMPONENT CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}} (`{{PROFILE_ID}}`)
- Repository: {{REPOSITORY}}
- Scoped working directory: {{SCOPE}}

OBJECTIVE
Incrementally update every canonical page affected by current repository changes. Do not limit the update to the relationship page.

CANONICAL PAGE SET
{{CANONICAL_FILES}}

SPECIALIZED PAGE CONTRACTS
{{SUPPLEMENTAL_CONTRACTS}}

UPDATE RULES
- Inspect current repository evidence and existing wiki content.
- Update only pages whose claims, catalogs, diagrams, links, risks, or source references are affected.
- Create any missing canonical page and satisfy its complete contract.
- Remove or qualify stale claims that no longer match the repository.
- Preserve exact required catalog headers in specialized pages.
- Keep explicit `Not Observed`, `Unknown`, and `Conflicting` statuses when evidence remains incomplete.
- Update `quickstart.md` navigation and `knowledge/relationships.md` whenever page inventory or relationships change.
- Never expose secret values or modify product source.

STOP CONDITION
All affected canonical pages accurately reflect current evidence and remain structurally valid, linked, and internally consistent.
