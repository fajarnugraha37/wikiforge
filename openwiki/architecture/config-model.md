---
type: Reference
title: WikiForge Configuration Model
description: YAML/JSON configuration schema (v3), component types, documentation units, evidence config, adaptive planning, and validation rules
tags: [configuration, yaml, components, profiles]
resource: /internal/config/config.go
---

# Configuration Model

WikiForge uses a YAML or JSON configuration file (default `wikiforge.yaml`) with a version 3 schema. Version 1 and 2 configs are migrated on load (v1 `services` → `components`, v2 kept as-is).

## Config structure

The top-level [`Config` struct](/internal/config/config.go) has these sections:

```yaml
version: 3                # 1, 2, or 3 (v1/v2 migrated on load)
workspace: .              # working directory root
openwiki: { ... }         # OpenWiki command, args, timeout, environment
execution: { ... }        # parallelism, retries, repair rounds, failure policy, isolation
documentation: { ... }    # quality thresholds, views, catalogs, evidence config
mermaid: { ... }          # Mermaid CLI config (render/basic/off mode)
components: [ ... ]       # component definitions
documentationUnits: [ ... ] # explicit documentation unit definitions
system: { ... }           # whole-system aggregation config
```

### OpenWiki config

| Field | Default | Description |
|---|---|---|
| `command` | `npx` | OpenWiki executable |
| `args` | `["--yes", "openwiki@0.2.0", "code"]` | CLI arguments |
| `modelId` | `""` | Override OpenWiki model ID |
| `timeoutMinutes` | `60` | Per-phase timeout |
| `maxCaptureBytes` | `262144` | Maximum bytes captured from OpenWiki stdout/stderr |
| `logDirectory` | `""` | Optional directory for complete process logs |
| `environment` | `{OPENWIKI_TELEMETRY_DISABLED: "1", OPENWIKI_PROVIDER_RETRY_ATTEMPTS: "3"}` | Extra env vars |

### Execution config

| Field | Default | Description |
|---|---|---|
| `parallelComponents` | `2` | Max concurrent component groups |
| `maxProcessRetries` | `2` | Max retries per phase process |
| `maxRepairRounds` | `2` | Max validation/repair cycles |
| `continueOnComponentFailure` | `true` | Keep going after component failure |
| `isolateSameRepository` | `true` | Execute same-repo components in isolated working directories |

### Documentation config

| Field | Default | Description |
|---|---|---|
| `language` | `English` | Documentation language |
| `minimumQualityScore` | `85` | Minimum validation score to accept |
| `minimumPages` | `0` | Minimum pages (0=planner defaults) |
| `requireFrontMatter` | `true` | Enforce OpenWiki front matter |
| `requireSourceReferences` | `true` | Enforce source reference sections |
| `validateSourcePaths` | `true` | Verify source paths resolve in scope |
| `requireMermaid` | `true` | Enforce Mermaid diagrams |
| `minimumMermaidBlocks` | `0` | Minimum Mermaid blocks (0=planner defaults) |
| `allowedDiagramTypes` | 8 types | Permitted Mermaid diagram types |
| `views` | `[]` | Active documentation views |
| `catalogs` | `{...}` | Catalog sharding and size limits |
| `evidence` | `{...}` | Evidence index config |
| `frontMatterPolicy` | `namespaced` | Front matter field policy |
| `requireVerifiedEvidence` | `true` | Require verified evidence sources |
| `requireCatalogIdentity` | `true` | Require stable catalog IDs |
| `requireRelationshipEvidence` | `true` | Require evidence for relationship edges |

### Mermaid config

| Field | Default | Description |
|---|---|---|
| `mode` | `render` | `render` (CLI render), `basic` (offline checks), or `off` |
| `command` | `npx` | Mermaid CLI command |
| `args` | `["--yes", "@mermaid-js/mermaid-cli@11.12.0", "-i", "{input}", "-o", "{output}", "--quiet"]` | Mermaid args with `{input}`/`{output}` placeholders |
| `timeoutSeconds` | `90` | Per-diagram render timeout |
| `cacheDirectory` | `""` | Optional Mermaid render cache directory |
| `maxWorkers` | `2` | Maximum concurrent Mermaid render workers |

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
    owners: [platform-team]         # ownership for adaptive planning
    capabilities: [order-management]
    packs: [api, messaging, data]   # capability packs for catalog pages
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

Seven documentation profiles classify the component type. Profiles serve as **identity metadata** — they control evidence lens and writing direction but do not prescribe fixed page sets. Profile definitions are in [`internal/prompts/profiles.go`](/internal/prompts/profiles.go).

### Phase structure

Adaptive page contracts are generated by `AdaptivePageContract()` in [`internal/prompts/adaptive.go`](/internal/prompts/adaptive.go). Each contract includes:

- **Path** — Canonical Markdown file path (e.g., `components/order-service/architecture.md`)
- **Kind** — Page kind (`single`, `index`, `collection`, `shard`)
- **Required headings** — Default: `Purpose`, `Knowledge Gaps`, `Source References`; index/collection pages use `Navigation` instead of `Purpose`
- **Required diagram** — `flowchart` for most pages; empty for index/collection pages
- **Catalog contracts** — Pages under `catalogs/` require a table header: `| ID | Name | Direction | Owner | Evidence |`

### Documentation views

Pages are organized by view:

| View | Canonical location | Ownership |
|---|---|---|
| System | `quickstart.md`, `system/` | Whole-system topology, landscape, cross-component relationships |
| Domain | `domains/<domain>/` | Business capability, concepts, rules, state, interfaces, events |
| Component | `components/<component>/` | Runtime boundary, architecture, contracts, data, operations |
| Flow | `flows/<flow>.md` | Trigger, actor, steps, state changes, events, failure, compensation |
| Catalog | `catalogs/<catalog>/` | Typed lookup data with stable IDs, direction, ownership, evidence |
| Platform | `platform/` | Shared messaging, security/identity, container, deployment |
| Engineering | `engineering/` | Reusable engineering, testing, implementation standards |
| Operations | `operations/` | Runtime operation, recovery, ownership, failure handling |

Navigation is progressive: root links view indexes, view indexes link unit/collection indexes, collections link leaf pages or shards.

### Page kinds and sharding

| Kind | Behaviour |
|---|---|
| `single` | Standalone leaf page with Purpose, Knowledge Gaps, Source References |
| `index` | Navigation hub for a view or unit (no diagram contract) |
| `collection` | Catalog collection page with required table header and data rows |
| `shard` | Partitioned catalog page when rows or bytes exceed thresholds |

Sharding dimensions include: domain, subdomain, bounded-context, component, owner, repository, runtime, transport, data-store, criticality.

### Profile contracts

| Profile | Description |
|---|---|
| `application` | Deployable application (monolith, microservice, worker, gateway, frontend, CLI) |
| `modular-application` | Modular monolith with module-aware documentation |
| `reusable` | Library, SDK, framework, shared package |
| `infrastructure` | IaC, GitOps, platform, deployment |
| `configuration` | Shared configuration, policy, templates, manifests |
| `contracts` | API, event, message, data, protocol contracts |
| `generic` | Language/tech-neutral fallback |

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

## V1/V2 backward compatibility

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
| `/internal/prompts/profiles.go` | All 7 profile identity definitions |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization utilities |
