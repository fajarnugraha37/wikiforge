{{BASE}}

COMPONENT CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}} (`{{PROFILE_ID}}`)
- Repository: {{REPOSITORY}}
- Scoped working directory: {{SCOPE}}

OBJECTIVE
Perform a final documentation consolidation without rewriting accurate content unnecessarily.

OWNED PAGE
Deepen only `openwiki/knowledge/relationships.md`, but make small link, terminology, front matter, or contradiction corrections in other canonical pages when required for consistency.

REQUIRED SECTIONS
## Concept Index
## Relationship Table
## Traceability Paths
## Canonical Terminology
## Contradictions
## Knowledge Gaps
## Source References

RELATIONSHIP TABLE FORMAT
| Subject | Relationship | Object | Evidence | Authority | Confidence |

Use concise uppercase relationships such as OWNS, CONTAINS, DEPENDS_ON, CALLS, PRODUCES, CONSUMES, CONFIGURES, DEPLOYS, PROVISIONS, GENERATES, IMPLEMENTS, EXTENDS, VALIDATED_BY, PROTECTED_BY, or RECOVERED_BY. Use only relationships supported by evidence.

PROFILE GUIDANCE
{{GUIDANCE}}

FINAL AUDIT
- Every canonical page is linked from `quickstart.md`.
- Important concepts have one canonical home.
- Terminology and component names are consistent.
- Mermaid diagrams agree with prose and relationship tables.
- Unsupported claims are removed, qualified, or marked Unknown.
- Source references resolve to real paths inside the component scope or canonical documentation.
- Application-only concepts are not forced onto non-application profiles.
- Monorepo neighbours are not silently treated as part of this scoped component.
