# Build Verification

Build: `1.3.0` — Phase 1 adaptive-planning foundation

## Verified in this build environment

- `go test -count=1 ./...`
- `go test -race -count=1 ./...`
- `go vet ./...`
- Configuration v3 normalization with v1/v2 compatibility and reloadable migration output.
- Deterministic discovery manifests with include/exclude globs, file-size and binary guards, symlink avoidance, nested module/domain inference, BPMN flow inference, and configured-unit precedence.
- Adaptive planner coverage for every registered capability pack, all documentation views, documentation-unit outputs, explicit collision decisions, catalog shard policy, and whole-system aggregation.
- CLI coverage for `discover`, `plan --explain`, `config migrate`, unknown-component failures, and deterministic artifact persistence.
- Last-successful source, documentation, discovery, and plan checkpoints survive failed generation attempts.
- Published v3 YAML examples parse through WikiForge and validate against the Draft 2020-12 JSON Schema.
- OpenWiki executable runner contract tests:
  - `--init --print`;
  - `--update --print`;
  - phased prompt mode;
  - `--modelId` propagation;
  - help/doctor command boundary;
  - live stdout/stderr forwarding;
  - quiet-process heartbeat;
  - non-interactive child stdin;
  - temporary prompt cleanup.
- Large-prompt transport regression:
  - complete 160,000-character prompt stored in a temporary UTF-8 file;
  - spawned command arguments remain below 4 KiB;
  - helper process reads the complete original prompt;
  - path is absolute and appears between explicit marker lines;
  - no quote, backslash, CR, or LF is embedded in the transported path value.
- Cross-platform path tests:
  - Windows drive path;
  - Windows extended-length drive path;
  - Windows UNC path;
  - Windows extended-length UNC path;
  - macOS path with spaces;
  - Linux path with Unicode;
  - mixed slash/backslash repository scope;
  - absolute-scope and parent-escape rejection;
  - portable component-ID validation;
  - Mermaid input/output placeholder preservation.
- Doctor prompt-transport preflight:
  - temporary file creation;
  - absolute external-tool path conversion;
  - reopen through converted path;
  - cleanup.
- Deterministic invocation/path failures skip retries; transient provider errors remain retryable.
- Documentation contract tests and end-to-end fake-runner orchestration for every profile, monorepo scopes, whole-system aggregation, validation, repair, reports, graph export, state, and update no-op behaviour.
- Cross-compilation:
  - Linux amd64 and arm64;
  - Windows amd64 and arm64;
  - macOS amd64 and arm64.

## External integration boundary

The production runner invokes the configured OpenWiki CLI. The default configuration remains pinned to `openwiki@0.2.0` through `npx`, and Mermaid rendering remains pinned to `@mermaid-js/mermaid-cli@11.12.0`.

A live model-backed generation was not executed in this build environment because no model-provider credential was supplied. The executable boundary, prompt-file transport, arguments, streaming, heartbeat, cancellation, timeout, retry classification, phased orchestration, validation, repair, and aggregation are covered by automated tests.
