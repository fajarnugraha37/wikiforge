# WikiForge Configuration

WikiForge accepts only configuration `version: 4`. Hybrid discovery builds a
deterministic evidence inventory, asks the configured OpenWiki model for
bounded semantic classifications, validates evidence references, resolves
stable identities, and then plans documentation. The planner has no legacy
directory or capability fallback.

## Minimal Configuration

```yaml
version: 4
workspace: .

openwiki:
  command: npx
  args: [--yes, openwiki@0.2.0, code]
  timeoutMinutes: 60
  modelId: ""

documentation:
  discovery:
    mode: hybrid
    required: true
    reuseCache: true
    onConflict:
      domainIdentity: fail
      sourceOwnership: fail
      moduleRole: retain
      ownership: retain
      criticality: retain
      relationships: retain

components:
  - id: order-service
    type: microservice
    repository: ./repositories/order-service
    enabled: true
```

Provider credentials belong in the process environment expected by OpenWiki,
never in this file or generated artifacts.

## Discovery

`documentation.discovery.mode` is `hybrid`, `explicit`, or `disabled`. `hybrid` is the
default and required mode for semantic inference. `explicit` performs no model
call and uses only configured `documentationUnits` and explicit component
packs. `disabled` performs no semantic inference and is allowed only when
discovery is not required for the enabled component profiles. `required: true` fails generation when a semantic result cannot be
validated. `reuseCache` reuses a graph only when source revision, evidence
fingerprint, normalized configuration, prompt version, command, and model
identity match.

Conflict handling is dimension-aware. `domainIdentity` and `sourceOwnership`
default to `fail` because they can change authoritative page ownership.
`moduleRole`, `ownership`, `criticality`, and `relationships` default to
`retain`; those findings remain diagnostics and do not block unrelated pages.

The model references evidence IDs created before inference. It cannot invent
paths or line ranges. Accepted findings are evidence-backed; absence is
`not-observed` or `uncertain`, not `not-applicable`. Stable IDs are assigned by
the deterministic normalizer and persisted in `semantic-identities.json`.

## Components And Units

Every enabled component requires a portable `id`, `type`, `repository`, and
`enabled: true`. `scope` selects a relative monorepo directory. `packs` and
`documentationUnits` are explicit opt-ins. `capabilities` are metadata hints;
they never become domains automatically.

```yaml
components:
  - id: commerce-core
    type: modular-monolith
    repository: ./repositories/commerce-core
    enabled: true
    packs: [telemetry]

documentationUnits:
  - id: order-management
    component: commerce-core
    kind: domain
    sourceRoots: [modules/order]
    output: domains/order-management
```

## Evidence And Pages

`documentation.evidence.include` and `exclude` use repository-relative glob
boundaries on top of Git-aware tracked and standard untracked enumeration.
Empty lists mean no user include/exclude restriction; built-in `.git`, vendor,
generated, binary, unsafe symlink, and configured file-size safeguards still
apply. Evidence is used for discovery and documentation validation.

Catalogs may set `shardBy: [domain, owner]`, but there is no maximum page row or
byte setting. Page content is never truncated or rejected for size. A new page
is created only at an accepted semantic boundary or explicit unit boundary.

## Commands

```text
wikiforge doctor --config wikiforge.yaml
wikiforge discover --config wikiforge.yaml [--component ID]
wikiforge plan --explain --config wikiforge.yaml [--component ID]
wikiforge generate --config wikiforge.yaml [--component ID]
```

`plan --explain` consumes persisted validated discovery and never silently
invokes a model. Refresh it with `discover` when the artifact is missing or
stale.

## Artifacts

Each component stores `inventory.json`, `semantic-discovery.json`,
`semantic-identities.json`, `discovery.json`, and `plan.json` under
`.wikiforge/components/<id>/`. Prompts, raw model output, credentials, and
secret values are not persisted as normal artifacts.
