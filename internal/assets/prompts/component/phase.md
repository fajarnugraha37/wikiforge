{{BASE}}

COMPONENT CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}} (`{{PROFILE_ID}}`)
- Repository: {{REPOSITORY}}
- Scoped working directory: {{SCOPE}}

OBJECTIVE
{{OBJECTIVE}}

OWNED PAGE
Deepen only `openwiki/{{OUTPUT_FILE}}`. You may make small link corrections elsewhere, but do not rewrite unrelated canonical pages.

REQUIRED SECTIONS
{{REQUIRED_HEADINGS}}

DIAGRAM CONTRACT
{{DIAGRAM_CONTRACT}}

PROFILE GUIDANCE
{{GUIDANCE}}

DEPTH AND QUALITY REQUIREMENTS
- Start from the current page and preserve accurate content.
- Inspect the most relevant repository evidence for this page rather than sampling unrelated files.
- Explain concrete control flow, data flow, lifecycle, state, dependency, compatibility, or operational consequences as appropriate to the profile.
- Include failure paths, invalid inputs, unsafe changes, and edge cases when evidence exists.
- Use specific file paths, symbols, commands, schemas, manifests, modules, tests, or generated artifacts as evidence.
- Clearly mark Verified, Derived, Unknown, and Conflicting conclusions where the distinction matters.
- Avoid generic software-engineering advice that is not grounded in this repository.
- End with `## Knowledge Gaps` and `## Source References`.

STOP CONDITION
The owned page satisfies every required section and diagram contract, gives actionable safe-change context, and contains no unsupported claims.
