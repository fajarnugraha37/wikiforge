<!-- WIKIFORGE:START -->
# WikiForge Component Documentation Contract

This wiki is generated and maintained through phased OpenWiki runs orchestrated by WikiForge.

## Component identity

- Identifier: `{{TARGET_ID}}`
- Type: `{{COMPONENT_TYPE}}`
- Documentation profile: `{{PROFILE_NAME}}` (`{{PROFILE_ID}}`)
- Repository: `{{REPOSITORY}}`
- Scope: `{{SCOPE}}`

{{PROFILE_DESCRIPTION}}

## Audience

The documentation must support:

- engineers joining with no prior context;
- maintainers reviewing high-risk changes;
- operators or consumers appropriate to this component type;
- LLM coding agents that need reliable context before editing;
- whole-system aggregation and knowledge-graph extraction.

## Evidence and authority

Use current repository evidence, tests, configuration, contracts, infrastructure definitions, deployment files, existing documentation, and Git history. Never invent missing facts.

Classify important information as **Verified**, **Derived**, **Unknown**, or **Conflicting**. Current implementation is evidence of observed behaviour, not automatically authoritative intent.

## Canonical pages

{{CANONICAL_FILES}}

## Profile-specific direction

{{GUIDANCE}}

## Writing rules

- Give each concept one canonical home and link to it elsewhere.
- Cite concrete source paths for significant claims.
- Include failure behaviour, edge cases, compatibility, and safe-change guidance appropriate to the selected profile.
- Use Mermaid only for evidence-backed diagrams and explain each diagram in prose.
- Keep unresolved gaps and contradictions explicit.
- Never expose secrets, credentials, private keys, or production personal data.
- Stay within the configured repository scope unless a referenced neighbour is required to explain an interaction.

Documentation language: {{LANGUAGE}}
<!-- WIKIFORGE:END -->
