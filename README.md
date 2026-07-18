# WikiForge

WikiForge is a cross-platform Go CLI that orchestrates OpenWiki to generate adaptive, evidence-backed documentation for repositories and whole systems.

The current implementation has one documentation pipeline:

```text
configuration
  -> discovery
  -> documentation-unit planning
  -> bounded OpenWiki page runs
  -> evidence indexing
  -> semantic validation and repair
  -> coverage, impact, report, and graph artifacts
  -> optional whole-system federation
```

There is no fixed-layout executor or compatibility command surface. Configuration files must use version `3`.

## Capabilities

- Adaptive hierarchical documentation with domain, component, flow, catalog, platform, engineering, operations, and system views.
- Documentation units that separate business/domain/flow ownership from deployable repository boundaries.
- Conditional concern packs and typed catalog collections with domain/owner sharding.
- Git-aware evidence index, cached tracked-file identity, configurable include/exclude roots, size limits, and symlink escape protection.
- Surgical update impact using changed paths, evidence dependencies, documentation units, pages, and shards.
- Semantic validation for evidence resolution, stable catalog IDs, relationship vocabulary, duplicate concepts, absolute links, navigation, security findings, and catalog bounds.
- Canonical JSONL knowledge graph with typed relationships and evidence edges.
- Content-addressed whole-system snapshots that reuse unchanged component documentation.
- Bounded OpenWiki diagnostics, optional complete process logs, bounded Mermaid rendering, and runtime/token metrics when reported by the provider.
- Isolated staging for scoped components that share a repository.
- Cross-platform CI for Linux, macOS, and Windows.

## Requirements

- Go 1.23 or newer.
- Git.
- Node.js 22 or newer for the default OpenWiki and Mermaid commands.
- OpenWiki 0.2.0.
- Mermaid CLI 11.12.0 when `mermaid.mode: render`.
- Provider credentials configured outside the repository according to the selected OpenWiki provider.

WikiForge never stores provider secrets in generated configuration, documentation, reports, or source control.

## Quick Start

Create a v3 configuration:

```bash
wikiforge init
```

Edit `wikiforge.yaml`, then inspect prerequisites and the adaptive plan:

```bash
wikiforge doctor
wikiforge discover
wikiforge plan --explain
```

Generate documentation:

```bash
wikiforge generate
```

Generate or update one component without system aggregation:

```bash
wikiforge generate --component order-service --skip-system
wikiforge update --component order-service --skip-system
```

Validate and inspect artifacts:

```bash
wikiforge validate --strict
wikiforge coverage
wikiforge impact
wikiforge graph --system
```

Resume a run after interruption:

```bash
wikiforge resume
```

See [DOCUMENTATION-CONFIG.md](DOCUMENTATION-CONFIG.md) for the complete configuration reference. The canonical examples are [wikiforge.example.yaml](wikiforge.example.yaml) and [examples/wikiforge.yaml](examples/wikiforge.yaml).

## Configuration model

A component describes a repository or relative monorepo scope:

```yaml
version: 3

components:
  - id: commerce-core
    type: modular-monolith
    repository: ./repositories/commerce-core
    enabled: true
    owners: [commerce-team]
    capabilities: [order-management, pricing]
    packs: [workflow, telemetry]
```

Use documentation units when one runtime component contains multiple domains, bounded contexts, workflows, catalogs, or operational views:

```yaml
documentationUnits:
  - id: order-management
    component: commerce-core
    kind: domain
    sourceRoots: [modules/order]
    evidenceRoots: [modules/order, workflows/order]
    output: domains/order-management
    shardBy: [domain]
    maximumRows: 150
```

Components are typed as applications, modular applications, reusable libraries/frameworks, infrastructure, configuration, contracts, or generic repositories. The profile and capability packs are selected from these declarations and repository discovery.

## Adaptive output

The planner creates only applicable pages and records every decision:

```text
quickstart.md
  -> view index
    -> unit or catalog collection
      -> leaf page or catalog shard
```

Small repositories do not receive empty concern pages. Large catalogs receive typed collection pages and shards when configured dimensions apply. Workflow and BPMN sources are discovered from `workflows/`, `bpmn/`, and `processes/`.

## Evidence and validation

Every adaptive run persists artifacts below `.wikiforge/`:

```text
components/<component-id>/
  discovery.json
  plan.json
  evidence-index.json
  impact-index.json
  coverage.json
system/<system-id>/
  discovery.json
  plan.json
  evidence-index.json
  impact-index.json
  coverage.json
validation/
graph/
state.json
```

Validation dimensions include structure, navigation, evidence, coverage, semantic consistency, catalog integrity, graph integrity, security, freshness, and diagrams. A `Verified` claim must resolve to evidence. Unknown or conflicting facts remain visible as knowledge gaps.

## CI

The normal CI workflow runs only dependency-free repository checks:

- `go test ./...` on Linux, macOS, and Windows.
- `go test -race ./...` on Linux.
- `go vet ./...`.
- `go build ./cmd/wikiforge`.
- Go build verification.

No CI job invokes an LLM or requires provider credentials. Real OpenWiki execution is intentionally excluded from CI because the pipeline has no model credentials; run it manually in an environment with approved credentials when needed.

## Repository documentation

- [DOCUMENTATION-CONFIG.md](DOCUMENTATION-CONFIG.md): configuration reference and operational examples.
- [DOCUMENTATION-CATALOG.md](DOCUMENTATION-CATALOG.md): adaptive views, packs, and catalog locations.
- [BUILD-VERIFICATION.md](BUILD-VERIFICATION.md): local build verification record.
- [RELEASING.md](RELEASING.md): release workflow.
- [schema/wikiforge-config.schema.json](schema/wikiforge-config.schema.json): machine-readable v3 schema.
