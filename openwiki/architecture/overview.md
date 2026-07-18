---
type: Architecture
title: WikiForge Architecture Overview
description: High-level architecture, component model, phase lifecycle, state management, and cross-platform path handling in WikiForge
tags: [architecture, go, cli, orchestration]
resource: /internal/orchestrator/orchestrator.go
---

# Architecture Overview

WikiForge is a **phased OpenWiki orchestrator** written in Go (1.23). It does not parse programming languages, frameworks, or IaC dialects — that responsibility stays with OpenWiki. WikiForge provides repeatable orchestration, validation, repair, reporting, and aggregation.

## Entry point and signal handling

The application starts in [`cmd/wikiforge/main.go`](/cmd/wikiforge/main.go) which delegates to [`internal/app/app.go`](/internal/app/app.go). The `app.Run` function sets up a context that responds to `SIGINT` and `SIGTERM`, enabling clean cancellation of in-flight OpenWiki child processes.

```
cmd/wikiforge/main.go
  └─ internal/app/app.go (signal context, CLI dispatch)
       └─ internal/cli/cli.go (command routing, flag parsing, version)
```

## CLI routing

[`internal/cli/cli.go`](/internal/cli/cli.go) routes all commands from the `CLI.Run` method. Each command (init, doctor, profiles, plan, generate, update, resume, validate, graph) has its own handler. The `--component` flag (backward-compatible `--service` alias) selects individual components.

Key default values are defined here:
- **Version**: `1.2.3` (constant)
- **Config file**: `wikiforge.yaml` (overridable with `--config`)

## Configuration subsystem

[`internal/config/config.go`](/internal/config/config.go) provides the full configuration model:

- **Version 2** config with backward-compatible v1 `services` → `components` migration.
- Custom YAML subset parser at [`internal/config/yaml.go`](/internal/config/yaml.go) — minimal indentation-based parser that avoids full YAML library dependencies for generated configs.
- JSON Schema at [`/schema/wikiforge-config.schema.json`](/schema/wikiforge-config.schema.json) for editor validation.
- Defaults applied automatically (parallelism, timeouts, Mermaid mode, etc.).

For details, see the [configuration model](config-model.md).

## Component model

Every enabled `ComponentConfig` has:

| Field | Description |
|---|---|
| `id` | Unique identifier (restricted to portable path segments) |
| `type` | Repository class (microservice, monolith, framework, iac, etc.) |
| `profile` | Documentation profile selected from type or explicitly overridden |
| `repository` | Absolute or relative path to the Git repository |
| `scope` | Relative subdirectory within the repository (monorepo support) |
| `group` | Optional logical grouping |
| `tags` | Classification tags |
| `dependsOn` | Declared component dependencies |
| `includeInSystem` | Whether to include in whole-system aggregation |

Multiple components may share one `repository` with different `scope` values. They are automatically serialized during generation to avoid competing OpenWiki writes.

## Profile and adaptive planning system

Seven [documentation profiles](config-model.md#profiles) classify the component type:

- **application** — Deployable applications (monoliths, microservices, gateways, frontends, CLIs)
- **modular-application** — Modular monoliths with module-aware documentation
- **reusable** — Libraries, SDKs, frameworks
- **infrastructure** — IaC, GitOps, platform repos
- **configuration** — Shared config and policy repos
- **contracts** — API, event, and schema contract repos
- **generic** — Language/tech-neutral fallback

Profiles are now **identity metadata** rather than phase contracts. They control evidence-lens and writing direction, but page selection is determined by the [adaptive planner](../workflows/generation-pipeline.md#1-discovery-and-planning). Profile definitions are in [`internal/prompts/profiles.go`](/internal/prompts/profiles.go).

## Documentation views

The adaptive system organizes documentation across eight views:

- **System** — Whole-system topology, landscape, and cross-component relationships
- **Domain** — Business capability, concepts, rules, state, interfaces, events
- **Component** — Runtime boundary, architecture, contracts, data, configuration, operations
- **Flow** — Trigger, actor, steps, state changes, transactions, events, failure, compensation
- **Catalog** — Typed lookup data with stable IDs and evidence
- **Platform** — Shared messaging, security/identity, container, deployment mechanisms
- **Engineering** — Reusable engineering, testing, implementation standards
- **Operations** — Runtime operation, recovery, ownership, failure handling

## Orchestration engine

[`internal/orchestrator/orchestrator.go`](/internal/orchestrator/orchestrator.go) is the core:

1. **Adaptive planning** — Run discovery and planning for each component to determine page set (see [Planner](#adaptive-planner)).
2. **Evidence preparation** — Build the [evidence index](config-model.md#evidence-config) with file identity, content hashes, and change detection.
3. **Repository grouper** — Groups components by repository path. Same-repo components are serialized (or isolated with `execution.isolateSameRepository`); different repos run in parallel (up to `execution.parallelComponents`).
4. **Adaptive page executor** — Per component: iterate planned pages, call OpenWiki via the runner, validate results, repair if needed.
5. **System aggregator** — After all components complete, generates a whole-system wiki with content-addressed snapshots.
6. **Report writer** — Writes JSON and Markdown reports under `.wikiforge/reports/<runID>/`.

### Adaptive component execution

[`internal/orchestrator/adaptive.go`](/internal/orchestrator/adaptive.go) implements `runAdaptiveComponent`:

1. Runs repository discovery via `planner.Discover()`.
2. Plans adaptive pages via `planner.Plan()`.
3. Saves plan artifacts (discovery, plan, evidence-index, impact-index, coverage).
4. Writes `openwiki/INSTRUCTIONS.md` with adaptive context.
5. Removes unplanned documentation files.
6. Prepares evidence index from repository (cached file scanning).
7. Generates pages: each planned page gets an adaptive prompt rendered from `prompts/component/phase.md`.
8. Updates use the evidence index's change impact to only regenerate affected pages.
9. Finalizes with adaptive validation + evidence-backed checks + export.

### Parallelism and serialization

```go
// From orchestrator.go — repository groups are dispatched to bounded worker goroutines
groups := repositoryGroups(components)
workers := o.Config.Execution.ParallelComponents
```

Components sharing a Git repository are deliberately placed in the same group and serialized.

## State store and resume

[`internal/state/store.go`](/internal/state/store.go) persists run state to `.wikiforge/state.json` in JSON format with atomic file writes (write-to-tmp, rename). The state includes:

- **Run ID**, mode (generate/update), start time
- **Per-component state** — Git HEAD, documentation hash, source hash, phase statuses
- **System state** — Phase statuses for whole-system phases

The state enables:
- Resume after Ctrl+C: `wikiforge resume` skips completed phases.
- Scoped update no-op detection: if a component's source hash hasn't changed, its phases are skipped.

## Cross-platform path handling

[`internal/pathutil/pathutil.go`](/internal/pathutil/pathutil.go) provides:

- `IsAbsoluteAny` — Detects POSIX, Windows drive, UNC, and extended-length paths.
- `NormalizeRelative` — Rejects absolute paths, parent escapes, control characters; normalizes mixed slashes.
- `Resolve` — Expands home directories, normalizes separators, returns cleaned absolute path.
- `ExternalToolPath` — Converts native paths to quote-free forward-slash paths for Node tools on Windows.

### Prompt virtual paths

The prompt bridge uses **absolute virtual paths** rooted at `/openwiki/` in the OpenWiki repository filesystem, not host filesystem paths. Temporary prompt files are written to `{workdir}/openwiki/.wikiforge-prompt-<hash>.md` and referenced as `/openwiki/.wikiforge-prompt-<hash>.md`. This avoids Windows path-length limits, UNC prefix issues, and shell quoting problems.

For details, see the [OpenWiki bridge](../integrations/openwiki-bridge.md).

## Source map

| File | Role |
|---|---|
| `/cmd/wikiforge/main.go` | Binary entry point |
| `/internal/app/app.go` | Signal handling, CLI dispatch |
| `/internal/cli/cli.go` | Command routing, flag parsing, version |
| `/internal/config/config.go` | Config struct, defaults, loading, validation |
| `/internal/config/yaml.go` | Custom YAML subset parser |
| `/internal/orchestrator/orchestrator.go` | Core orchestration, generate/update/resume, system aggregation |
| `/internal/orchestrator/adaptive.go` | Adaptive component and system execution |
| `/internal/orchestrator/evidence_artifacts.go` | Evidence prep, impact, coverage |
| `/internal/orchestrator/progress.go` | Line-based progress bar display |
| `/internal/planner/planner.go` | Repository discovery, adaptive page planning |
| `/internal/evidence/evidence.go` | Evidence index, change impact, coverage |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization |
| `/internal/state/store.go` | Persistent run-state store |
| `/internal/model/model.go` | Core data types (PageContract, PageKind, ValidationResult, etc.) |
| `/internal/prompts/profiles.go` | Profile identity definitions |
| `/internal/prompts/adaptive.go` | Adaptive page contracts and rendering |
| `/schema/wikiforge-config.schema.json` | JSON Schema for configuration |
