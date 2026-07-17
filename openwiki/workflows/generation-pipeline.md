---
type: Workflow
title: WikiForge Generation Pipeline
description: End-to-end documentation generation workflow including plan, generate, update, resume, validation, repair rounds, reporting, and knowledge-graph export
tags: [pipeline, generation, validation, repair, workflow]
resource: /internal/orchestrator/orchestrator.go
---

# Generation Pipeline

The generation pipeline is the core workflow of WikiForge. It starts from a validated configuration and produces component wikis, a whole-system wiki, validation reports, and a knowledge graph.

## Pipeline phases

### 1. Plan phase

`wikiforge plan` (or `orchestrator.Plan()`) prints a human-readable phase plan for all enabled components and optionally the system. It does not execute any generation.

### 2. Generate phase

`wikiforge generate` (or `orchestrator.Generate()`) runs the full pipeline:

1. **State initialization** — Load or create run state. If resuming, preserve completed phases.
2. **Repository grouping** — Group components by repository path. Same-repo components are serialized.
3. **Worker dispatch** — Bounded goroutine workers (up to `execution.parallelComponents`) process repository groups.
4. **Per-component generation** — For each component, run the profile-defined phases sequentially.
5. **Validation** — After all component phases, run complete validation.
6. **Repair rounds** — If validation fails below threshold, run targeted repair phases.
7. **System aggregation** — Generate whole-system wiki from component documentation snapshots.
8. **Report writing** — JSON report and Markdown summary under `.wikiforge/reports/<runID>/`.
9. **Graph export** — Optionally export JSONL knowledge graph.

### 3. Update phase

`wikiforge update` runs the same pipeline as generate but with these differences:

- Preserves last successful hashes for scoped no-op detection.
- If a component's source hash hasn't changed, its documentation phases are skipped.
- Starts a new run ID while retaining previous state.

### 4. Resume phase

`wikiforge resume` loads the existing state and resumes from the last incomplete phase. It is designed for recovery after Ctrl+C or transient failures.

## Component generation flow

For each component, the orchestrator runs its profile's phases in order:

```text
[Component X]
  ├─ A00  Bootstrap OpenWiki and quickstart
  ├─ A10  Overview (quickstart.md)
  ├─ A20  Architecture (architecture/overview.md)
  ├─ A30  Domain behaviour (domain/behavior.md)
  ├─ A40  Interfaces (interfaces/contracts.md)
  ├─ A50  Data and consistency (data/consistency.md)
  ├─ A60  Security and reliability (reliability/security-operations.md)
  ├─ A70  Development and change (development/change-guide.md)
  ├─ AS01 Specialized catalogs 1/6 (configuration, integrations, interfaces)
  ├─ AS02 Specialized catalogs 2/6 (messaging, processing, business)
  ├─ ...  (up to 6 batches of 4 pages each)
  ├─ AC90 Consolidate knowledge (knowledge/relationships.md)
  ├─ validate → repair-1 → validate → repair-2 → ... → accept or fail
  └─ graph export
```

### Phase execution

Each phase:

1. **Render prompt** — Combine base template, phase-specific template, profile guidance, and supplemental contracts into a complete prompt (~160 KB max).
2. **Externalize prompt** — Write prompt to a temporary `.wikiforge-prompt-*.md` file in the component working directory.
3. **Bridge invocation** — Pass a single-line JSON reference to OpenWiki via `--print`.
4. **Capture output** — Stream stdout/stderr from OpenWiki with heartbeat.
5. **Validate** — After the phase process exits, validate the generated documentation.
6. **Repair if needed** — If validation fails below threshold and repair rounds remain, run a targeted repair prompt.
7. **Cleanup** — Remove the temporary prompt file.

## Validation and repair

After all component phases complete, WikiForge runs a full validation pass. See [Testing and validation](../operations/testing-validation.md) for the complete validation rule set.

### Repair rounds

If validation produces a score below `minimumQualityScore` (default 85) or any errors, WikiForge runs targeted repair:

1. A repair prompt is generated from the validator findings.
2. OpenWiki is invoked with the repair prompt targeting only the affected files.
3. A new validation pass runs after the repair process exits.
4. Up to `execution.maxRepairRounds` (default 2) rounds are attempted.
5. The final validation result determines acceptance.

### Semantic failure detection

OpenWiki can exit with code 0 without performing the requested work (e.g., "Could you clarify?" responses). WikiForge detects these semantic failures and retries them according to the process retry policy.

## Whole-system aggregation

After all components complete, if `system.enabled` is true:

1. Component wikis are copied into `system.output/sources/components/{component-id}/`.
2. Authoritative facts from `system.factsPath` are optionally copied into `system.output/sources/facts/`.
3. A whole-system wiki is generated using the system phase set (9 core phases + up to 5 specialized catalog phases).
4. The system wiki validates, repairs, and exports a knowledge graph independently.

### System phases

| ID | Page | Description |
|---|---|---|
| W00 | (bootstrap) | Initialize system workspace |
| W05 | `quickstart.md` | System overview and reading paths |
| W10 | `system/landscape.md` | Business and technical capabilities |
| W20 | `system/component-map.md` | Component catalog with dependency relationships |
| W30 | `system/cross-component-flows.md` | Cross-component journey flows |
| W40 | `system/data-events-contracts.md` | Data ownership, events, and contract catalog |
| W45 | `system/infrastructure-deployment.md` | Infrastructure topology and deployment |
| W50 | `system/failure-security-operations.md` | Failure modes, security, observability |
| WS01–WS05 | System specialized catalogs | Dependency matrix, endpoint/event/job catalogs, etc. |
| WO60 | `system/onboarding-change-guide.md` | Reading order, change workflows, review checklist |
| WC90 | `knowledge/relationships.md` | Entity index, relationship table, traceability |

## Reporting

After generation, `report.Write()` produces:

- **`report.json`** — Full structured report with per-component validation results, system results, and failures.
- **`summary.md`** — Human-readable Markdown summary with component scores and acceptance status.
- **`latest.txt`** — Points to the most recent report directory.

Reports are stored under `.wikiforge/reports/<runID>/`.

## Knowledge graph export

[`internal/graph/graph.go`](/internal/graph/graph.go) walks documentation directories and produces two JSONL files:

- **`nodes.jsonl`** — Document nodes (from `.md` files) and concept nodes (from link targets and relationship tables).
- **`edges.jsonl`** — `LINKS_TO` edges from Markdown links and semantic relationship edges from relationship tables.

Each edge has `from`, `relationship`, `to`, `source`, and optional `evidence`, `authority`, and `confidence` fields.

## Source map

| File | Role |
|---|---|
| `/internal/orchestrator/orchestrator.go` | Core orchestration, generate/update/resume |
| `/internal/orchestrator/orchestrator_test.go` | End-to-end fake-runner tests for all profiles |
| `/internal/orchestrator/progress.go` | Progress bar display |
| `/internal/graph/graph.go` | JSONL knowledge graph export |
| `/internal/report/report.go` | Report writing (JSON + Markdown) |
| `/internal/state/store.go` | Persistent state store |
| `/internal/validation/validation.go` | Validation engine |
| `/internal/openwiki/runner.go` | Phase execution via OpenWiki |
