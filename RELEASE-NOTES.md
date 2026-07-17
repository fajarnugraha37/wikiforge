# WikiForge 1.3.0 Release Notes

## Adaptive planning foundation

WikiForge now separates deployable components from documentation units and builds a deterministic discovery manifest plus adaptive documentation plan before generation. Configuration version 3 adds documentation views, composable capability packs, evidence boundaries, catalog shard policy, ownership/capability metadata, and explicit domain/module/flow/platform/catalog units.

New commands:

```text
wikiforge discover
wikiforge plan --explain
wikiforge config migrate
```

Version 1 and 2 configurations remain accepted and are normalized to version 3. Existing fixed profile phases remain the compatibility renderer during Phase 1; prompts and system aggregation now consume the adaptive plan so Phase 2 can change the physical layout without discarding the orchestration core.

## Correctness and safety

- all registered capability packs have include, skip, or defer outcomes;
- output-path collisions are explicit planning decisions rather than silent drops;
- discovery is deterministic, bounded by configurable evidence rules, and does not follow symbolic links;
- discovery and plan artifacts are persisted and included in whole-system snapshots;
- failed runs preserve the last-successful source, documentation, discovery, and plan checkpoint;
- JSON Schema, examples, CLI workflows, profile fixtures, orchestration, race detection, vet, build, and cross-platform CI are verified.

---

## WikiForge 1.2.3

## GitHub Releases and downloadable binaries

WikiForge now includes a dedicated tag-triggered GitHub Actions release workflow. Pushing a semantic version tag such as `v1.2.3` runs tests and GoReleaser, creates the GitHub Release, and uploads archives for:

- Windows amd64 and arm64;
- Linux amd64 and arm64;
- macOS amd64 and arm64;
- SHA-256 checksums.

The ordinary `ci` workflow intentionally remains a verification workflow. A successful branch or push build does not create a GitHub Release by itself.

## Non-interactive OpenWiki prompt bridge

Large WikiForge prompts remain stored in temporary UTF-8 files, but the short OpenWiki bridge is now a strictly single-line JSON reference:

```text
WIKIFORGE_PROMPT_REF={"path":"C:/absolute/path/.wikiforge-prompt-123.md"} ...
```

This avoids embedded-newline handling differences in Windows `cmd.exe` and `npx.cmd`. OpenWiki is explicitly instructed to:

- parse the JSON path;
- read that exact absolute file immediately;
- execute the complete specification;
- avoid searching for `wikiforge.yaml` as a replacement;
- avoid merely summarizing the specification;
- avoid asking the user for clarification.

The temporary prompt is still removed after success, failure, timeout, or cancellation.

## Semantic success detection

An OpenWiki process can exit with code `0` without executing the requested phase. Responses such as “Could you clarify?”, “Where is the file?”, or “What would you like me to do?” are now classified as semantic failures.

WikiForge no longer marks those phases completed. The response is retried according to the configured process retry policy and is reported as a failure if all attempts return clarification instead of performing the task.

## Validation and Mermaid progress

After a repair process exits, WikiForge immediately performs another complete validation pass. With Mermaid mode `render`, that pass can launch many sequential Mermaid CLI processes and previously produced no console output.

Validation now reports:

- validation-pass start and completion;
- current Markdown file and file count;
- Mermaid render start and completion;
- a 15-second Mermaid heartbeat;
- final score, finding count, and acceptance status.

This makes the period after `repair-N OpenWiki process completed` observable instead of appearing frozen.

## Cross-platform path safety retained

Prompt and Mermaid paths remain absolute and normalized for Windows, macOS, and Linux, including drive paths, UNC paths, spaces, Unicode, mixed separators, symlinks, and Windows junctions. `wikiforge doctor` continues to run prompt-transport preflight checks.

## Compatibility

Configuration version remains `2`. Canonical documentation contracts and phase IDs are unchanged. Existing checkpoints remain readable, but phases previously marked completed after clarification-only responses must be invalidated or regenerated because those completion records are false positives from the older runner.
