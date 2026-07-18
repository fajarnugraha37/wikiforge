# WikiForge Configuration

WikiForge reads one version `3` YAML configuration file. The file describes repositories, documentation units, evidence boundaries, OpenWiki execution, validation, and optional whole-system aggregation. The adaptive documentation planner is always used; there is no fixed-layout mode.

## Minimal configuration

```yaml
version: 3
workspace: .

openwiki:
  command: npx
  args: [--yes, openwiki@0.2.0, code]
  timeoutMinutes: 60
  modelId: ""

components:
  - id: order-service
    type: microservice
    repository: ./repositories/order-service
    enabled: true

system:
  enabled: true
  id: enterprise-system
  title: Enterprise System
  output: ./enterprise-wiki
```

Provider credentials are never stored in this file. Configure the environment expected by the selected OpenWiki provider before running `doctor` or `generate`.

## Top-level fields

| Field | Required | Description |
|---|---:|---|
| `version` | yes | Must be `3`. |
| `workspace` | no | Base directory for state, reports, indexes, graphs, and caches. Defaults to the config directory. |
| `openwiki` | yes | OpenWiki executable, arguments, model, timeout, diagnostics, and environment. |
| `execution` | no | Parallelism, retries, repair rounds, failure policy, and isolated monorepo staging. |
| `documentation` | no | Language, views, evidence policy, catalog limits, and validation policy. |
| `mermaid` | no | Mermaid validation mode, renderer, timeout, cache, and worker count. |
| `components` | yes | Enabled repository documentation targets. |
| `documentationUnits` | no | Domain, flow, catalog, platform, engineering, or operations boundaries inside a component. |
| `system` | no | Whole-system aggregation target. |

## OpenWiki

`openwiki.command` and `openwiki.args` identify the local OpenWiki command. The default is `npx --yes openwiki@0.2.0 code`. `modelId` is passed as `--modelId` when non-empty. `timeoutMinutes` bounds each process. `maxCaptureBytes` bounds retained diagnostics; `logDirectory` optionally stores complete stdout/stderr logs. `environment` adds process environment variables without embedding secrets in generated configuration.

## Execution

```yaml
execution:
  parallelComponents: 2
  maxProcessRetries: 2
  maxRepairRounds: 2
  continueOnComponentFailure: true
  isolateSameRepository: true
```

Scoped components in one repository use isolated staging directories when `isolateSameRepository` is enabled. Their generated `openwiki` directories are synchronized back to their own scopes after successful execution. Set `continueOnComponentFailure: false` to stop scheduling after the first component failure.

## Components

Every enabled component requires a unique portable `id`, a `type`, a `repository`, and `enabled: true`. `scope` selects a relative directory inside a monorepo. `profile` is optional and is derived from `type`; use it only when a custom type needs a different adaptive evidence lens.

```yaml
components:
  - id: commerce-core
    type: modular-monolith
    repository: ./repositories/commerce-core
    enabled: true
    owners: [commerce-team]
    capabilities: [order-management, pricing]
    packs: [workflow, telemetry]

  - id: submit-order-worker
    type: worker
    repository: ./repositories/platform-monorepo
    scope: apps/submit-order-worker
    enabled: true
    includeInSystem: true
    dependsOn: [commerce-core]
```

Supported types map to `application`, `modular-application`, `reusable`, `infrastructure`, `configuration`, `contracts`, or `generic` profiles. `owners`, `capabilities`, `packs`, `tags`, `dependsOn`, `group`, and `includeInSystem` enrich planning and aggregation without changing the runtime boundary.

## Documentation units

Documentation units separate documentation ownership from deployable component boundaries.

```yaml
documentationUnits:
  - id: order-management
    component: commerce-core
    kind: domain
    domain: Order Management
    boundedContext: Ordering
    sourceRoots: [modules/order]
    evidenceRoots: [modules/order, workflows/order]
    output: domains/order-management
    owners: [commerce-team]
    shardBy: [domain]
    maximumRows: 150

  - id: submit-order
    component: commerce-core
    kind: flow
    sourceRoots: [workflows/order]
    relatedUnits: [order-management]
    output: flows/submit-order.md
```

`kind` may be `component`, `domain`, `flow`, `catalog`, `platform`, `engineering`, or `operations`. Repository directories named `modules`, `domains`, `bounded-contexts`, `contexts`, `workflows`, `bpmn`, and `processes` are also discovered when explicit units are absent.

## Documentation and evidence

```yaml
documentation:
  language: English
  views: [system, domain, component, flow, catalog, platform, engineering, operations]
  minimumQualityScore: 85
  minimumPages: 0 # use adaptive planner defaults
  requireFrontMatter: true
  requireSourceReferences: true
  validateSourcePaths: true
  requireMermaid: true
  minimumMermaidBlocks: 0
  allowedDiagramTypes: [flowchart, sequenceDiagram, stateDiagram-v2, erDiagram]
  catalogs:
    shardBy: [domain, owner]
    maximumRowsPerPage: 150
    maximumBytesPerPage: 524288
  evidence:
    include: [src/**, config/**, deploy/**]
    exclude: [vendor/**, generated/**, "**/*.bin"]
    cacheDirectory: ./.wikiforge/cache/evidence
    maxFileBytes: 4194304
  frontMatterPolicy: namespaced
  requireVerifiedEvidence: true
  requireCatalogIdentity: true
  requireRelationshipEvidence: true
```

If `include` is empty, tracked and standard untracked repository files are considered, subject to built-in exclusions. Unit `evidenceRoots` are added to the evidence scope. Binary files, generated/vendor directories, oversized files, and symlinks escaping the repository are excluded deterministically.

Adaptive output uses progressive navigation: root quickstart links view indexes, indexes link units or collections, and collections link catalog shards. Catalog rows require stable IDs and evidence. The validator rejects rows beyond configured row/byte limits.

## Mermaid

Use `mode: basic` for offline structural checks, `mode: render` for renderer-backed SVG validation, or `mode: off` when diagrams are not part of a local validation run. Rendering uses a bounded worker pool and content-hash cache.

## System aggregation

```yaml
system:
  enabled: true
  id: enterprise-system
  title: Enterprise System
  output: ./enterprise-wiki
  factsPath: ./facts
  tags: [enterprise]
```

Component documentation is copied into content-addressed snapshots under `sources/components/<component>/<docs-hash>/`. The system manifest points to the active snapshot and does not expose raw component repositories to system-level OpenWiki prompts.

## Commands

```text
wikiforge init [--config wikiforge.yaml] [--force]
wikiforge doctor [--config wikiforge.yaml]
wikiforge discover [--config wikiforge.yaml] [--component ID]
wikiforge plan [--config wikiforge.yaml] [--component ID] [--skip-system] [--explain]
wikiforge generate [--config wikiforge.yaml] [--component ID] [--skip-system] [--resume]
wikiforge update [--config wikiforge.yaml] [--component ID] [--skip-system]
wikiforge resume [--config wikiforge.yaml]
wikiforge validate [--config wikiforge.yaml] [--component ID] [--system] [--strict]
wikiforge coverage [--config wikiforge.yaml] [--component ID] [--system]
wikiforge impact [--config wikiforge.yaml] [--component ID] [--system]
wikiforge graph [--config wikiforge.yaml] [--component ID] [--system]
wikiforge version
```

Use `plan --explain` before generation to inspect discovered units, pages, packs, shard decisions, and skip reasons. Use `coverage` and `impact` after generation to inspect persisted evidence-driven artifacts.
