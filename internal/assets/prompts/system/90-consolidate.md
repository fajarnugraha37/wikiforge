{{BASE}}

OBJECTIVE
Perform a final whole-system consistency audit and deepen only `openwiki/knowledge/relationships.md`, with small link or terminology corrections elsewhere when necessary.

REQUIRED SECTIONS
# System Knowledge Relationships
## Entity Index
## Relationship Table
## Capability-to-Component Traceability
## Component-to-Contract Traceability
## Application-to-Infrastructure Traceability
## Data and Event Traceability
## Failure and Recovery Traceability
## Canonical Terminology
## Contradictions
## Knowledge Gaps
## Source References

RELATIONSHIP TABLE FORMAT
| Subject | Relationship | Object | Evidence | Authority | Confidence |

FINAL AUDIT
- component identities and types match `sources/manifest.json`
- deployables, modules, libraries, frameworks, contracts, infrastructure, and configuration are described using correct terminology
- producer/consumer, dependency, deployment, configuration, and provisioning directions agree across pages
- system pages link back to source component pages
- diagrams agree with relationship tables
- contradictions are explicit rather than silently resolved
- no unsupported business, ownership, SLO, regulatory, infrastructure, or recovery claims remain
