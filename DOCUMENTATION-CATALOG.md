# WikiForge Documentation Catalog

WikiForge generates adaptive documentation from one validated semantic discovery graph per component. The graph is evidence-backed; component packs and documentation units are explicit opt-ins, while profile names and capability strings cannot create semantic pages. The tables below describe canonical ownership.

## View hierarchy

| View | Canonical location | Ownership |
|---|---|---|
| System | `quickstart.md`, `system/` | Whole-system topology, capability map, component landscape, runtime shape, and cross-component relationships. |
| Domain | `domains/<domain>/` | Business capability, concepts, rules, state, interfaces, events, component mapping, and participating flows. |
| Component | `components/<component>/` | Runtime boundary, architecture, contracts, owned data, consistency, configuration, and operations. |
| Flow | `flows/<flow>.md` | Trigger, actor, preconditions, sync/async steps, state changes, transactions, events, failure, compensation, and telemetry. |
| Catalog | `catalogs/<catalog>/` | Typed lookup data, stable IDs, direction, ownership, evidence, and optional shards. |
| Platform | `platform/` | Shared messaging, security/identity, telemetry, container, and deployment mechanisms. |
| Engineering | `engineering/` | Reusable engineering, compatibility, testing, and implementation standards. |
| Operations | `operations/` | Runtime operation, recovery, ownership, failure handling, and safe change procedures. |

Navigation is progressive: the root links view indexes, view indexes link unit or collection indexes, and collections link leaf pages or shards. The root does not directly enumerate every leaf page.

## Component pages

| Canonical page | Purpose |
|---|---|
| `components/index.md` | Component-view navigation. |
| `components/<component>/index.md` | Component identity, ownership, responsibilities, and documentation map. |
| `components/<component>/architecture.md` | Boundaries, major parts, dependency direction, entry points, runtime flow, and failure boundaries. |
| `components/<component>/contracts.md` | Inbound/outbound contracts, validation, authorization, idempotency, retry, and compatibility. |
| `components/<component>/data-and-consistency.md` | Data ownership, persistence, transactions, consistency, concurrency, migration, and recovery. |
| `components/<component>/runtime-and-operations.md` | Runtime shape, configuration, operational signals, failure modes, timeouts, retries, and recovery. |

## Domain and flow pages

Each domain unit receives an index and the applicable domain pages:

- `index.md`
- `concepts-and-data.md`
- `rules-and-invariants.md`
- `workflows-and-state.md`
- `interfaces-and-events.md`
- `component-mapping.md`

Each flow unit receives one canonical `flows/<flow>.md` page plus the shared `flows/index.md`.

## Catalog collections

| Collection | Pack | Canonical content |
|---|---|---|
| `interfaces` | `api`, `interface` | Inbound/outbound HTTP, GraphQL, RPC, gRPC, WebSocket, SSE, SFTP, and custom interfaces. |
| `service-interactions` | `api`, `integration` | Runtime calls, direction, protocols, contracts, identity, resilience, and failure impact. |
| `events` | `messaging` | Brokered events, internal events, queues, producers, consumers, delivery, ordering, retry, and replay. |
| `workflows` | `workflow`, `bpmn` | Process definitions, BPMN/workflow sources, state transitions, tasks, correlation, timers, retries, and compensation. |
| `jobs` | `jobs`, `scheduler` | Cron, scheduled, polling, batch, worker, delayed, and maintenance jobs. |
| `files` | `file-processing` | File transfers, uploads/downloads, object storage, formats, streaming, validation, retention, and cleanup. |
| `data` | `data`, `persistence` | Data stores, schemas, ownership, persistence boundaries, transactions, and consistency. |
| `database-objects` | `database-programmability` | Functions, procedures, packages, triggers, stored jobs, rights, and programmable database behavior. |
| `migrations-and-seeds` | `migration`, `migration-and-seeding` | Schema/data migrations, backfills, seeds, fixtures, rollback, and online compatibility. |
| `repositories` | `repository`, `data-access` | Domain model, repository interface, implementation, mapper, query, store, and transaction mapping. |
| `caches` | `cache` | Cache scope, keys, values, TTL, invalidation, consistency, stampede protection, ownership, and failure. |
| `rate-limits` | `rate-limit` | Request, concurrency, consumer, outbound quota, algorithm, state scope, and fail-open/closed behavior. |
| `distributed-coordination` | `distributed-coordination` | Locks, leases, fencing, leader election, semaphores, coordination stores, and split-brain risks. |
| `permissions` | `security`, `identity`, `authentication`, `authorization`, `acl` | Authentication and authorization mechanisms, identities, resources, operations, conditions, enforcement, deny behavior, and audit. |
| `cryptography` | `cryptography` | Algorithms, protocols, certificates, key sources, ownership, rotation, and risk without secret values. |
| `concurrency` | `concurrency`, `context-propagation` | Threading, pools, shared state, synchronization, ordering, shutdown, context, deadlines, and correlation. |
| `configuration` | `configuration` | Configuration sources, precedence, consumers, reload, rollout, secret references, and redaction. |
| `telemetry` | `telemetry` | Logs, metrics, traces, instrumentation, propagation, exporters, backends, dashboards, and alerts. |
| `deployment` | `deployment`, `container-runtime`, `container-and-deployment` | Docker/Compose, deployment resources, services, ingress, autoscaling, probes, configuration, and policy references. |

A collection is an index page plus one collection page or deterministic shards such as `catalogs/interfaces/<domain>.md` or `catalogs/interfaces/<owner>.md`. There is no row or byte maximum. Sharding occurs only at accepted semantic domain, owner, or explicit-unit boundaries. Each catalog row requires a stable ID and evidence.

## Platform, engineering, and operations

These views are conditional and contain shared mechanisms rather than duplicated domain truth:

- `platform/messaging.md`
- `platform/security-and-identity.md`
- `platform/telemetry.md`
- `platform/containerization.md`
- `engineering/index.md`
- `operations/index.md`

## Whole-system pages

The system plan includes:

- `quickstart.md`
- `system/index.md`
- `system/overview.md`
- `system/capability-map.md`
- `system/component-landscape.md`
- `system/runtime-topology.md`
- `system/domain-map.md` when domain units exist

System evidence is assembled from immutable component documentation snapshots under `sources/components/<component>/<docs-hash>/openwiki`. System pages own cross-component synthesis; component pages remain the canonical owner of implementation detail.

## Evidence and graph artifacts

Generated operational artifacts are not documentation pages:

- `.wikiforge/components/<id>/inventory.json`
- `.wikiforge/components/<id>/semantic-discovery.json`
- `.wikiforge/components/<id>/semantic-identities.json`
- `.wikiforge/components/<id>/discovery.json`
- `.wikiforge/components/<id>/plan.json`
- `.wikiforge/components/<id>/evidence-index.json`
- `.wikiforge/components/<id>/impact-index.json`
- `.wikiforge/components/<id>/coverage.json`
- `.wikiforge/graph/<id>/nodes.jsonl`
- `.wikiforge/graph/<id>/edges.jsonl`
- `.wikiforge/validation/<id>.json`
- `.wikiforge/state.json`

The discovery validator and planner implementation are the executable source of truth for evidence promotion, applicability, paths, packs, and shard decisions. This catalog documents the intended contract and must remain aligned with them.
