---
type: Workflow
title: WikiForge Adaptive Generation Pipeline
description: End-to-end adaptive documentation workflow including discovery, planning, evidence indexing, page generation, validation, repair rounds, reporting, impact analysis, and knowledge-graph export
tags: [pipeline, generation, adaptive, evidence, validation, repair, workflow]
resource: /internal/orchestrator/orchestrator.go
---

# Adaptive Generation Pipeline

The generation pipeline is the core workflow of WikiForge. It starts from a validated configuration and produces component wikis, a whole-system wiki, evidence indexes, validation reports, and a knowledge graph.

## Pipeline phases

### 1. Discovery and planning

`wikiforge discover` (or `orchestrator.Discover()`) runs repository discovery to find candidate documentation units — source directories, owners, capabilities, and packs. Results are saved to `.wikiforge/components/<id>/discovery.json`.

`wikiforge plan --explain` (or `orchestrator.Plan()`) produces the adaptive page plan: which pages will be generated for each component, their kinds (single, index, collection, shard), views, ownership, and rationale. Results are saved to `.wikiforge/components/<id>/plan.json`.

### 2. Generate phase

`wikiforge generate` (or `orchestrator.Generate()`) runs the full adaptive pipeline:

1. **State initialization** — Load or create run state (version 3). If resuming, preserve completed page statuses.
2. **Adaptive planning** — Run discovery and planning for each component to determine the page set.
3. **Evidence preparation** — Build the evidence index from the repository: tracked file identity, content hashes, changed paths, and documentation attachments.
4. **Instructions writing** — Write `openwiki/INSTRUCTIONS.md` with component identity, profile, canonical page roadmap, and adaptive views.
5. **Repository grouping** — Group components by repository path. Same-repo components are serialized (or isolated with `execution.isolateSameRepository`).
6. **Worker dispatch** — Bounded goroutine workers (up to `execution.parallelComponents`) process repository groups.
7. **Per-component adaptive generation** — For each component, iterate planned pages and invoke OpenWiki for each:
   - Quickstart page uses `op="init"` (first-time) or `op="prompt"` (update).
   - Other pages use `op="prompt"` with an adaptive phase prompt.
8. **Validation** — After all component pages complete, run adaptive validation (hierarchy, links, evidence, Mermaid).
9. **Repair rounds** — If validation fails below threshold, run targeted repair prompts (up to `execution.maxRepairRounds`).
10. **Whole-system aggregation** — Generate system wiki from component documentation snapshots using content-addressed reuse.
11. **Graph export** — Export JSONL knowledge graph with evidence edges.
12. **Report writing** — JSON report and Markdown summary under `.wikiforge/reports/<runID>/`.

### 3. Update phase

`wikiforge update` runs the same pipeline with surgical no-op detection:

- Preserves last successful evidence revision, source hash, and docs hash.
- Compares current Git HEAD against previous evidence revision to detect changed paths.
- If the source hash matches and docs hash matches, all phases are skipped.
- If only some paths changed, the impact index identifies affected pages for targeted updates.
- Starts a new run ID while retaining previous state for scoped comparison.

### 4. Resume phase

`wikiforge resume` loads the existing state and resumes from the last incomplete page. Completed pages are skipped. Failed pages are retried up to `maxProcessRetries`. Designed for recovery after Ctrl+C or transient failures.

## Adaptive component generation

For each component, the orchestrator executes the adaptive plan produced by the planner:

```text
[Component X]
  ├─ [discovery]             → discovery.json
  ├─ [planning]              → plan.json
  ├─ [evidence indexing]     → evidence-index.json, impact-index.json
  ├─ [INSTRUCTIONS.md]       → written once
  ├─ [adaptive pages...]
  │    ├─ quickstart.md  (op=init)
  │    ├─ components/<id>/index.md
  │    ├─ components/<id>/architecture.md
  │    ├─ components/<id>/contracts.md
  │    ├─ components/<id>/data-and-consistency.md
  │    ├─ components/<id>/runtime-and-operations.md
  │    ├─ domains/<domain>/*.md
  │    ├─ flows/<flow>.md
  │    ├─ catalogs/<collection>/*.md
  │    └─ ... (planned pages per view/pack)
  ├─ [validate]      → adaptive validation + evidence verification
  ├─ [repair rounds] → up to maxRepairRounds
  ├─ [graph export]  → nodes.jsonl + edges.jsonl
  └─ [finalize]      → coverage.json, checkpoint state
```

### Page execution

Each adaptive page:

1. **Render prompt** — Combine base template, adaptive page contract, profile guidance, unit context, and view/pack metadata into a complete prompt.
2. **Externalize prompt** — Write prompt to a temporary `.wikiforge-prompt-*.md` file in the component `openwiki/` directory.
3. **Bridge invocation** — Pass a single-line JSON virtual-path reference (`/openwiki/.wikiforge-prompt-<hash>.md`) to OpenWiki via `--print`.
4. **Capture output** — Stream stdout/stderr from OpenWiki with heartbeat.
5. **Validate** — After exit, validate the generated documentation for the adaptive plan.
6. **Repair if needed** — If validation fails below threshold and repair rounds remain, run a targeted repair prompt.
7. **Cleanup** — Remove the temporary prompt file.

## Evidence index and change impact

[`internal/evidence/`](/internal/evidence/) provides the evidence subsystem:

- **`BuildIndexCached`** — Builds an evidence index with cached file identity, content hashes, symlink escape protection, and configurable include/exclude roots.
- **`ChangedPaths`** — Compares current Git HEAD against a previous revision to detect changed files.
- **`BuildImpactWithPrevious`** — Maps source change paths to affected documentation pages using evidence dependencies.
- **`BuildCoverage`** — Tracks which documentation pages are backed by which evidence sources.
- **`AttachDocumentation`** — Attaches existing documentation files to the evidence index for impact analysis.

Artifacts are persisted under `.wikiforge/components/<id>/`:

| File | Purpose |
|---|---|
| `discovery.json` | Repository discovery manifest |
| `plan.json` | Adaptive page plan |
| `evidence-index.json` | File identity, content hashes, documentation index |
| `impact-index.json` | Changed paths → affected pages mapping |
| `coverage.json` | Source-to-documentation coverage map |

## Isolated component execution

When `execution.isolateSameRepository` is true (default), components sharing a Git repository are executed in isolated working directories to avoid conflicting OpenWiki writes to agent files. The orchestrator creates a staging directory and copies the relevant source tree for each component before execution.

## Validation and repair

After all component pages complete, WikiForge runs `ValidateAdaptiveComponent` which:

1. Checks every planned page exists and satisfies its `AdaptivePageContract`.
2. Validates the page hierarchy (index → collection → shard link structure).
3. Runs source-path verification, front matter checks, Mermaid rendering, and evidence-backed verification.
4. Scores findings (errors -10/-25, warnings -5) against the `minimumQualityScore` threshold.

See [Testing and validation](../operations/testing-validation.md) for the complete validation rule set.

## Whole-system aggregation

After all components complete, if `system.enabled` is true:

1. Component wikis are content-addressed — unchanged documentation is reused across generations.
2. Authoritative facts from `system.factsPath` are optionally included.
3. A whole-system adaptive plan is generated with system-level views (landscape, component map, cross-component flows, etc.).
4. The system wiki validates, repairs, and exports a knowledge graph independently.

## Reporting

After generation, `report.Write()` produces:

- **`report.json`** — Full structured report with per-component validation results, system results, and failures.
- **`summary.md`** — Human-readable Markdown summary with component scores and acceptance status.
- **`latest.txt`** — Points to the most recent report directory.

Reports are stored under `.wikiforge/reports/<runID>/`.

## Knowledge graph export

[`internal/graph/graph.go`](/internal/graph/graph.go) walks documentation directories with evidence context and produces two JSONL files:

- **`nodes.jsonl`** — Document nodes and concept nodes with evidence references.
- **`edges.jsonl`** — Typed relationship edges (LINKS_TO, DEPENDS_ON, OWNED_BY, etc.) with evidence, authority, and confidence fields.

## Source map

| File | Role |
|---|---|
| `/internal/orchestrator/orchestrator.go` | Core orchestration, generate/update/resume |
| `/internal/orchestrator/adaptive.go` | Adaptive component and system execution |
| `/internal/orchestrator/evidence_artifacts.go` | Evidence preparation, impact, coverage |
| `/internal/planner/planner.go` | Repository discovery and adaptive page planning |
| `/internal/evidence/evidence.go` | Evidence index, change impact, coverage |
| `/internal/graph/graph.go` | JSONL knowledge graph export with evidence |
| `/internal/report/report.go` | Report writing (JSON + Markdown) |
| `/internal/state/store.go` | Persistent state store |
| `/internal/validation/validation.go` | Adaptive validation engine |
| `/internal/openwiki/runner.go` | Phase execution via OpenWiki |
