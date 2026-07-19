---
type: Reference
title: WikiForge Adaptive Prompt System
description: Embedded prompt templates, adaptive page contracts, profile guidance, and prompt rendering for evidence-driven documentation generation
tags: [prompts, templates, adaptive, rendering]
resource: /internal/prompts/adaptive.go
---

# Adaptive Prompt System

WikiForge renders [OpenWiki](https://github.com/fajarnugraha37/OpenWiki) prompts from embedded templates and feeds them to the OpenWiki executor through the [prompt bridge](../integrations/openwiki-bridge.md). The prompt system is defined in [`internal/prompts/`](/internal/prompts/) with templates in [`internal/assets/prompts/`](/internal/assets/prompts/).

## Prompt architecture

The prompt system has been redesigned around **adaptive page generation**. Instead of fixed phase prompts per profile, WikiForge uses a small set of generic templates that receive page-specific contracts determined by the adaptive planner.

### Template inventory

| Template | Path | Purpose |
|---|---|---|
| Base | `prompts/common/base.md` | Shared operating rules — evidence rules, writing conventions, front matter requirements, documentation language. Every prompt starts with this. |
| Component phase | `prompts/component/phase.md` | Generate or deepen a single adaptive page for a component. Receives objective, required headings, diagram contract, profile guidance, unit context, and adaptive view/pack metadata. |
| Component update | `prompts/component/update.md` | Incrementally update all affected canonical pages. Receives profile context, full canonical page set, and adaptive view data. |
| System adaptive page | `prompts/system/adaptive-page.md` | Generate or update a single whole-system wiki page with view, owner, and navigation contract. |
| Repair | `prompts/common/repair.md` | Targeted repair round using validator findings. Receives findings text, profile context, and scoped instructions. |

The following prompt templates from the earlier fixed-phase system have been removed: `initialize.md`, `supplemental.md`, `consolidate.md`, all per-phase system prompts, and `system-instructions.md`.

## Adaptive page contracts

[`internal/prompts/adaptive.go`](/internal/prompts/adaptive.go) provides `AdaptivePageContract`, which supplies a small typed contract for pages selected by the adaptive planner:

```go
func AdaptivePageContract(path, kind string) model.PageContract
```

The contract shape varies by page kind:

| Kind | Required headings | Required diagram |
|---|---|---|
| `single` | Purpose, Knowledge Gaps, Source References | flowchart |
| `index` | Navigation, Knowledge Gaps, Source References | (none) |
| `collection` | Navigation, Knowledge Gaps, Source References | (none) |
| `shard` | Purpose, Knowledge Gaps, Source References | flowchart |
| Catalog pages (`catalogs/`)* | Catalog Scope, Catalog Entries, Knowledge Gaps, Source References | (none) |

*Catalog pages additionally check a `RequiredTableHeader` of `| ID | Name | Direction | Owner | Evidence |`.

## Prompt rendering

[`internal/prompts/prompts.go`](/internal/prompts/prompts.go) provides the core rendering engine:

1. **`Render(assetPath, language, targetID, values)`** — Loads the base template, substitutes values using simple `{{VAR}}` replacement (not Go text/template), appends the body template, and returns the complete prompt string.
2. **`RenderTemplate` / `RenderTemplateValues`** — Renders non-base templates (e.g., instructions without the base prefix).
3. **`RenderAdaptivePage`** — Renders a component adaptive page with objective, headings, diagram, guidance, unit context, and adaptive view/pack data.
4. **`RenderAdaptiveInstructions`** — Renders the persistent `openwiki/INSTRUCTIONS.md` for a component.
5. **`RenderAdaptiveUpdate`** — Renders the update prompt for incremental regeneration of affected pages.
6. **`RenderAdaptiveSystemPage`** — Renders a whole-system page with system-level context and navigation contracts.

## Profile guidance

Profiles are now **identity metadata** rather than phase contracts. Seven profiles exist (`application`, `modular-application`, `reusable`, `infrastructure`, `configuration`, `contracts`, `generic`), each providing targeted writing direction in `profileGuidance()`. Profile definitions are in [`internal/prompts/profiles.go`](/internal/prompts/profiles.go).

## Instructions template

[`internal/assets/templates/instructions.md`](/internal/assets/templates/instructions.md) generates the persistent `openwiki/INSTRUCTIONS.md` for each component. It is written once during the first bootstrap phase and reused for updates. The old `templates/system-instructions.md` has been removed.

## Documentation catalog

[`DOCUMENTATION-CATALOG.md`](/DOCUMENTATION-CATALOG.md) documents the canonical page ownership model:

- **View hierarchy** — System, domain, component, flow, catalog, platform, engineering, operations views with canonical locations.
- **Component pages** — Index, architecture, contracts, data-and-consistency, runtime-and-operations under `components/<component>/`.
- **Domain and flow pages** — Per-domain and per-flow pages plus shared indexes.
- **Catalog collections** — Typed collections sharded by pack (interfaces, events, jobs, caches, rate-limits, etc.).

## Template variables

| Variable | Component | System | Description |
|---|---|---|---|
| `BASE` | Yes | Yes | Common operating rules |
| `PROFILE_ID` | Yes | No | Profile identifier |
| `PROFILE_NAME` | Yes | No | Profile display name |
| `COMPONENT_TYPE` | Yes | No | Component class |
| `REPOSITORY` | Yes | No | Repository virtual root path |
| `SCOPE` | Yes | No | Repo subdirectory |
| `OUTPUT_FILE` | Yes | Yes | Owned page for this phase |
| `OBJECTIVE` | Yes | Yes | Page purpose |
| `REQUIRED_HEADINGS` | Yes | Yes | Required section headings |
| `DIAGRAM_CONTRACT` | Yes | Yes | Required Mermaid type |
| `GUIDANCE` | Yes | Yes | Profile or view-specific direction |
| `ADAPTIVE_VIEWS` | Yes | Yes | Active documentation views |
| `ADAPTIVE_PACKS` | Yes | No | Active capability packs |
| `ADAPTIVE_OWNER` | No | Yes | Page owner unit |
| `UNIT_CONTEXT` | Yes | No | Owner unit context description |
| `CANONICAL_FILES` | Yes | Yes | Full page roadmap |
| `NAVIGATION_RULE` | No | Yes | Link hierarchy contract |

## Source map

| File | Role |
|---|---|
| `/internal/prompts/profiles.go` | Profile identity definitions |
| `/internal/prompts/prompts.go` | Core prompt rendering + helper functions |
| `/internal/prompts/adaptive.go` | Adaptive page contracts, rendering, instructions, system pages |
| `/internal/prompts/adaptive_test.go` | Adaptive prompt rendering tests |
| `/internal/assets/prompts/common/base.md` | Shared base prompt template |
| `/internal/assets/prompts/common/repair.md` | Repair round prompt template |
| `/internal/assets/prompts/component/phase.md` | Component adaptive page prompt template |
| `/internal/assets/prompts/component/update.md` | Component update prompt template |
| `/internal/assets/prompts/system/adaptive-page.md` | System adaptive page prompt template |
| `/internal/assets/templates/instructions.md` | Component INSTRUCTIONS.md template |
| `/DOCUMENTATION-CATALOG.md` | Canonical list of adaptive page contracts and catalog collections |
