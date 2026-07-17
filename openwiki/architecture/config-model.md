---
type: Reference
title: WikiForge Configuration Model
description: YAML/JSON configuration schema, component types, profile selection, path normalization, and validation rules
tags: [configuration, yaml, components, profiles]
resource: /internal/config/config.go
---

# Configuration Model

WikiForge uses a YAML or JSON configuration file (default `wikiforge.yaml`) with a version 2 schema and backward-compatible v1 migration.

## Config structure

The top-level [`Config` struct](/internal/config/config.go) has these sections:

```yaml
version: 2                # 1 or 2 (v1 services are migrated to components)
workspace: .              # working directory root
openwiki: { ... }         # OpenWiki command, args, timeout, environment
execution: { ... }        # parallelism, retries, repair rounds, failure policy
documentation: { ... }    # quality thresholds, validation toggles
mermaid: { ... }          # Mermaid CLI config (render/basic/off mode)
components: [ ... ]       # component definitions
system: { ... }           # whole-system aggregation config
```

### OpenWiki config

| Field | Default | Description |
|---|---|---|
| `command` | `npx` | OpenWiki executable |
| `args` | `["--yes", "openwiki@0.2.0", "code"]` | CLI arguments |
| `modelId` | `""` | Override OpenWiki model ID |
| `timeoutMinutes` | `60` | Per-phase timeout |
| `environment` | `{OPENWIKI_TELEMETRY_DISABLED: "1", OPENWIKI_PROVIDER_RETRY_ATTEMPTS: "3"}` | Extra env vars |

### Execution config

| Field | Default | Description |
|---|---|---|
| `parallelComponents` | `2` | Max concurrent component groups |
| `maxProcessRetries` | `2` | Max retries per phase process |
| `maxRepairRounds` | `2` | Max validation/repair cycles |
| `continueOnComponentFailure` | `true` | Keep going after component failure |

### Documentation config

| Field | Default | Description |
|---|---|---|
| `language` | `English` | Documentation language |
| `minimumQualityScore` | `85` | Minimum validation score to accept |
| `requireFrontMatter` | `true` | Enforce OpenWiki front matter |
| `requireSourceReferences` | `true` | Enforce source reference sections |
| `validateSourcePaths` | `true` | Verify source paths resolve in scope |
| `allowedDiagramTypes` | 9 types | Permitted Mermaid diagram types |

### Mermaid config

| Field | Default | Description |
|---|---|---|
| `mode` | `render` | `render` (CLI render), `basic` (offline checks), or `off` |
| `command` | `npx` | Mermaid CLI command |
| `args` | `["--yes", "@mermaid-js/mermaid-cli@11.12.0", "-i", "{input}", "-o", "{output}", "--quiet"]` | Mermaid args with `{input}`/`{output}` placeholders |
| `timeoutSeconds` | `90` | Per-diagram render timeout |

## Component configuration

```yaml
components:
  - id: order-service
    type: microservice
    repository: ./repositories/order-service
    enabled: true
    group: commerce
    tags: [order, deployable]
    dependsOn: [shared-contracts]
    scope: ""                       # empty = repository root
    includeInSystem: true           # default: true
```

### Component types and profile mapping

| Type | Profile | Description |
|---|---|---|
| `monolith`, `microservice`, `worker`, `gateway`, `frontend`, `cli` | `application` | Deployable application |
| `modular-monolith` | `modular-application` | Modular monolith with module docs |
| `library`, `shared-library`, `internal-library`, `framework`, `sdk` | `reusable` | Library, SDK, framework |
| `iac`, `infrastructure`, `gitops`, `deployment`, `platform` | `infrastructure` | IaC, GitOps, platform |
| `configuration`, `shared-config`, `config` | `configuration` | Shared config/policy |
| `contract`, `contracts`, `schema`, `schemas` | `contracts` | API/event/data schemas |
| `generic`, `repository` | `generic` | Unclassified fallback |

## Profiles

Seven documentation profiles each define a set of phases with required output files, sections, and diagram types. Profile definitions are in [`internal/prompts/profiles.go`](/internal/prompts/profiles.go).

### Phase structure

Each phase has:
- **ID** (e.g., `A10`, `M25`, `I70`)
- **Name** (e.g., "Architecture", "Domain behaviour")
- **Output file** (e.g., `architecture/overview.md`)
- **Objective** — Describes the phase purpose
- **Required headings** — Exact section headings to include
- **Required diagram type** — Mermaid diagram contract (`flowchart`, `sequenceDiagram`, `erDiagram`, `classDiagram`, or `any`)
- **Page contracts** — For specialized catalog phases

### Phase IDs by profile

Every profile follows the pattern:
- `{PREFIX}00` — Bootstrap and quickstart
- `{PREFIX}10–{PREFIX}70` — Core phases
- `{PREFIX}S01–{PREFIX}SNN` — Specialized catalog batches (inserted before consolidate)
- `{PREFIX}C90` — Consolidate/relationship audit

Prefixes: A (application), M (modular-application), R (reusable), I (infrastructure), C (configuration), K (contracts), G (generic).

### Profile contracts

| Profile | Core pages | Specialized pages | Total canonical pages | Minimum Mermaid blocks |
|---|---|---|---|---|
| `application` | 8 | 22 | 30 | 5 |
| `modular-application` | 9 | 22 | 31 | 6 |
| `reusable` | 8 | 19 | 27 | 5 |
| `infrastructure` | 8 | 15 | 23 | 5 |
| `configuration` | 8 | 11 | 19 | 5 |
| `contracts` | 8 | 10 | 18 | 5 |
| `generic` | 8 | 22 | 30 | 5 |

## Path normalization

When loading a configuration, WikiForge normalizes all paths:

1. **Workspace** — Resolved to absolute path relative to config file directory.
2. **Repository** — Same resolution, supports repo-relative and absolute paths.
3. **Scope** — Normalized with `pathutil.NormalizeRelative`: accepts `/` or `\`, rejects absolute paths and parent escapes.
4. **System output** — Resolved to absolute path.
5. **Facts path** — Resolved if non-empty.

The `ComponentConfig.WorkDir()` method returns `{repository}/{scope}` (or just `{repository}` if scope is empty). The `DocumentationRoot()` method returns `{workdir}/openwiki`.

## Configuration validation

[`config.Validate()`](/internal/config/config.go) checks:

- Version is 1 or 2
- All components have non-empty IDs
- OpenWiki command is non-empty
- Documentation minimum quality score is 0–100
- Mermaid timeout is positive
- Component IDs are portable path segments (no spaces, special chars, etc.)

## V1 backward compatibility

Configs with `version: 1` or legacy `services` array are migrated:
- `version: 0` (unset) → treated as v1
- Each `services[].{id, path, enabled}` is converted to `components[]` with `type: microservice`

## Source map

| File | Role |
|---|---|
| `/internal/config/config.go` | Config struct, defaults, loading, validation, normalization |
| `/internal/config/yaml.go` | Custom indentation-based YAML subset parser |
| `/internal/config/config_test.go` | Config loading and normalization tests |
| `/schema/wikiforge-config.schema.json` | JSON Schema |
| `/wikiforge.example.yaml` | Example config with all component types |
| `/internal/prompts/profiles.go` | All 7 profile definitions with phase contracts |
| `/internal/prompts/supplements.go` | Specialized catalog page contracts |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization utilities |
