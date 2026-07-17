---
type: Reference
title: WikiForge Prompt System
description: Embedded prompt templates, phase contracts, specialized catalog supplements, and system-level prompt rendering
tags: [prompts, templates, phases, contracts]
resource: /internal/prompts/profiles.go
---

# Prompt System

WikiForge renders [OpenWiki](https://github.com/fajarnugraha37/OpenWiki) prompts from embedded templates and feeds them to the OpenWiki executor through the [prompt bridge](../integrations/openwiki-bridge.md). The prompt system is defined in [`internal/prompts/`](/internal/prompts/) with templates in [`internal/assets/prompts/`](/internal/assets/prompts/).

## Prompt architecture

### Base template

Located at [`internal/assets/prompts/common/base.md`](/internal/assets/prompts/common/base.md). Every phase prompt starts with `{{BASE}}`, which expands to the common instructions: evidence rules, writing conventions, front matter requirements, and documentation language.

### Repair template

Located at [`internal/assets/prompts/common/repair.md`](/internal/assets/prompts/common/repair.md). Used during repair rounds. Receives validator findings as input and instructs OpenWiki to fix only the affected files.

## Component prompts

### Bootstrap/initialize

[`internal/assets/prompts/component/initialize.md`](/internal/assets/prompts/component/initialize.md) — Runs in the first phase (`{PREFIX}00`). Bootstraps OpenWiki state for the component and creates a focused first-pass `quickstart.md`. Must not attempt to generate all canonical pages.

### Phase prompt

[`internal/assets/prompts/component/phase.md`](/internal/assets/prompts/component/phase.md) — Used by all core phases (`{PREFIX}10` to `{PREFIX}70`). Includes:

- `{{OBJECTIVE}}` — Phase purpose
- `{{OUTPUT_FILE}}` — The owned canonical page
- `{{REQUIRED_HEADINGS}}` — Exact section headings to include
- `{{DIAGRAM_CONTRACT}}` — Required Mermaid diagram type
- `{{PROFILE_GUIDANCE}}` — Profile-specific writing direction
- `{{SUPPLEMENTAL_CONTRACTS}}` — Additional page contracts for specialized catalogs

### Supplemental batch prompt

[`internal/assets/prompts/component/supplemental.md`](/internal/assets/prompts/component/supplemental.md) — Used for specialized catalog phases (`{PREFIX}S01`–`{PREFIX}SNN`). Each batch generates up to 4 specialized pages with exact table-header contracts.

### Consolidate prompt

[`internal/assets/prompts/component/consolidate.md`](/internal/assets/prompts/component/consolidate.md) — Used in the final phase (`{PREFIX}C90`). Performs consistency, navigation, terminology, evidence, diagram, and relationship audit across all component pages.

### Update prompt

[`internal/assets/prompts/component/update.md`](/internal/assets/prompts/component/update.md) — Used by `wikiforge update`. Instructs OpenWiki to refresh every affected canonical page rather than only the relationship page.

## System prompts

System prompts mirror the component structure but operate on aggregated component documentation:

- `00-initialize.md` — Bootstrap system workspace
- `05-quickstart.md` — System overview
- `10-system-overview.md` — System landscape
- `20-component-map.md` — Component catalog and dependencies
- `30-cross-component-flows.md` — Cross-component journey flows
- `40-data-events-contracts.md` — Data, events, and contract aggregation
- `45-infrastructure-deployment.md` — Infrastructure topology
- `50-failure-and-security.md` — Failure, security, and operations
- `55-specialized-catalogs.md` — System-level catalog batches
- `60-onboarding-change.md` — Onboarding and change guide
- `90-consolidate.md` — System relationship audit
- `update.md` — System update prompt

All system templates are in [`internal/assets/prompts/system/`](/internal/assets/prompts/system/).

## Instructions template

[`internal/assets/templates/instructions.md`](/internal/assets/templates/instructions.md) generates the persistent `openwiki/INSTRUCTIONS.md` for each component. This is written once during bootstrap and contains:

- Component identity (ID, type, profile, repository, scope)
- Audience definition (engineers, maintainers, agents, system aggregation)
- Evidence and authority classification rules
- Canonical page roadmap
- Profile-specific writing direction
- Evidence and writing quality rules

`internal/assets/templates/system-instructions.md` generates the equivalent for the whole-system wiki.

## Specialized catalog contracts

Specialized pages are documented in [`DOCUMENTATION-CATALOG.md`](/DOCUMENTATION-CATALOG.md) and defined in [`internal/prompts/supplements.go`](/internal/prompts/supplements.go). Each page has:

- **Exact path** (e.g., `configuration/runtime-configuration.md`)
- **Objective** — One-sentence purpose
- **Required diagram** — Mermaid type
- **Required table header** — Exact column header string for the catalog table
- **Required headings** — Complete list of required section headings

### Component specialized pages (22 total)

Covering: runtime configuration, service-to-service integrations, external services, cloud services, dependency matrix, endpoint catalog, event catalog, job catalog, business flows, rules/validation, business data, traffic flows, request flows, authentication, authorization, concurrency, async processing, context propagation, database structure, database programmability, cryptography, file handling.

### System specialized pages (17 total)

Aggregate versions: dependency matrix, endpoint catalog, event catalog, job catalog, business flow/rules/data, traffic flows, request flows, authentication, authorization, concurrency, async processing, context propagation, database structures, configuration/secrets, cloud services, cryptography/key management, file handling.

### Profile-specific coverage

Each profile selects a subset of specialized pages. `application`, `modular-application`, and `generic` profiles get all 22. `reusable` gets 19 (fewer database/storage pages). `infrastructure` gets 15. `configuration` gets 11. `contracts` gets 10.

## Prompt rendering

[`internal/prompts/prompts.go`](/internal/prompts/prompts.go) handles rendering:

1. `RenderComponentPhase` — Renders a phase prompt for a component, merging profile contract, component context, guidance, and supplemental contracts.
2. `RenderSystemPhase` — Renders a phase prompt for the whole-system wiki.
3. `RenderComponentUpdate` / `RenderSystemUpdate` — Renders update prompts.
4. `RenderInstructions` — Renders the persistent INSTRUCTIONS.md.
5. `Render` — Core function that loads the prompt asset, renders Go template with substitutions, and returns the complete prompt string.

## Template variables

| Variable | Component | System | Description |
|---|---|---|---|
| `PROFILE_ID` | Yes | No | Profile identifier |
| `PROFILE_NAME` | Yes | No | Profile display name |
| `COMPONENT_TYPE` | Yes | No | Component class |
| `REPOSITORY` | Yes | No | Repo path |
| `SCOPE` | Yes | No | Repo subdirectory |
| `CANONICAL_FILES` | Yes | No | Expected page roadmap |
| `OUTPUT_FILE` | Yes | No | Owned page for this phase |
| `OBJECTIVE` | Yes | No | Phase purpose |
| `REQUIRED_HEADINGS` | Yes | No | Required section headings |
| `DIAGRAM_CONTRACT` | Yes | No | Required Mermaid type |
| `GUIDANCE` | Yes | No | Profile-specific direction |
| `SUPPLEMENTAL_CONTRACTS` | Yes | Yes | Specialized page contracts |
| `BASE` | Yes | Yes | Common instructions |
| `SYSTEM_CANONICAL_FILES` | No | Yes | System page roadmap |

## Source map

| File | Role |
|---|---|
| `/internal/prompts/profiles.go` | Profile definitions with phase contracts |
| `/internal/prompts/prompts.go` | Prompt rendering, system phases, update prompts |
| `/internal/prompts/supplements.go` | Specialized catalog page contracts |
| `/internal/prompts/supplements_test.go` | Supplement contract tests |
| `/internal/prompts/profiles.go` | Profile IDs listing |
| `/internal/assets/prompts/common/base.md` | Shared base prompt template |
| `/internal/assets/prompts/common/repair.md` | Repair round prompt template |
| `/internal/assets/prompts/component/*.md` | Component-phase prompt templates |
| `/internal/assets/prompts/system/*.md` | System-phase prompt templates |
| `/internal/assets/templates/instructions.md` | Component INSTRUCTIONS.md template |
| `/internal/assets/templates/system-instructions.md` | System INSTRUCTIONS.md template |
| `/DOCUMENTATION-CATALOG.md` | Canonical list of all specialized page contracts |
