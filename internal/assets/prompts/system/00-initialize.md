{{BASE}}

OBJECTIVE
Bootstrap a whole-system wiki from component documentation snapshots under `sources/components/` and optional authoritative facts under `facts/`. Modify documentation under `openwiki/` only.

CANONICAL DOCUMENTATION ROADMAP
The complete run will eventually create or update these pages:
{{SYSTEM_CANONICAL_FILES}}

THIS PHASE IS BOOTSTRAP ONLY
- Initialize OpenWiki state for the aggregation workspace.
- Create or update `openwiki/quickstart.md` as a concise first-pass system orientation.
- Treat every source according to its declared component type and profile.
- Never call a library, framework, IaC repository, contract repository, or configuration repository a service unless the source explicitly uses that term.
- Add valid OpenWiki front matter, an initial context diagram, knowledge gaps, and source references.
- Do not attempt to fully synthesize every system page during this phase; later WikiForge phases own those pages in bounded batches.

STOP CONDITION
OpenWiki is initialized and `openwiki/quickstart.md` gives an evidence-backed first-pass system orientation. Return control to WikiForge.
