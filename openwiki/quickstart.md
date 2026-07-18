---
type: Quickstart
title: WikiForge Documentation
description: Entrypoint for WikiForge — a component-centric, phased, validated OpenWiki orchestration tool for generating multi-profile documentation at scale
tags: [wikiforge, documentation, orchestration, openwiki, go]
resource: /cmd/wikiforge/main.go
---

# WikiForge Documentation

WikiForge is a **cross-platform Go CLI** that orchestrates [OpenWiki](https://github.com/fajarnugraha37/OpenWiki) to generate adaptive, evidence-backed documentation for software repositories. It uses a configurable version 3 schema, supports monorepo scoping, cross-repository parallelism, evidence-driven validation and repair, whole-system aggregation, and knowledge-graph export.

**Key capabilities:**

- **Adaptive hierarchical documentation** — Pages are planned from component type, capability packs, discovered source roots, explicit documentation units, and configured views (system, domain, component, flow, catalog, platform, engineering, operations).
- **Documentation units** — Separate business/domain/flow ownership from deployable repository boundaries.
- **Evidence index and surgical updates** — Git-aware evidence index tracks file identity, changed paths, and documentation dependencies. Update mode only regenerates affected pages.
- **Validation and repair** — Semantic validation for evidence resolution, stable catalog IDs, relationship vocabulary, navigation hierarchy, Mermaid diagrams, and security findings. Runs targeted repair rounds.
- **Whole-system aggregation** — Combines component wikis into a system-level wiki with content-addressed snapshots that reuse unchanged component documentation.
- **Resume capability** — Persistent state store enables checkpoint-based resume after Ctrl+C or failure.
- **Knowledge graph export** — Canonical JSONL export with typed relationships and evidence edges.
- **Run reporting** — JSON reports and Markdown summaries per run, plus runtime/token metrics when reported by the provider.

## Quick start

### 1. Extract a release

Download from [GitHub Releases](https://github.com/fajarnugraha37/wikiforge/releases).

**Windows:**
```powershell
Expand-Archive .\wikiforge-*-windows-amd64.zip
cd .\wikiforge-*-windows-amd64
```

**Linux/macOS:**
```bash
unzip wikiforge-*-linux-amd64.zip
cd linux-amd64
```

### 2. Generate a configuration

```bash
./wikiforge init
```

Edit `wikiforge.yaml`, add enabled components with correct types. A documentation profile is selected automatically from the type. Optionally add explicit documentation units and enable evidence caching.

### 3. Configure provider credentials

```bash
export OPENWIKI_PROVIDER=openai-compatible
export OPENAI_COMPATIBLE_API_KEY=replace-me
export OPENAI_COMPATIBLE_BASE_URL=https://gateway.example.com/v1
export OPENWIKI_MODEL_ID=cheap-code-model
```

### 4. Validate prerequisites

```bash
./wikiforge doctor
```

### 5. Preview the adaptive plan

```bash
./wikiforge discover
./wikiforge plan --explain
```

### 6. Generate all wikis

```bash
./wikiforge generate
```

Generate a single component:

```bash
./wikiforge generate --component order-service --skip-system
```

### 7. Incremental update

```bash
./wikiforge update
```

### 8. Validate and inspect artifacts

```bash
./wikiforge validate --strict
./wikiforge coverage
./wikiforge impact
```

### 9. Export knowledge graph

```bash
./wikiforge graph --system
```

## Documentation sections

| Section | Description |
|---|---|
| [Architecture overview](architecture/overview.md) | High-level architecture, component model, profiles, state store, paths |
| [Configuration model](architecture/config-model.md) | Config schema, component types, profile selection, normalization |
| [Generation pipeline](workflows/generation-pipeline.md) | End-to-end generate/update/resume workflow, validation, repair, reports, graph |
| [Prompt system](workflows/prompt-system.md) | Prompt assets, phase contracts, specialized catalogs, system phases |
| [OpenWiki bridge](integrations/openwiki-bridge.md) | Child-process execution, prompt bridge protocol, cross-platform path safety |
| [CI/CD and releasing](integrations/ci-cd.md) | CI workflow, release process, Docker build |
| [Testing and validation](operations/testing-validation.md) | Test strategy, validation rules, Mermaid rendering, finding codes |
| [Operations and monitoring](operations/monitoring.md) | Progress display, heartbeat, state persistence, resume, failure handling |

## Concepts at a glance

- **Component** — An independently documented repository scope (may be a whole repo or a monorepo subdirectory).
- **Profile** — Identity metadata classifying the component type (application, modular-application, reusable, infrastructure, configuration, contracts, generic). Profiles guide evidence selection and writing tone but do not prescribe fixed page sets.
- **Documentation view** — One of: system, domain, component, flow, catalog, platform, engineering, operations. Views organize pages by perspective.
- **Documentation unit** — A separately planned documentation scope owned by a domain, flow, capability, or team. May cross repository boundaries.
- **Documentation pack** — A capability pack (e.g., `api`, `messaging`, `cache`, `jobs`) that enables conditional catalog collections and concern pages.
- **Page kind** — `single` (leaf page), `index` (navigation hub), `collection` (catalog collection with table contracts), or `shard` (partitioned catalog page).
- **Adaptive planner** — Discovers candidate documentation units from the repository, selects views and packs based on component configuration, and produces a page plan executed by the orchestrator.
- **Evidence index** — Git-aware file index with tracked identity, content hashes, and change detection. Enables surgical update mode (only regenerates affected pages).
- **Whole-system wiki** — Aggregated documentation combining component wikis with cross-component relationship pages using content-addressed snapshots.
- **Knowledge graph** — Canonical JSONL export of document nodes and relationship edges with typed relationships and evidence sources.
- **Prompt bridge** — WikiForge stores large phase prompts in temporary files and passes a single-line JSON virtual path reference (`/openwiki/.wikiforge-prompt-*.md`) to OpenWiki, avoiding command-line length limits.
- **State store** — Persistent JSON checkpoint at `.wikiforge/state.json` enabling resume after cancellation.

## Runtime requirements

- **Go binary** — Native Go, cross-compiled for Windows, Linux, and macOS (amd64 + arm64).
- **Git** — Required for repository inspection.
- **Node.js 22+** — Required for OpenWiki and Mermaid CLI through `npx`.
- **OpenWiki 0.2.0** — The default pinned version executed by WikiForge.
- **Mermaid CLI 11.12.0** — Optional but default for Mermaid rendering validation.
- **Provider credentials** — Set via environment variables (`OPENWIKI_PROVIDER`, `OPENAI_COMPATIBLE_API_KEY`, etc.).

## CLI commands

| Command | Description |
|---|---|
| `init` | Generate a default `wikiforge.yaml` configuration (v3) |
| `doctor` | Validate prerequisites and component scopes |
| `discover` | Run repository discovery and list candidate documentation units |
| `plan` | Preview the adaptive plan (`--explain` for rationale) |
| `generate` | Generate all component wikis and the whole-system wiki |
| `update` | Incremental update using evidence index (surgical no-op detection) |
| `resume` | Resume a cancelled or failed generation |
| `validate` | Validate component or system documentation (`--strict` for higher threshold) |
| `coverage` | Show evidence coverage for component or system documentation |
| `impact` | Show change impact analysis from the evidence index |
| `graph` | Export knowledge graph from documentation |
| `version` | Print version number |

## Backlog

No areas deferred. The updated documentation set covers the adaptive v3 architecture of WikiForge.
