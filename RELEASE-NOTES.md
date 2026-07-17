# WikiForge 1.2.3 Release Notes

## Cross-platform prompt-path transport fix

WikiForge 1.2.2 externalized large prompts successfully, but its bridge instruction could lead OpenWiki to include quotation marks around the absolute Windows path. The downstream filesystem tool then received a value such as:

```text
"C:\Users\name\project\.wikiforge-prompt-123.md"
```

The quotes became part of the tool argument, causing `path must be absolute` even though the underlying path was absolute.

WikiForge 1.2.3 now transports the prompt path as:

```text
BEGIN_WIKIFORGE_PROMPT_PATH
C:/Users/name/project/.wikiforge-prompt-123.md
END_WIKIFORGE_PROMPT_PATH
```

The bridge explicitly requires the exact characters between the markers and prohibits quotes, backticks, URI prefixes, escapes, marker text, and surrounding whitespace.

## Path normalization and portability

The path layer now handles:

- Windows drive paths, including paths containing spaces and Unicode;
- Windows UNC shares;
- Windows extended-length drive and UNC prefixes;
- macOS and Linux absolute paths;
- `/` and `\` in configured repository scopes;
- symlink and junction resolution for the OpenWiki working directory;
- absolute workspace, repository, system-output, and facts paths;
- quote-free absolute Mermaid input/output arguments;
- portable component IDs used as directory names in reports, graph output, and system snapshots.

Unsafe relative escapes, absolute scopes, control characters, double quotes, Windows reserved device names, trailing dots/spaces, and path separators inside component IDs are rejected during configuration validation.

## Doctor preflight

`wikiforge doctor` now validates workspace, system, repository, and scoped component paths. For every enabled component it also performs the exact prompt-file transport sequence used by generation: create, normalize, reopen, and clean up.

## Retry behaviour

Deterministic local failures such as command-line overflow, invalid absolute-path transport, and prompt externalization errors now stop immediately rather than consuming all process retry attempts. Transient provider and network failures remain retryable.

## Compatibility

Configuration version remains `2`. Documentation contracts and phase IDs are unchanged from 1.2.2, so existing 1.2.2 checkpoints can be resumed with 1.2.3.
