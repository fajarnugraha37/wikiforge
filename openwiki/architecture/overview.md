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

[`internal/cli/cli.go`](/internal/cli/cli.go) routes all commands from the `CLI.Run` method. Each command (init, doctor, profiles, config migrate, discover, plan, generate, update, resume, validate, graph) has its own handler. The `--component` flag (backward-compatible `--service` alias) selects individual components.

Key default values are defined here:
- **Version**: `1.3.0` (constant)
- **Config file**: `wikiforge.yaml` (overridable with `--config`)

## Configuration subsystem

[`internal/config/config.go`](/internal/config/config.go) provides the full configuration model:

- **Version 3** config with backward-compatible v1 `services` and v2 component normalization, explicit documentation units, composable capability packs, views, evidence boundaries, and shard policy.
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
| `owners` | Ownership hints |
| `capabilities` | Business capabilities converted to configured domain units |
| `packs` | Explicit capability packs composed with profile defaults and discovery |
| `includeInSystem` | Whether to include in whole-system aggregation |

Multiple components may share one `repository` with different `scope` values. They are automatically serialized during generation to avoid competing OpenWiki writes.

## Discovery and adaptive planning

Before generation, WikiForge scans the normalized component scope using configured evidence include/exclude rules. It writes deterministic `discovery.json` and `plan.json` artifacts under `.wikiforge/components/<id>/`. The planner separates component boundaries from documentation units, combines profile, explicit, and discovered capability packs, and records include/skip/defer decisions. See [Adaptive planning](adaptive-planning.md).

## Profile and phase system

Seven [documentation profiles](config-model.md#profiles) define phase contracts:

- **application** (A00–A90) — Deployable applications
- **modular-application** (M00–M90) — Modular monoliths with module-aware phases
- **reusable** (R00–R90) — Libraries, SDKs, frameworks
- **infrastructure** (I00–I90) — IaC, GitOps, platform repos
- **configuration** (C00–C90) — Shared config and policy repos
- **contracts** (K00–K90) — API, event, and schema contract repos
- **generic** (G00–G90) — Language/tech-neutral fallback

Each profile has 8 core phases plus specialized catalog batches and a consolidate phase. Phases are defined in [`internal/prompts/profiles.go`](/internal/prompts/profiles.go).

## Orchestration engine

[`internal/orchestrator/orchestrator.go`](/internal/orchestrator/orchestrator.go) is the core:

1. **Repository grouper** — Groups components by repository path. Same-repo components are serialized; different repos run in parallel (up to `execution.parallelComponents`).
2. **Phase executor** — Per component: iterate phases, call OpenWiki via the runner, validate results, repair if needed.
3. **System aggregator** — After all components complete, generates a whole-system wiki in a separate output directory from component snapshots.
4. **Report writer** — Writes JSON and Markdown reports under `.wikiforge/reports/<runID>/`.

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
- **Per-component state** — Git HEAD, last-successful documentation/source/discovery/plan hashes, status, and phase statuses
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

These functions are critical for the prompt bridge (see [OpenWiki bridge](../integrations/openwiki-bridge.md)).

## Source map

| File | Role |
|---|---|
| `/cmd/wikiforge/main.go` | Binary entry point |
| `/internal/app/app.go` | Signal handling, CLI dispatch |
| `/internal/cli/cli.go` | Command routing, flag parsing, version |
| `/internal/config/config.go` | Config struct, defaults, loading, validation |
| `/internal/config/yaml.go` | Custom YAML subset parser |
| `/internal/orchestrator/orchestrator.go` | Core orchestration, phase execution, system aggregation |
| `/internal/orchestrator/progress.go` | Line-based progress bar display |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization |
| `/internal/state/store.go` | Persistent run-state store |
| `/internal/model/model.go` | Core data types (Phase, Component, RunState, etc.) |
| `/schema/wikiforge-config.schema.json` | JSON Schema for configuration |
