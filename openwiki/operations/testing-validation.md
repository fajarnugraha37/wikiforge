---
type: Playbook
title: Testing and Validation
description: WikiForge's testing pyramid, validation rules, Mermaid diagram checks, and end-to-end orchestration tests
tags: [testing, validation, mermaid, e2e]
resource: /internal/validation/validation.go
---

# Testing and Validation

WikiForge has a comprehensive testing and validation system covering unit tests, contract tests for adaptive documentation profiles, end-to-end orchestration tests, an adaptive validation engine that checks generated documentation quality, and evidence-backed validation.

## Test pyramid

### Unit tests

| Package | Test file | Coverage |
|---|---|---|
| `config` | `config_test.go` | Config loading, normalization, v1→v2→v3 migration |
| `pathutil` | `pathutil_test.go` | Cross-platform path normalization |
| `prompts` | `adaptive_test.go` | Adaptive page contracts and rendering |
| `evidence` | `evidence_test.go` | Evidence index, change detection, impact |
| `graph` | `graph_test.go` | Knowledge graph export |
| `validation` | `validation_test.go` | Validation rules, front matter, Mermaid, adaptive |
| `orchestrator` | `orchestrator_test.go` | Full pipeline with fake runner |
| `orchestrator` | `adaptive_test.go` | Adaptive component and system execution |
| `orchestrator` | `progress_test.go` | Progress bar display |
| `openwiki` | `runner_test.go` | Bridge contract, semantic failure detection, prompt transport |
| `benchmark` | `benchmark_test.go` | Phase 4 benchmark tests |

### Contract tests

The validation package includes adaptive contract tests that verify each profile can satisfy its adaptive page contract. Tests ensure:

- Profile identity metadata is valid.
- Adaptive page contracts are internally consistent.
- Hierarchical page structure validates correctly.

### End-to-end orchestration tests

`internal/orchestrator/orchestrator_test.go` and `adaptive_test.go` exercise the full pipeline:

- Adaptive discovery, planning, and page generation.
- All 7 profiles with adaptive execution.
- Monorepo serialization verification (same-repo components never run concurrently).
- Whole-system aggregation with content-addressed snapshots.
- `fakeRunner` simulates OpenWiki without model calls.

Additional tests cover:
- `TestSameRepositoryComponentsAreSerialized` — Verifies serialization within a repo.
- `TestAdaptiveComponent` — Adaptive discovery, planning, and page execution.
- Call counting and phase completion tracking.

## Validation engine

[`internal/validation/validation.go`](/internal/validation/validation.go) provides the `Validator` with adaptive validation methods:

```go
func (v Validator) ValidateAdaptiveComponent(ctx context.Context, component config.ComponentConfig, plan planner.ComponentPlan) model.ValidationResult
func (v Validator) ValidateAdaptiveSystem(ctx context.Context, root string, plan planner.SystemPlan) model.ValidationResult
```

### Validation rules

| Code | Severity | Check |
|---|---|---|
| `DOC-MISSING` | error | Required canonical document is missing |
| `DOC-PAGE-COUNT` | error | Markdown files below profile minimum |
| `DOC-READ` | error | Cannot read Markdown file |
| `DOC-FRONTMATTER` | error | Missing or empty required front matter field (`type`, `title`, `description`, `tags`) |
| `DOC-FRONTMATTER-UNSUPPORTED` | error | Unsupported front matter field present |
| `DOC-PLACEHOLDER` | warning | Contains TODO/TBD |
| `DOC-SOURCE-SECTION` | error | Missing or empty `## Source References` section |
| `DOC-SOURCE-EMPTY` | warning | Source References has no recognizable path/link |
| `DOC-SOURCE-PATH` | warning | Source reference path does not resolve in scope |
| `DOC-BROKEN-LINK` | error | Relative Markdown link target does not exist |
| `DOC-REQUIRED-SECTION` | error | Missing required heading from contract |
| `DOC-EMPTY-SECTION` | error | Required section is empty |
| `DOC-REQUIRED-TABLE` | error | Required catalog table header not found |
| `DOC-CATALOG-EMPTY` | error | Catalog table has no data rows |
| `DOC-CATALOG-IDENTITY` | error | Catalog entries missing stable identity (from evidence) |
| `DOC-CATALOG-BOUNDS` | error | Catalog exceeds configured row or byte bounds |
| `DOC-RELATIONSHIP-VOCABULARY` | error | Relationship vocabulary not aligned with plan |
| `DOC-NAV-STRUCTURE` | error | Navigation hierarchy contradicts planned page tree |
| `DOC-DUPLICATE-CONCEPT` | warning | Concept appears under multiple owners without explanation |
| `DOC-ABSOLUTE-LINK` | error | Link uses absolute host path instead of relative |
| `DOC-EVIDENCE-UNAVAILABLE` | warning | Claim lacks backed evidence |
| `DOC-SECRET-LEAK` | error | Possible credential or private key detected |
| `DOC-HIERARCHY` | error | Page hierarchy mismatch (index/collection/shard structure) |
| `MERMAID-TYPE` | error | Unsupported or missing diagram type |
| `MERMAID-BASIC` | error | Basic Mermaid syntax error |
| `MERMAID-RENDER` | error | Mermaid CLI rendering failure |
| `MERMAID-REQUIRED` | error | Required diagram for page is missing |
| `MERMAID-CONTRACT` | error | Required diagram type not found in page |

### Mermaid validation modes

Three modes controlled by `mermaid.mode`:

| Mode | Behaviour | Use case |
|---|---|---|
| `render` | Full parse + render with Mermaid CLI | Production validation with visual verification |
| `basic` | Structural checks only (no CLI) | CI without headless browser dependency |
| `off` | No Mermaid validation | Quick local development |

In `render` mode, validation:
1. Parses Mermaid blocks from Markdown.
2. Checks diagram type against allowed list (flowchart, sequenceDiagram, stateDiagram-v2, erDiagram, classDiagram, architecture-beta, gitGraph, mindmap).
3. Runs basic syntax checks.
4. Renders each diagram with Mermaid CLI (headless Chromium via Puppeteer).
5. Reports rendering failures as errors.

### Scoring and acceptance

- Initial score: 100
- Errors: -10 points each (severe: -25 for missing profile/document-level issues)
- Warnings: -5 points each
- **Accepted** when: `score >= minimumQualityScore` (default 85) AND no unresolved errors AND required page count met

### Validation progress reporting

During validation, WikiForge reports:
- Start and completion timestamps
- Current file and file index
- Mermaid render progress (with 15-second heartbeat)
- Final score, finding count, and acceptance status

## Running tests

```bash
# All tests
go test ./...

# With race detection (Linux)
go test -race ./...

# Specific package
go test ./internal/validation/...

# End-to-end (requires fixture repos)
go test ./internal/orchestrator/... -run TestGenerate
```

## Change guidance

When modifying validation rules or adaptive profiles:

1. Update `validation.go` and `validation_test.go` for new rules.
2. Update adaptive contract tests in `validation_test.go` if profile contracts change.
3. Update `orchestrator/adaptive_test.go` and `orchestrator/orchestrator_test.go` if pipeline structure changes.
4. Update `evidence/evidence.go` and `evidence_test.go` if evidence-index rules change.
5. Run the full test suite to verify no regressions.
6. Update `BUILD-VERIFICATION.md` for significant validation additions.

## Source map

| File | Role |
|---|---|
| `/internal/validation/validation.go` | Validation engine with all rules |
| `/internal/validation/validation_test.go` | Contract tests and edge-case tests |
| `/internal/evidence/evidence.go` | Evidence index and coverage |
| `/internal/evidence/evidence_test.go` | Evidence index tests |
| `/internal/orchestrator/orchestrator_test.go` | End-to-end pipeline tests |
| `/internal/orchestrator/adaptive_test.go` | Adaptive execution tests |
| `/internal/openwiki/runner_test.go` | Bridge and runner contract tests |
| `/internal/pathutil/pathutil_test.go` | Cross-platform path tests |
| `/internal/prompts/adaptive_test.go` | Adaptive page contract tests |
| `/internal/config/config_test.go` | Config loading tests |
| `/internal/orchestrator/progress_test.go` | Progress bar tests |
