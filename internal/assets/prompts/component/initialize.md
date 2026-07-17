{{BASE}}

OBJECTIVE
Bootstrap OpenWiki for this component and create a focused first-pass entry page. Modify documentation files only; never modify product source, infrastructure definitions, contracts, or configuration inputs.

COMPONENT CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}} (`{{PROFILE_ID}}`)
- Profile purpose: {{PROFILE_DESCRIPTION}}
- Repository: {{REPOSITORY}}
- Scoped working directory: {{SCOPE}}

CANONICAL DOCUMENTATION ROADMAP
The complete run will eventually create or update these pages:
{{CANONICAL_FILES}}

THIS PHASE IS BOOTSTRAP ONLY
- Initialize OpenWiki state and repository understanding.
- Create or update `openwiki/quickstart.md` as a concise evidence-backed first pass.
- Add valid OpenWiki front matter and an initial Mermaid context diagram.
- Record early knowledge gaps and source references.
- Do not attempt to fully generate every canonical page during this phase; later WikiForge phases own those pages in bounded batches.
- Do not create thin placeholders for all roadmap files.

PROFILE GUIDANCE
{{GUIDANCE}}

STOP CONDITION
OpenWiki is initialized and `openwiki/quickstart.md` provides a useful first-pass orientation grounded in repository evidence. Return control to WikiForge so the remaining bounded phases can continue.
