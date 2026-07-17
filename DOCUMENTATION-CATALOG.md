# WikiForge 1.2.3 Documentation Catalog

This document lists the specialized Markdown contracts enforced by WikiForge 1.2.3. Core profile documents remain in addition to these pages.

## Component specialized pages

| Markdown | Description | Design decision |
|---|---|---|
| `configuration/runtime-configuration.md` | Application properties, environment variables, secret references, external configuration sources, precedence, validation, reload/restart, consumers, and redaction. | Merged configuration inventory. |
| `integrations/service-to-service.md` | Internal component calls, protocols, contracts, identities, timeouts, retries, idempotency, failure impact, and correlation. | Dedicated internal communication catalog. |
| `integrations/external-services.md` | External parties, vendors, SaaS, partners, exchanged data, authentication, limits, outages, sandbox, and reconciliation. | Dedicated external-party catalog. |
| `integrations/cloud-services.md` | Cloud providers and managed services, accounts/regions, identity, data, quotas, cost, availability, backup, and recovery. | Dedicated cloud dependency catalog. |
| `integrations/dependency-matrix.md` | Runtime, build, library, configuration, contract, infrastructure, cloud, optional, and critical dependencies. | Component-level dependency matrix. |
| `interfaces/endpoint-catalog.md` | HTTP, REST, SOAP, GraphQL, RPC, gRPC, SFTP, FTP, TCP, UDP, WebSocket, SSE, file-drop, and custom endpoints. | Unified protocol endpoint catalog. |
| `messaging/event-catalog.md` | Kafka, RabbitMQ, NATS, cloud messaging, internal events, in-memory queues, producers, consumers, ordering, retry, DLQ, and replay. | Unified external and internal event catalog. |
| `processing/job-catalog.md` | Cron, scheduled, polling, timer, worker, batch, delayed, queue, and maintenance jobs. | Unified job and schedule catalog. |
| `business/business-flows.md` | Actors, preconditions, primary/alternate flows, state changes, calls, events, transactions, compensation, and change risks. | Dedicated business-flow catalog. |
| `business/rules-and-validation.md` | Business decisions, invariants, transport/schema/domain/authorization/external/database validation, ordering, errors, and test evidence. | Business and validation rules remain merged. |
| `business/business-data.md` | Business concepts, identity, ownership, lifecycle, reference data, sensitivity, derived/history data, and physical mapping. | Logical business-data catalog. |
| `runtime/traffic-flows.md` | DNS, edge, network hops, proxies, gateways, load balancing, protocol/TLS transitions, trust boundaries, timeouts, and failure paths. | Traffic is separate from internal request processing. |
| `runtime/request-flows.md` | Dispatch, middleware, authentication, authorization, validation, application/domain logic, persistence, events, dependencies, context, and response mapping. | Request processing is separate from network traffic. |
| `security/authentication.md` | Identity sources, protocols, credentials, tokens, certificates, validation, session lifecycle, workload identity, propagation, revocation, and rotation. | Authentication is first-class. |
| `security/authorization.md` | RBAC, ABAC, ACL, ownership, claims, policies, protected operations, decision inputs, enforcement, propagation, deny, and audit behaviour. | Authorization is first-class. |
| `runtime/concurrency.md` | Threads, executors, pools, schedulers, event loops, shared state, synchronization, ordering, race/deadlock/starvation risks, blocking, and shutdown. | Concurrency is separate from async workflow semantics. |
| `runtime/asynchronous-processing.md` | Futures, promises, callbacks, coroutines, reactive pipelines, local queues, background/detached work, ownership, durability, backpressure, retry, cancellation, and recovery. | Async processing is first-class. |
| `runtime/context-propagation.md` | Request, thread, trace, logging, security, tenant, transaction, locale, deadline, and business context propagation and cleanup. | Dedicated context page. |
| `data/database-structure.md` | Databases, schemas, tables, collections, fields, keys, constraints, indexes, sequences, partitions, views, and links. | Physical database catalog. |
| `data/database-programmability.md` | Functions, procedures, packages, triggers, rules, jobs, cursors, dynamic SQL, transactions, execution rights, and locking. | Database programmability and PL/SQL remain merged. |
| `security/cryptography.md` | Encryption, TLS, hashing, MACs, password protection, signatures, tokens, certificates, keys, rotation, randomness, and risky patterns. | Dedicated cryptography catalog. |
| `files/file-handling-and-formats.md` | Upload/download, SFTP/FTP, file-drop, volumes, object storage, temporary files, streaming, formats, schemas, validation, retention, and security. | File handling and formats remain merged. |

## Profile coverage

| Profile | Core pages | Specialized pages | Total canonical pages |
|---|---:|---:|---:|
| `application` | 8 | 22 | 30 |
| `modular-application` | 9 | 22 | 31 |
| `reusable` | 8 | 19 | 27 |
| `infrastructure` | 8 | 15 | 23 |
| `configuration` | 8 | 11 | 19 |
| `contracts` | 8 | 10 | 18 |
| `generic` | 8 | 22 | 30 |

## Whole-system specialized pages

| Markdown | Description |
|---|---|
| `system/dependency-matrix.md` | Directional runtime, library, configuration, contract, infrastructure, cloud, and external dependency matrix. |
| `system/endpoint-catalog.md` | Landscape-wide endpoint catalog across HTTP, RPC, gRPC, streaming, SFTP/FTP, file, and custom protocols. |
| `system/event-catalog.md` | Brokered, cloud, internal, and in-memory event topology with delivery, ordering, retry, DLQ, and replay. |
| `system/job-catalog.md` | Cron, scheduled, worker, polling, batch, maintenance, and infrastructure jobs. |
| `system/business-flow-rules-and-data.md` | Cross-component business flows, rules, validation boundaries, state transitions, and business-data ownership. |
| `system/traffic-flows.md` | Entry points, edge/network hops, protocol/TLS transitions, routing, load balancing, trust boundaries, and network failure paths. |
| `system/request-flows.md` | Cross-component request dispatch, security, validation, domain processing, data access, events, dependencies, context, and errors. |
| `system/authentication.md` | Human, service, workload, machine, partner, and cloud authentication, identity sources, credential lifecycle, trust, and propagation. |
| `system/authorization.md` | Authorization models, permissions, policies, ownership, enforcement, propagation, administrative access, deny, and audit behaviour. |
| `system/concurrency.md` | Threading models, pools, event loops, shared state, synchronization, ordering, blocking, shutdown, and systemic concurrency risks. |
| `system/asynchronous-processing.md` | Async flows, local/broker queues, reactive/callback execution, ownership, durability, backpressure, retry, cancellation, and recovery. |
| `system/context-propagation.md` | Trace, logging, security, tenant, transaction, deadline, locale, and business context across thread/process/network boundaries. |
| `system/database-structures-and-programmability.md` | Database ownership, physical structures, stored code, transactions, rights, locking, and migration coupling. |
| `system/configuration-secrets-and-external-sources.md` | Properties, environment variables, secret references, external sources, precedence, consumers, reload, and rollout. |
| `system/cloud-service-dependencies.md` | Cloud provider/service dependencies, accounts/regions, identities, data, quotas, cost, availability, and recovery. |
| `system/cryptography-and-key-management.md` | Cryptographic mechanisms, certificates, key stores/KMS/HSM, ownership, rotation, and risks. |
| `system/file-handling-and-formats.md` | File exchanges, transfer/storage, formats, validation, streaming, atomicity, retention, cleanup, and security. |

The whole-system wiki contains 26 canonical pages: 9 core pages plus 17 specialized aggregate pages.

## Validation behaviour

Every specialized page assigned to a profile is mandatory. When no applicable evidence exists, the page must still preserve its contract and state `Not Observed` or `Unknown`; it must not invent entries. Validation enforces required sections, exact table headers, Mermaid contracts, front matter, navigation links, source references, and knowledge gaps.
