---
type: Quickstart
title: WikiForge Documentation
description: Entrypoint for WikiForge — a component-centric, phased, validated OpenWiki orchestration tool for generating multi-profile documentation at scale
tags: [wikiforge, documentation, orchestration, openwiki, go]
resource: /cmd/wikiforge/main.go
---

# WikiForge Documentation

WikiForge is a **cross-platform Go CLI** (v1.3.0) that orchestrates [OpenWiki](https://github.com/fajarnugraha37/OpenWiki) in controlled phases to generate validated documentation for software repositories. It supports multiple documentation profiles, monorepo scoping, cross-repository parallelism, validation and repair, whole-system aggregation, and knowledge-graph export.

**Key capabilities:**

- **Adaptive planning** — Deterministic discovery, documentation units, composable capability packs, explicit include/skip/defer decisions, and persisted plans.
- **Profile-driven compatibility generation** — 7 documentation profiles (application, modular-application, reusable, infrastructure, configuration, contracts, generic) each with phase-specific contracts, required sections, and diagram types.
- **Phased orchestration** — Each component progresses through up to 10+ phases (init → overview → architecture → domain → interfaces → data → security → development → specialized catalogs → consolidate), with each phase owning one canonical page.
- **Monorepo support** — Multiple scoped components in one repository with automatic serialization to avoid competing OpenWiki writes.
- **Validation and repair** — Validates front matter, required sections, Mermaid diagrams (including rendering), source references, and relative links. Runs targeted repair rounds using validator findings.
- **Whole-system aggregation** — Combines component wikis into a system-level wiki with cross-component documentation and relationship tables.
- **Resume capability** — Persistent state store enables checkpoint-based resume after Ctrl+C or failure.
- **Knowledge graph export** — JSONL export of document nodes and relationship edges.
- **Run reporting** — JSON reports and Markdown summaries per run.

## Quick start

### 1. Extract a release

Download from [GitHub Releases](https://github.com/fajarnugraha37/wikiforge/releases).

**Windows:**
```powershell
Expand-Archive .\wikiforge-<version>-windows-amd64.zip
cd .\wikiforge-<version>-windows-amd64
```

**Linux/macOS:**
```bash
unzip wikiforge-<version>-linux-amd64.zip
cd linux-amd64
```

### 2. Generate a configuration

```bash
./wikiforge init
```

Edit `wikiforge.yaml`, add enabled components with correct types. A documentation profile is selected automatically from the type.

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

### 5. Discover and explain the adaptive plan

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
./wikiforge generate --component shared-runtime --skip-system
```

### 7. Incremental update

```bash
./wikiforge update
```

### 8. Validate existing documentation

```bash
./wikiforge validate --component order-service
./wikiforge validate --system
```

### 9. Export knowledge graph

```bash
./wikiforge graph --system
```

## Documentation sections

| Section | Description |
|---|---|
| [Architecture overview](architecture/overview.md) | High-level architecture, component model, profiles, state store, paths |
| [Configuration model](architecture/config-model.md) | Config v3, components, documentation units, views, packs, evidence boundaries |
| [Adaptive planning](architecture/adaptive-planning.md) | Deterministic discovery, composable packs, plan decisions, artifacts, checkpoint invariants |
| [Generation pipeline](workflows/generation-pipeline.md) | End-to-end generate/update/resume workflow, validation, repair, reports, graph |
| [Prompt system](workflows/prompt-system.md) | Prompt assets, phase contracts, specialized catalogs, system phases |
| [OpenWiki bridge](integrations/openwiki-bridge.md) | Child-process execution, prompt bridge protocol, cross-platform path safety |
| [CI/CD and releasing](integrations/ci-cd.md) | CI workflow, release process, Docker build |
| [Testing and validation](operations/testing-validation.md) | Test strategy, validation rules, Mermaid rendering, finding codes |
| [Operations and monitoring](operations/monitoring.md) | Progress display, heartbeat, state persistence, resume, failure handling |

## Concepts at a glance

- **Component** — An independently documented repository scope (may be a whole repo or a monorepo subdirectory).
- **Documentation unit** — A domain, bounded context, module, flow, platform area, or catalog documented independently from deployment boundaries.
- **Capability pack** — A composable concern selected by profile, explicit configuration, or source evidence.
- **Discovery manifest** — Deterministic component-scope evidence and inferred-unit artifact.
- **Documentation plan** — Explicit future page paths and include/skip/defer decisions.
- **Profile** — A backward-compatible documentation contract with required phases, pages, sections, and diagram types. Seven profiles exist: `application`, `modular-application`, `reusable`, `infrastructure`, `configuration`, `contracts`, `generic`.
- **Phase** — A single OpenWiki invocation that owns one canonical Markdown page with required sections and diagram.
- **Canonical pages** — Profile-specific Markdown files with enforced sections, front matter, and diagram contracts.
- **Specialized catalogs** — Additional optional pages with exact table-header contracts (e.g., endpoint catalogs, job catalogs, dependency matrices).
- **Whole-system wiki** — Aggregated documentation combining component wikis with cross-component relationship pages.
- **Knowledge graph** — JSONL export of document-to-document links and relationship tables.
- **Prompt bridge** — WikiForge stores large phase prompts in temporary files and passes a single-line JSON reference to OpenWiki, avoiding command-line length limits.
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
| `init` | Generate a default `wikiforge.yaml` configuration |
| `doctor` | Validate prerequisites and component scopes |
| `profiles` | List supported types, profiles, capability packs, and views |
| `config migrate` | Normalize a v1/v2/v3 config into portable v3 JSON |
| `discover` | Produce deterministic discovery manifests and component plans |
| `plan` | Preview adaptive pages and, with `--explain`, all planning decisions |
| `generate` | Generate all component wikis and the whole-system wiki |
| `update` | Incremental update of existing documentation |
| `resume` | Resume a cancelled or failed generation |
| `validate` | Validate component or system documentation |
| `graph` | Export knowledge graph from documentation |
| `version` | Print version number |

## Backlog

Phase 1 intentionally retains the fixed profile renderer. Hierarchical materialization, catalog sharding, semantic evidence indexing, and impact-based updates are deferred to the subsequent implementation phases and are not represented as completed capabilities here.
