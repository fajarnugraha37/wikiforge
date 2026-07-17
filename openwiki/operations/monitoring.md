---
type: Operations
title: Operations and Monitoring
description: Progress display, heartbeat monitoring, state management, resume, and failure handling in WikiForge
tags: [operations, monitoring, progress, state, resume, failure]
resource: /internal/orchestrator/progress.go
---

# Operations and Monitoring

WikiForge provides operational visibility through live progress bars, heartbeat monitoring for long-running phases, persistent state for resume, and explicit failure handling.

## Progress bar system

[`internal/orchestrator/progress.go`](/internal/orchestrator/progress.go) implements a line-based progress tracker:

- **Display**: `[component] [progress bar] NN% step N/N PHASE-ID STATUS description | elapsed=MM:SS`
- **Statuses**: `RUN` (in progress), `OK` (completed), `SKIP` (no-op detected), `ERR` (failed)
- **Basis**: Percentage is calculated from completed deterministic WikiForge steps (not model-token progress inside OpenWiki).
- **Safety**: Console output is mutex-protected for concurrent component display.

## Heartbeat monitoring

The `ExecRunner` in [`internal/openwiki/runner.go`](/internal/openwiki/runner.go) monitors long-running OpenWiki child processes:

- If no stdout/stderr output is received for **15 seconds**, a heartbeat message is printed:
  `[component/PHASE-ID] still running | elapsed=MM:SS | quiet=MM:SS | timeout=1h0m0s`
- Heartbeat interval is configurable via `HeartbeatInterval` (default 15 seconds).
- All child output is prefixed with the component and phase label.

## State persistence

[`internal/state/store.go`](/internal/state/store.go) persists run state to `.wikiforge/state.json` with atomic writes (write-to-tmp, rename):

- **Run ID** — Unique per generation run.
- **Mode** — `generate` or `update`.
- **Per-component state**:
  - `gitHead` — Git HEAD commit hash.
  - `docsHash` — Hash of generated documentation.
  - `sourceHash` — Hash of source tree (used for no-op detection).
  - `status` — Completed/failed/skipped.
  - `phases` — Map of phase ID to phase status (status, attempts, startedAt, completedAt, error).
- **System state** — Same structure for whole-system phases.

## Resume workflow

`wikiforge resume`:

1. Loads state from `.wikiforge/state.json`.
2. Scans phases for incomplete or failed statuses.
3. Resumes from the first incomplete phase.
4. Completed phases are skipped.
5. Failed phases are retried up to `maxProcessRetries`.
6. After Ctrl+C, cleanup removes the active temporary prompt file.

## Failure handling

### Process retries

- **Deterministic failures** (path errors, command errors): Not retried. Same invocation cannot repair them.
- **Provider/network failures**: Retried up to `execution.maxProcessRetries` (default 2).
- **Semantic failures** (clarification responses): Retried same as provider failures.

### Component failure policy

- `continueOnComponentFailure: true` (default): Failed components are reported but generation continues.
- `continueOnComponentFailure: false`: Generation stops after the first component failure.

### Failure reporting

- Failures are recorded in `rep.Failures[componentID] = errorMessage`.
- Both JSON report (`report.json`) and Markdown summary (`summary.md`) include failure details.
- The exit code is non-zero if any failure occurred.

## No-op detection (update mode)

In `wikiforge update`:

1. The orchestrator computes a source hash from the repository's current state.
2. If the source hash matches the last successful run, all phases are skipped.
3. This avoids unnecessary model calls when source code hasn't changed.

## Logging and telemetry

- **Stdout/stderr streaming**: All OpenWiki output is streamed to the WikiForge console.
- **Telemetry**: `OPENWIKI_TELEMETRY_DISABLED=1` is set by default.
- **Provider retry**: `OPENWIKI_PROVIDER_RETRY_ATTEMPTS=3` configured by default.
- **Reports**: JSON + Markdown written to `.wikiforge/reports/<runID>/`.

## Source map

| File | Role |
|---|---|
| `/internal/orchestrator/progress.go` | Line-based progress bar |
| `/internal/state/store.go` | Persistent state with atomic writes |
| `/internal/openwiki/runner.go` | Process execution, heartbeat, output streaming |
| `/internal/orchestrator/orchestrator.go` | Failure handling, retry logic, no-op detection |
| `/internal/report/report.go` | Report generation (JSON + Markdown) |
