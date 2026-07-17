# WikiForge

WikiForge is a general-purpose Go orchestrator that runs **OpenWiki** in controlled phases, validates generated Markdown and Mermaid diagrams, performs targeted repair, aggregates component documentation into a whole-system wiki, and exports a lightweight knowledge graph.

WikiForge intentionally does **not** parse programming languages, frameworks, IaC dialects, or configuration formats. OpenWiki remains responsible for understanding repository content. WikiForge provides repeatable orchestration, profile-specific documentation contracts, validation, resume, reporting, and cross-repository aggregation.

## Supported repository scenarios

WikiForge is component-centric. A component may be an entire repository or a scoped directory inside a monorepo.

| Scenario | Component type | Documentation profile |
|---|---|---|
| Traditional monolith | `monolith` | `application` |
| Microservice | `microservice` | `application` |
| Worker, gateway, frontend, CLI | `worker`, `gateway`, `frontend`, `cli` | `application` |
| Modular monolith | `modular-monolith` | `modular-application` |
| Shared/internal library | `library`, `shared-library`, `internal-library` | `reusable` |
| Internal framework or SDK | `framework`, `sdk` | `reusable` |
| Infrastructure as code | `iac`, `infrastructure` | `infrastructure` |
| GitOps/deployment/platform repository | `gitops`, `deployment`, `platform` | `infrastructure` |
| Shared configuration/policy/templates | `configuration`, `shared-config`, `config` | `configuration` |
| API/event/data schemas and contracts | `contract`, `contracts`, `schema`, `schemas` | `contracts` |
| Unclassified repository | `generic`, `repository` | `generic` |

A single configuration can combine all of these. Whole-system documentation treats each target according to its declared type instead of assuming everything is a service.

## Monorepo support

Use one `repository` with different relative `scope` values:

```yaml
components:
  - id: catalog-app
    type: microservice
    repository: ./repositories/platform-monorepo
    scope: apps/catalog
    enabled: true

  - id: shared-runtime
    type: framework
    repository: ./repositories/platform-monorepo
    scope: packages/runtime
    enabled: true

  - id: shared-contracts
    type: contracts
    repository: ./repositories/platform-monorepo
    scope: contracts
    enabled: true
```

Different Git repositories may run concurrently. Components sharing the same Git repository are automatically serialized to avoid competing writes to repository-local OpenWiki and agent files.

## What is implemented

- Native cross-platform Go CLI.
- YAML or JSON configuration.
- Config v3 adaptive-planning model with backward-compatible v1 `services` and v2 component migration.
- Repository-root and monorepo-scoped components.
- Explicit documentation units for domains, subdomains, bounded contexts, modules, flows, platform areas, and catalogs.
- Composable capability packs selected from profile defaults, explicit configuration, and deterministic source discovery.
- Deterministic discovery manifests and adaptive documentation plans persisted under `.wikiforge/components/<id>/`.
- Configurable domain/component/flow/catalog/platform/engineering/operations views, evidence include/exclude rules, and catalog shard policy.
- `discover`, `plan --explain`, and `config migrate` CLI workflows.
- Profile-specific phased generation for seven repository classes.
- Bounded parallelism across repositories and automatic serialization within a monorepo.
- Persistent profile-aware `openwiki/INSTRUCTIONS.md` contracts.
- Evidence, authority, unknown, contradiction, terminology, and source-reference rules.
- Profile-specific canonical documents, required sections, and Mermaid contracts.
- Profile-specific specialized catalog packs with exact table contracts.
- Merged runtime configuration, business rule/validation, database programmability/PLSQL, and file handling/format documentation.
- Separate authentication, authorization, concurrency, asynchronous-processing, traffic-flow, and request-flow documentation.
- Incremental update prompts that refresh every affected canonical page rather than only the relationship page.
- OpenWiki front matter validation, including rejection of unsupported fields.
- Relative Markdown link validation.
- Source-reference presence and scoped path resolution checks.
- Basic Mermaid validation and optional rendering with pinned Mermaid CLI.
- Targeted repair rounds using validator findings only.
- Whole-system aggregation from generated component wikis rather than all raw source repositories.
- Stable component manifest containing type, profile, repository, scope, group, tags, and declared dependencies.
- Optional authoritative system facts copied into the aggregation workspace.
- Resume/checkpoint state with v1 state migration.
- Scoped update no-op detection to avoid model calls when a component has not changed.
- JSON validation reports and Markdown run summaries.
- JSONL knowledge graph export for document links and standardized relationship tables.
- End-to-end tests covering every profile, mixed repository types, monorepo scopes, whole-system generation, and same-repository serialization.

The detailed Phase 1 architecture, invariants, CLI behavior, and verification scope are documented in [`PHASE-1-ADAPTIVE-PLANNING.md`](PHASE-1-ADAPTIVE-PLANNING.md).

## Live progress and long-running phases

WikiForge does not appear frozen while OpenWiki is working. Every generation displays a line-based progress bar with the component, phase ID, completed percentage, step number, status, and elapsed time. The percentage is based on completed deterministic WikiForge steps; it does not invent model-token progress inside a single OpenWiki call.

Example:

```text
[sentinel] [----------------------------]   0% step 1/17 A00  RUN   Bootstrap OpenWiki and quickstart | elapsed=00:00
[sentinel/A00] OpenWiki process started pid=18420 operation=init timeout=1h0m0s
[sentinel/A00][stdout] Indexing repository...
[sentinel/A00] still running | elapsed=00:15 | quiet=00:12 | timeout=1h0m0s
[sentinel] [#---------------------------]   5% step 1/17 A00  OK    Bootstrap OpenWiki and quickstart | elapsed=02:08
```

OpenWiki stdout and stderr are streamed immediately with the current component/phase label. If the child process produces no output, WikiForge prints a heartbeat every 15 seconds including elapsed and quiet time. Process attempts, failures, retry delays, validation, repair rounds, graph export, and final completion are also visible.

The bootstrap phase creates only a first-pass `quickstart.md`. Large specialized documentation sets are split into batches of at most four pages, which reduces oversized prompts and makes progress advance more frequently.

WikiForge also externalizes every complete phase prompt into a temporary `.wikiforge-prompt-*.md` file in the component working directory. Only a short bridge instruction is passed to OpenWiki through `--print`. This avoids Windows, PowerShell, `cmd.exe`, and `npx.cmd` command-line length limits. The temporary prompt is removed after the child process exits and is explicitly excluded as documentation evidence.

### Cross-platform path safety

Every configuration path is resolved to an absolute native path before execution. Repository scopes accept either `/` or `\` and are normalized for the current operating system. The OpenWiki prompt bridge uses an absolute, quote-free path with forward slashes, which is accepted by Node on Windows and remains native on macOS/Linux. Windows drive paths, UNC shares, extended-length `\\?\` paths, spaces, Unicode, symlinks, and junctions are normalized before they cross the external-tool boundary.

`wikiforge doctor` performs a prompt-transport preflight for every enabled component: it creates a temporary prompt, converts the path to the external-tool representation, reopens the file through that representation, and removes it. Component IDs are also restricted to portable path segments so reports, graph output, and whole-system snapshots cannot create invalid Windows/macOS/Linux paths.

Deterministic path and command-length errors are not retried because repeating the same invocation cannot repair them. Provider/network failures remain retryable according to `execution.maxProcessRetries`.

Press `Ctrl+C` to cancel the current OpenWiki child process. The checkpoint remains available for `wikiforge resume`.

## Runtime requirements

The distributed WikiForge binary is native Go. The default configuration invokes these external tools through `npx`:

- Git;
- Node.js 22 or newer;
- OpenWiki `0.2.0`;
- Mermaid CLI `11.12.0`.

Provider credentials required by the selected OpenWiki model must be available as environment variables. WikiForge does not store API keys in its generated configuration.

## Quick start

### 1. Extract a release

Windows:

```powershell
Expand-Archive .\wikiforge-<version>-windows-amd64.zip
cd .\wikiforge-<version>-windows-amd64
```

Linux/macOS:

```bash
unzip wikiforge-<version>-linux-amd64.zip
cd linux-amd64
```

### 2. Generate a configuration

```bash
./wikiforge init
```

Edit `wikiforge.yaml`, add enabled components, and assign the correct type. A profile is selected automatically from the type; advanced users may explicitly override `profile`.

### 3. Configure provider credentials

Example for an OpenAI-compatible gateway:

```bash
export OPENWIKI_PROVIDER=openai-compatible
export OPENAI_COMPATIBLE_API_KEY=replace-me
export OPENAI_COMPATIBLE_BASE_URL=https://gateway.example.com/v1
export OPENWIKI_MODEL_ID=cheap-code-model
```

PowerShell:

```powershell
$env:OPENWIKI_PROVIDER = "openai-compatible"
$env:OPENAI_COMPATIBLE_API_KEY = "replace-me"
$env:OPENAI_COMPATIBLE_BASE_URL = "https://gateway.example.com/v1"
$env:OPENWIKI_MODEL_ID = "cheap-code-model"
```

### 4. Validate prerequisites and component scopes

```bash
./wikiforge doctor
```

### 5. Inspect supported types and profiles

```bash
./wikiforge profiles
```

### 6. Discover capabilities and preview the adaptive plan

```bash
./wikiforge discover
./wikiforge plan --explain
```

Discovery and planning are deterministic for the same normalized configuration and source content. They write `.wikiforge/components/<component-id>/discovery.json` and `plan.json`. The existing profile renderer consumes this context while Phase 2 introduces the hierarchical physical layout.

### 7. Generate all component wikis and the whole-system wiki

```bash
./wikiforge generate
```

Generate one component:

```bash
./wikiforge generate --component shared-runtime --skip-system
```

### 8. Incremental maintenance

```bash
./wikiforge update
```

WikiForge hashes each configured component scope independently. A change elsewhere in a monorepo does not force an unchanged scoped component to call the model.

## Commands

```text
wikiforge init [--config wikiforge.yaml] [--force]
wikiforge doctor [--config wikiforge.yaml]
wikiforge profiles
wikiforge config migrate [--config wikiforge.yaml] [--output wikiforge.v3.json] [--force]
wikiforge discover [--config wikiforge.yaml] [--component ID]
wikiforge plan [--config wikiforge.yaml] [--component ID] [--skip-system] [--explain]
wikiforge generate [--config wikiforge.yaml] [--component ID] [--skip-system] [--resume]
wikiforge update [--config wikiforge.yaml] [--component ID] [--skip-system]
wikiforge resume [--config wikiforge.yaml]
wikiforge validate [--config wikiforge.yaml] [--component ID] [--system]
wikiforge graph [--config wikiforge.yaml] [--component ID] [--system]
wikiforge version
```

The legacy `--service` flag remains an alias for `--component`.

## Component configuration

```yaml
components:
  - id: commerce-core
    type: modular-monolith
    repository: ./repositories/commerce-core
    enabled: true
    includeInSystem: true
    group: commerce
    tags: [core, deployable]
    dependsOn: [shared-contracts]
    owners: [commerce-team]
    capabilities: [order-management, pricing]
    packs: [workflow, messaging, database]
```

Fields:

- `id`: stable unique component identifier;
- `type`: repository/component scenario;
- `profile`: optional explicit profile override;
- `repository`: Git repository root;
- `scope`: optional relative directory inside the repository;
- `enabled`: whether WikiForge processes the component;
- `includeInSystem`: whether its generated wiki is included in whole-system aggregation;
- `group`: optional logical grouping;
- `tags`: optional classification;
- `dependsOn`: optional declared component dependencies included in the system manifest;
- `owners`: optional ownership hints used by planning and future catalog sharding;
- `capabilities`: business capabilities that become configured domain documentation units;
- `packs`: capability packs composed with profile defaults and source-discovered packs.

`scope` must be relative and cannot escape the repository.

### Documentation units

A documentation unit is deliberately separate from a deployable component:

```yaml
documentationUnits:
  - id: order-management
    component: commerce-core
    kind: domain
    sourceRoots: [modules/order, workflows/order]
    output: domains/order-management
    owners: [commerce-team]
    capabilities: [order-management]
    criticality: high

  - id: submit-order
    component: commerce-core
    kind: flow
    sourceRoots: [workflows/order/submit-order.bpmn]
    relatedUnits: [order-management]
    output: flows/submit-order
```

Supported kinds are `domain`, `subdomain`, `bounded-context`, `component`, `module`, `flow`, `platform`, and `catalog`. Source roots and output paths are normalized relative paths and cannot escape the component scope.

### Adaptive planning

The planner combines three inputs:

1. capability packs required by the component profile;
2. explicit component `packs`;
3. capability evidence discovered from eligible source files.

It records included, skipped, and deferred decisions rather than forcing every concern into a fixed page set. The Phase 1 plan describes the future hierarchical pages while the existing renderer remains backward-compatible.

## Documentation profiles

### Application

Used by monoliths, microservices, workers, gateways, frontends, and CLIs:

```text
openwiki/
├── quickstart.md
├── architecture/overview.md
├── domain/behavior.md
├── interfaces/contracts.md
├── data/consistency.md
├── reliability/security-operations.md
├── development/change-guide.md
└── knowledge/relationships.md
```

The prompt allows stateless or non-domain applications to explicitly record that no owned persistence or domain lifecycle was found instead of inventing one.

### Modular application

Adds explicit module documentation:

```text
openwiki/
├── quickstart.md
├── architecture/overview.md
├── modules/catalog.md
├── modules/dependency-rules.md
├── domain/behavior.md
├── interfaces/contracts.md
├── data/consistency.md
├── development/change-guide.md
└── knowledge/relationships.md
```

### Reusable library or framework

Focuses on public API, integration, extension, lifecycle, thread safety, compatibility, migration, and contribution:

```text
openwiki/
├── quickstart.md
├── architecture/design.md
├── api/public-api.md
├── integration/usage-and-extension.md
├── configuration/configuration.md
├── compatibility/versioning.md
├── development/contribution-guide.md
└── knowledge/relationships.md
```

### Infrastructure/IaC/GitOps

Focuses on managed resources, topology, environments, state, drift, security, delivery, rollback, and recovery:

```text
openwiki/
├── quickstart.md
├── architecture/topology.md
├── resources/resource-inventory.md
├── environments/environment-model.md
├── delivery/change-pipeline.md
├── security/security-controls.md
├── operations/operations-and-recovery.md
└── knowledge/relationships.md
```

### Configuration

Focuses on configuration semantics, sources, precedence, validation, consumers, compatibility, promotion, rollback, and secrets references:

```text
openwiki/
├── quickstart.md
├── configuration/model.md
├── configuration/sources-and-precedence.md
├── configuration/schema-and-validation.md
├── configuration/consumers-and-compatibility.md
├── security/secrets-and-sensitive-values.md
├── development/change-guide.md
└── knowledge/relationships.md
```

### Contracts and schemas

Focuses on canonical contracts, semantics, producers/providers, consumers, compatibility, generation, distribution, and testing:

```text
openwiki/
├── quickstart.md
├── contracts/catalog.md
├── contracts/semantics.md
├── contracts/compatibility.md
├── contracts/consumers-and-producers.md
├── contracts/validation-and-testing.md
├── development/change-guide.md
└── knowledge/relationships.md
```

### Generic

A neutral fallback that does not force service or application terminology.


## Specialized documentation pack

The backward-compatible profile renderer provides deterministic first-class documentation for runtime configuration, integrations, interfaces, messaging, jobs, business behaviour, traffic, request processing, security, concurrency, asynchronous work, context propagation, databases, cryptography, and files.

The following related subjects remain intentionally merged:

| Combined page | Merged subjects |
|---|---|
| `configuration/runtime-configuration.md` | application properties, environment variables, secret references, and external configuration sources |
| `business/rules-and-validation.md` | business rules and validation rules |
| `data/database-programmability.md` | database functions, procedures, packages, triggers, stored jobs, and PL/SQL or equivalent stored code |
| `files/file-handling-and-formats.md` | file handling, transfer/storage, and file formats/schemas |

The following subjects are now separate canonical pages:

| Subject | Canonical Markdown |
|---|---|
| Network traffic | `runtime/traffic-flows.md` |
| Internal request processing | `runtime/request-flows.md` |
| Authentication | `security/authentication.md` |
| Authorization | `security/authorization.md` |
| Multithreading and concurrency | `runtime/concurrency.md` |
| Asynchronous processing | `runtime/asynchronous-processing.md` |
| Request/thread/process context | `runtime/context-propagation.md` |

Every profile receives the relevant subset. Application and modular-application profiles receive all 22 specialized pages. A page is still generated when no applicable implementation is observed; it must explicitly say `Not Observed` or `Unknown`, describe the evidence searched, and must not invent catalog entries.

Each specialized page has enforced:

- exact required section names;
- an exact catalog table header;
- a Mermaid diagram contract;
- OpenWiki front matter;
- knowledge gaps and source references;
- secret-safe evidence rules.

Whole-system aggregation adds 17 dedicated system catalogs. Traffic/request, authentication/authorization, and concurrency/asynchronous/context concerns are separate at system level as well. See [DOCUMENTATION-CATALOG.md](DOCUMENTATION-CATALOG.md) for the complete list.

## Whole-system aggregation

```text
enterprise-wiki/
├── README.md
├── facts/
├── sources/
│   ├── manifest.json
│   └── components/
│       └── <component-id>/openwiki/...
└── openwiki/
    ├── INSTRUCTIONS.md
    ├── quickstart.md
    ├── system/landscape.md
    ├── system/component-map.md
    ├── system/cross-component-flows.md
    ├── system/data-events-contracts.md
    ├── system/infrastructure-deployment.md
    ├── system/failure-security-operations.md
    ├── system/dependency-matrix.md
    ├── system/endpoint-catalog.md
    ├── system/event-catalog.md
    ├── system/job-catalog.md
    ├── system/business-flow-rules-and-data.md
    ├── system/traffic-flows.md
    ├── system/request-flows.md
    ├── system/authentication.md
    ├── system/authorization.md
    ├── system/concurrency.md
    ├── system/asynchronous-processing.md
    ├── system/context-propagation.md
    ├── system/database-structures-and-programmability.md
    ├── system/configuration-secrets-and-external-sources.md
    ├── system/cloud-service-dependencies.md
    ├── system/cryptography-and-key-management.md
    ├── system/file-handling-and-formats.md
    ├── system/onboarding-change-guide.md
    └── knowledge/relationships.md
```

The manifest preserves component type and profile, preventing libraries, infrastructure, or configuration repositories from being misrepresented as runtime services.

## Validation model

Hard failures include:

- missing profile-specific canonical document;
- missing required or non-empty section;
- missing exact specialized catalog table header or catalog data/Not Observed row;
- canonical page missing from `quickstart.md` navigation;
- missing or unsupported OpenWiki front matter fields;
- broken relative Markdown links;
- missing source-reference section;
- missing required Mermaid diagram;
- unsupported Mermaid type;
- Mermaid basic or render failure;
- insufficient profile-specific diagram coverage;
- missing standardized relationship table.

Unresolved source paths are warnings because generated documentation may reference patterns or generated locations, but warnings reduce the quality score. Failed validation triggers a bounded targeted-repair loop.

## Mermaid modes

```yaml
mermaid:
  mode: render
```

- `render`: every Mermaid block must produce a non-empty SVG through the pinned CLI;
- `basic`: offline type, delimiter, line-length, size, and contract checks;
- `off`: no renderer invocation; profile page contracts can still require Mermaid unless `documentation.requireMermaid` is disabled and prompts are customized.

## State, reports, and graph output

```text
.wikiforge/
├── state.json
├── components/
│   └── <component-id>/
│       ├── discovery.json
│       └── plan.json
├── validation/
│   ├── <component-id>.json
│   └── system.json
├── reports/
│   ├── latest.txt
│   └── <run-id>/
│       ├── report.json
│       └── summary.md
└── graph/
    ├── <component-id>/
    │   ├── nodes.jsonl
    │   └── edges.jsonl
    └── system/
        ├── nodes.jsonl
        └── edges.jsonl
```

## Backward compatibility

Version 1 configurations using:

```yaml
services:
  - id: order
    path: ./repositories/order
    enabled: true
```

are normalized to `microservice` components using the `application` profile. Version 2 component configurations are also accepted and normalized in memory to version 3. New configurations should use `version: 3`, `components`, and optional `documentationUnits`. Use `wikiforge config migrate` to emit a normalized version 3 JSON file.

## Security

- Credentials are inherited from environment variables.
- Prompts prohibit reading or exposing secret values, private keys, tokens, and production personal data.
- Infrastructure and configuration profiles document references and protection mechanisms, never secret content.
- OpenWiki has repository access; use least-privilege filesystem and provider credentials.
- Generated documentation is evidence-derived and still requires human review before being treated as authoritative policy.

## Development and verification

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/wikiforge
```

The project uses only the Go standard library.
