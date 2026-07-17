---
type: Reference
title: WikiForge Configuration Model
description: Configuration v3, compatibility migration, components, documentation units, views, capability packs, evidence boundaries, and validation
tags: [configuration, yaml, components, documentation-units, adaptive-planning]
resource: /internal/config/config.go
---

# Configuration Model

WikiForge loads YAML or JSON from `wikiforge.yaml` by default. The normalized in-memory model is version 3. Version 1 `services` and version 2 `components` remain accepted and are converted before validation.

## Config structure

```yaml
version: 3
workspace: .
openwiki: { ... }
execution: { ... }
documentation:
  language: English
  views: [system, domain, component, flow, catalog, platform, engineering, operations]
  catalogs:
    shardBy: [domain, owner]
    maximumRowsPerPage: 150
  evidence:
    include: ["**"]
    exclude: [.git/**, .wikiforge/**, openwiki/**, vendor/**, node_modules/**, generated/**]
    maxFileSizeBytes: 2097152
mermaid: { ... }
components: [ ... ]
documentationUnits: [ ... ]
system: { ... }
```

## OpenWiki config

| Field | Default | Description |
|---|---|---|
| `command` | `npx` | OpenWiki executable |
| `args` | `--yes openwiki@0.2.0 code` | Child-process arguments |
| `modelId` | empty | Optional model override |
| `timeoutMinutes` | `60` | Timeout per process |
| `environment` | telemetry disabled, provider retries `3` | Additional environment variables |

## Execution config

| Field | Default | Description |
|---|---:|---|
| `parallelComponents` | 2 | Maximum repository groups processed concurrently |
| `maxProcessRetries` | 2 | Additional retry attempts for retryable child-process failures |
| `maxRepairRounds` | 2 | Maximum targeted validation-repair rounds |
| `continueOnComponentFailure` | true | Continue processing independent components after failure |

Legacy `parallelServices` and `continueOnServiceFailure` are read and normalized.

## Documentation config

The existing quality and Mermaid-related fields remain. Version 3 adds the planning fields below.

### Views

Supported values:

- `system`;
- `domain`;
- `component`;
- `flow`;
- `catalog`;
- `platform`;
- `engineering`;
- `operations`.

Disabling a view does not silently discard a selected capability. The planner records a `defer` decision for pages or units that require that view. `quickstart.md` remains planned as the bounded component entry point even when the detailed component view is disabled.

### Catalog policy

| Field | Default | Description |
|---|---|---|
| `shardBy` | `domain`, `owner` | Future collection partition dimensions |
| `maximumRowsPerPage` | `150` | Future maximum rows before a collection is sharded |

Supported shard dimensions are `domain`, `subdomain`, `bounded-context`, `component`, `owner`, `repository`, `runtime`, `transport`, `data-store`, and `criticality`.

### Evidence policy

| Field | Default | Description |
|---|---|---|
| `include` | `**` | Relative globs eligible for deterministic discovery |
| `exclude` | Git, WikiForge, generated, dependency, and build directories | Relative globs excluded from discovery |
| `maxFileSizeBytes` | 2 MiB | Per-file scan guard |

Discovery does not follow symbolic links and skips binary or oversized files. A missing component root is an error; an unreadable child path is recorded as an unknown evidence gap.

## Component configuration

```yaml
components:
  - id: commerce-core
    type: modular-monolith
    repository: ./repositories/commerce-core
    scope: .
    enabled: true
    includeInSystem: true
    group: commerce
    tags: [core]
    dependsOn: [shared-contracts]
    owners: [commerce-team]
    capabilities: [order-management, pricing]
    packs: [workflow, messaging, database]
```

`repository` and `scope` define the runtime source boundary. They do not define the documentation decomposition.

### Type-to-profile mapping

| Types | Profile |
|---|---|
| `monolith`, `microservice`, `worker`, `gateway`, `frontend`, `cli` | `application` |
| `modular-monolith` | `modular-application` |
| `library`, `shared-library`, `internal-library`, `framework`, `sdk` | `reusable` |
| `iac`, `infrastructure`, `gitops`, `deployment`, `platform` | `infrastructure` |
| `configuration`, `shared-config`, `config` | `configuration` |
| `contract`, `contracts`, `schema`, `schemas` | `contracts` |
| `generic`, `repository` | `generic` |

An explicit `profile` may override the type mapping when it names a supported profile.

## Documentation units

```yaml
documentationUnits:
  - id: order-management
    component: commerce-core
    kind: domain
    sourceRoots: [modules/order, workflows/order]
    relatedUnits: [pricing, fulfilment/dispatch]
    output: domains/order-management
    owners: [commerce-team]
    capabilities: [order-management]
    criticality: high
```

Supported kinds are `domain`, `subdomain`, `bounded-context`, `component`, `module`, `flow`, `platform`, and `catalog`.

Invariants:

- IDs are unique within a component, not globally;
- cross-component relations use `component/unit` when necessary;
- a unit must reference an enabled component;
- relative source roots and output paths cannot escape through `..` or absolute paths;
- two units in one component cannot claim the same configured output;
- criticality is empty, `low`, `medium`, `high`, or `critical`;
- explicit units take precedence over inferred units covered by the same source root.

## Capability packs

Supported packs:

```text
api, cache, concurrency, configuration, container-runtime, cryptography,
data-access, database, distributed-coordination, domain, engineering,
files, jobs, messaging, migrations, rate-limit, runtime, security,
telemetry, workflow
```

The adaptive planner unions profile defaults, explicit `components[].packs`, and discovered packs. Every registered pack has a canonical planning mapping and an include, skip, or defer decision.

## Path normalization

- `workspace`, component repositories, system output, and facts paths become absolute paths relative to the config file.
- `scope`, unit source roots, and unit outputs remain normalized relative paths.
- portable component and unit IDs are enforced across Windows, Linux, and macOS.
- `config migrate` emits paths relative to the output config location when possible, avoiding machine-specific absolute paths.

## Compatibility migration

```text
version 1 services -> microservice components -> normalized version 3
version 2 components -> normalized version 3 defaults
version 3 -> normalized and validated version 3
```

Use:

```bash
wikiforge config migrate --config wikiforge.yaml --output wikiforge.v3.json
```

The command refuses to replace an existing output unless `--force` is provided. The generated JSON is reloadable by WikiForge.

## Validation

`config.Validate` enforces:

- current normalized version;
- at least one enabled component;
- portable and unique component IDs;
- valid profiles, packs, views, unit kinds, criticalities, and shard dimensions;
- unique component work directories;
- valid dependency and related-unit references;
- safe relative scopes, source roots, and output paths;
- positive catalog and evidence limits;
- required system output when system aggregation is enabled;
- valid Mermaid mode.

The published Draft 2020-12 schema is [`/schema/wikiforge-config.schema.json`](/schema/wikiforge-config.schema.json). Tests prevent the schema pack/view enums from drifting from the Go registry and load all published version 3 examples through the production parser.

## Source map

| File | Role |
|---|---|
| `/internal/config/config.go` | Model, defaults, normalization, migration, validation |
| `/internal/config/yaml.go` | Dependency-free YAML subset parser |
| `/internal/config/config_test.go` | Compatibility and configuration invariant tests |
| `/internal/config/schema_test.go` | JSON Schema registry-drift tests |
| `/schema/wikiforge-config.schema.json` | Draft 2020-12 schema |
| `/wikiforge.example.yaml` | Version 3 example |
| `/internal/discovery/discovery.go` | Evidence-bound deterministic discovery |
| `/internal/planner/planner.go` | Adaptive plan construction |

## Knowledge Gaps

The schema validates shape and enumerated values but cannot validate filesystem existence or Git work-tree status. `wikiforge doctor` performs those environmental checks, including configured documentation-unit source roots.

## Source References

- `/internal/config/config.go`
- `/internal/config/config_test.go`
- `/internal/config/schema_test.go`
- `/schema/wikiforge-config.schema.json`
- `/internal/cli/cli.go`
