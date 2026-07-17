{{BASE}}

OBJECTIVE
Deepen only `openwiki/system/component-map.md`.

REQUIRED SECTIONS
# Component Map
## Component Catalog
## Deployable Components
## Modular Applications
## Libraries and Frameworks
## Infrastructure and Configuration Components
## Contract and Schema Components
## Dependency Relationships
## External Systems
## Shared Foundations
## Dependency Risks
## Contradictions
## Knowledge Gaps
## Source References

DIAGRAM CONTRACT
Include one Mermaid `flowchart LR` showing verified dependency direction across component categories. Split into focused diagrams only if readability would otherwise be poor.

Use the component manifest under `sources/manifest.json` as the canonical identity and type registry. Do not infer that every relationship is a runtime call; distinguish USES, DEPENDS_ON, DEPLOYS, CONFIGURES, PROVISIONS, PRODUCES, and CONSUMES when evidence supports them.
