<!-- WIKIFORGE:START -->
# WikiForge Whole-System Documentation Contract

Build the whole-system wiki from component documentation snapshots under `sources/components/` and optional human-authored facts under `facts/`.

Components may be deployable applications, modular monoliths, libraries, frameworks, contract repositories, infrastructure-as-code, GitOps repositories, platforms, or shared configuration. Do not describe every component as a service.

Component wikis are derived evidence. Files under `facts/` are authoritative only when they explicitly declare scope and ownership. Never silently resolve contradictions between components.

The wiki must help humans and LLM agents trace capabilities, deployables, modules, reusable foundations, contracts, data, events, infrastructure, configuration, failures, tests, and safe change paths across the landscape.

Documentation language: {{LANGUAGE}}
System identifier: {{TARGET_ID}}
<!-- WIKIFORGE:END -->
