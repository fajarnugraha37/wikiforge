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

1. WikiForge writes the complete rendered prompt to a temporary file: `.wikiforge-prompt-<hash>.md` in the component working directory.
2. A short single-line JSON bridge instruction is passed to OpenWiki through `--print`:

```
WIKIFORGE_PROMPT_REF={"path":"C:/absolute/path/.wikiforge-prompt-abc123.md"}
```

3. OpenWiki parses the JSON, reads the exact file at that path, and executes the full specification.
4. The temporary file is removed after the process exits (success, failure, timeout, or cancellation).

This approach avoids:
- Windows `cmd.exe` and `npx.cmd` command-line length limits.
- Embedded-newline handling differences across shells.
- Quote and backslash escaping issues.

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
- The `doctor` command runs a prompt-transport preflight for every enabled component.

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
| `/internal/openwiki/runner.go` | Runner interface, ExecRunner implementation, prompt externalization |
| `/internal/openwiki/runner_test.go` | Bridge contract tests (single-line prompt, clarification rejection) |
| `/internal/pathutil/pathutil.go` | Cross-platform path normalization for external tool boundaries |
