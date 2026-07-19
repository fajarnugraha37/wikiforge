---
type: Reference
title: OpenWiki Bridge
description: WikiForge's protocol for invoking OpenWiki child processes, including prompt externalization, single-line bridge, live output streaming, heartbeat, and cross-platform path safety
tags: [openwiki, bridge, child-process, prompts, cross-platform]
resource: /internal/openwiki/runner.go
---

# OpenWiki Bridge

WikiForge invokes OpenWiki as a child process using the `ExecRunner` defined in [`internal/openwiki/runner.go`](/internal/openwiki/runner.go). The bridge is designed to handle large prompts, cross-platform path differences, and long-running generation phases reliably.

## Prompt externalization

Large phase prompts (up to ~160 KB) are not passed directly on the command line. Instead:

1. WikiForge writes the complete rendered prompt to a temporary file: `.wikiforge-prompt-<hash>.md` in the component's `openwiki/` subdirectory.
2. A short single-line JSON bridge instruction is passed to OpenWiki through `--print`:

```
WIKIFORGE_PROMPT_REF={"path":"/openwiki/.wikiforge-prompt-abc123.md"} This is a non-interactive WikiForge page task. Parse the JSON object after WIKIFORGE_PROMPT_REF, take only its path string value without quotation marks, and immediately use the filesystem read tool to read that exact absolute virtual UTF-8 file. The path is rooted at the repository virtual filesystem and begins with /openwiki/; do not convert it to a host path. Execute every instruction in that file now. Do not ask for clarification, do not search for another specification such as wikiforge.yaml, do not merely summarize the file, and do not modify, document, move, or delete the prompt file.
```

3. OpenWiki parses the JSON, reads the exact file at the **absolute virtual path** `/openwiki/...` (rooted in the OpenWiki repository virtual filesystem), and executes the full specification.
4. The temporary file is removed after the process exits (success, failure, timeout, or cancellation).

This approach avoids:
- Windows `cmd.exe` and `npx.cmd` command-line length limits.
- Embedded-newline handling differences across shells.
- Quote and backslash escaping issues.
- UNC prefix (`\\?\`) conflicts on Windows long paths.

The key change from earlier versions is the switch from host filesystem paths to absolute virtual paths (`/openwiki/...`). This eliminates all host-OS path encoding issues at the cost of requiring OpenWiki's prompt transport to resolve virtual paths to host paths. See `promptHostPath` in [`runner.go`](/internal/openwiki/runner.go) for the reverse mapping logic.

## Runner interface

[`internal/openwiki/runner.go`](/internal/openwiki/runner.go) defines the `Runner` interface:

```go
type Runner interface {
    Run(ctx context.Context, workdir string, operation string, prompt string) (string, error)
    Check(ctx context.Context) error
}
```

- `Run` — Executes an OpenWiki operation (init, update, or prompt) in the specified working directory.
- `Check` — Verifies the OpenWiki command is available by running `openwiki --help`.

### ExecRunner

The default implementation `ExecRunner` provides:

- **Timeout control** — Configurable timeout (default 60 minutes) via `Context.WithTimeout`.
- **Prompt path externalization** — The `externalizePrompt` function writes the prompt to a temp file and returns the single-line bridge reference.
- **Live output streaming** — OpenWiki stdout and stderr are piped and streamed immediately with the component/phase label.
- **Heartbeat** — If the child process produces no output for 15 seconds, WikiForge prints a heartbeat with elapsed and quiet time.
- **Semantic failure detection** — Responses like "Could you clarify?" are detected as semantic failures even with exit code 0.

## Process operations

The `operation` parameter controls how OpenWiki is invoked:

| Operation | CLI flag | Description |
|---|---|---|
| `init` | `--init --print <bridge>` | First-time generation of a component or system |
| `update` | `--update --print <bridge>` | Incremental refresh of existing docs |
| `prompt` | `--print <bridge>` | Arbitrary prompt execution (used for repair) |

The model ID can be appended with `--modelId` if configured.

## Cross-platform path safety

All paths crossing the external-tool boundary are normalized by [`internal/pathutil/pathutil.go`](/internal/pathutil/pathutil.go):

- Absolute paths use forward slashes on Windows (accepted by Node).
- Extended-length `\\?\` prefixes are removed before transport.
- Unicode paths, spaces, symlinks, and junctions are preserved.
- Temporary prompt files are placed inside the component's `openwiki/` directory to ensure the virtual path `/openwiki/...` has a valid host-side counterpart.
- The `doctor` command runs a prompt-transport preflight (`CheckPromptTransport`) for every enabled component, verifying that the virtual path round-trips correctly.

## Doctor preflight check

`wikiforge doctor` performs for each component:

1. Creates a temporary prompt file.
2. Converts the path to the external-tool representation.
3. Reopens the file through that representation.
4. Removes the temporary file.

This validates the full prompt transport pipeline before any generation.

## Environment variables

OpenWiki receives the configured environment from `openwiki.environment` in the config. By default:

- `OPENWIKI_TELEMETRY_DISABLED=1` — Disables telemetry.
- `OPENWIKI_PROVIDER_RETRY_ATTEMPTS=3` — Provider-level retries.

Provider credentials must be set separately as environment variables (e.g., `OPENWIKI_PROVIDER`, `OPENAI_COMPATIBLE_API_KEY`, `OPENWIKI_MODEL_ID`). WikiForge does not store API keys in its generated configuration.

## Cancellation and cleanup

- `Ctrl+C` cancels the current OpenWiki child process.
- The checkpoint state remains available for `wikiforge resume`.
- Temporary prompt files are always cleaned up, even after cancellation.

## Source map

| File | Role |
|---|---|
| `/internal/openwiki/runner.go` | Runner interface, ExecRunner implementation, prompt externalization with virtual paths |
| `/internal/openwiki/runner_test.go` | Bridge contract tests (single-line prompt, clarification rejection, prompt transport) |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization for external tool boundaries |
| `/internal/pathutil/pathutil_test.go` | Path normalization tests |
