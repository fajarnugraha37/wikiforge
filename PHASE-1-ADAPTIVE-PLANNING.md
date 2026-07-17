# Phase 1: Adaptive Planning Foundation

Phase 1 changes WikiForge's planning model without replacing the existing validated profile renderer. The compatibility renderer remains active; discovery and planning artifacts now provide the bounded inputs required by the hierarchical renderer planned for Phase 2.

## Implemented architecture

```text
ComponentConfig v3
  -> deterministic source discovery
    -> DiscoveryManifest
      -> composable profile + explicit + discovered capability packs
        -> DocumentationUnit set
          -> adaptive DocumentationPlan
            -> persisted artifacts
              -> component/system prompts and immutable aggregation snapshots
```

## Configuration v3

Version 3 adds:

- `components[].owners`, `capabilities`, and `packs`;
- top-level `documentationUnits`;
- `documentation.views`;
- `documentation.catalogs.shardBy` and `maximumRowsPerPage`;
- `documentation.evidence.include`, `exclude`, and `maxFileSizeBytes`.

Version 1 services and version 2 components are accepted and normalized in memory to version 3. `wikiforge config migrate` emits a normalized version 3 JSON configuration.

## Documentation units

A documentation unit is not assumed to be deployable. Supported kinds are:

- domain;
- subdomain;
- bounded context;
- component;
- module;
- flow;
- platform area;
- catalog.

Configured units preserve source roots, related units, output path, owners, capabilities, and criticality. Discovery can infer module/domain units from conventional roots and flow units from BPMN files. Explicit units take precedence and prevent duplicate inferred units.

## Capability packs

The planner composes:

1. base profile packs;
2. explicit component packs;
3. packs discovered from source evidence.

Every supported pack has a canonical Phase 1 planning outcome. A pack is included, skipped with a reason, or deferred because its required view is disabled. No registered pack is silent or dead configuration.

## Deterministic discovery

Discovery:

- walks only the configured component scope;
- applies configurable include/exclude globs;
- skips excluded directories, oversized files, binary files, and symbolic links;
- records evidence paths per capability pack;
- calculates a stable source hash;
- emits no timestamps or map-order-dependent content.

Artifacts are written to:

```text
.wikiforge/components/<component-id>/discovery.json
.wikiforge/components/<component-id>/plan.json
.wikiforge/system/plan.json
```

The whole-system aggregation workspace snapshots component discovery/plan files and `sources/system-plan.json` next to generated component wiki snapshots.

## CLI

```text
wikiforge discover [--config PATH] [--component ID]
wikiforge plan [--config PATH] [--component ID] [--skip-system] [--explain]
wikiforge config migrate [--config PATH] [--output PATH] [--force]
```

`plan --explain` reports selected packs, planned page kinds, collection shard policy, and every skip/defer reason. `discover` and `plan` persist deterministic artifacts.

## Runtime integration

Component and system prompts receive:

- selected capability packs;
- documentation units;
- future adaptive page paths;
- include/skip/defer decisions.

Persistent `INSTRUCTIONS.md` files carry the same context. The current renderer still owns its legacy profile page for each phase, preventing Phase 1 from silently changing the physical wiki contract before Phase 2 validation and migration are available.

## Verification scope

Automated coverage includes:

- v1/v2 to v3 normalization and reloadable migration output;
- documentation-unit reference, path, pack, and view validation;
- deterministic discovery and source-hash changes;
- evidence include/exclude, root-level `**` globs, binary, size, and symlink guards;
- configured capability units, inferred module/domain units, BPMN flow inference, and deduplication;
- canonical planning outcome for every capability pack;
- disabled-view deferral and explicit reasoning;
- different relevant plans for monolith, modular monolith, microservice, library, framework, and infrastructure fixtures;
- system-plan aggregation;
- CLI artifact creation and `--explain` output;
- prompt placeholder resolution;
- end-to-end generation, immutable system snapshots, no-op update behavior, validation, graph export, and monorepo serialization.
