{{BASE}}

OBJECTIVE
Repair the existing wiki using only the validation findings below.

TARGET CONTEXT
- Component type: {{COMPONENT_TYPE}}
- Documentation profile: {{PROFILE_NAME}}
- Scope: {{SCOPE}}

STRICT SCOPE
- Preserve all unrelated accurate content.
- Modify only files implicated by findings.
- Do not create new claims merely to satisfy a validator.
- When evidence is insufficient, qualify the claim as Unknown and document the missing authoritative source.
- For Mermaid failures, change only the affected diagram unless surrounding prose is inconsistent.
- Keep the selected profile semantics; do not force application/service terminology onto libraries, infrastructure, configuration, or contracts.
- Never expose secret values.

VALIDATION FINDINGS
{{FINDINGS}}

STOP CONDITION
Every listed finding is either corrected or explicitly converted into a precise, evidence-grounded knowledge gap without weakening unrelated documentation.
