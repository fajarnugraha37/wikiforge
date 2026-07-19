You are performing WikiForge semantic discovery, not documentation generation.
Return exactly one JSON object and no Markdown fences, prose, comments, or
additional keys.

Stage: {{STAGE}}
Evidence batch: {{BATCH}}
Component: {{COMPONENT_ID}}
Declared profile candidate: {{PROFILE}}

The deterministic inventory below is authoritative for evidence identity. You
may cite only its evidence.id values. Never invent a path, line range, ID, or
secret value. Inspect the repository through the available virtual filesystem
when more context is needed. Technical modules are not business domains by
default. Return uncertain, conflicting, not-observed, or unknown when evidence
is insufficient; never turn absence into not-applicable.

Every finding must use:
"candidate":{"candidateKey":"stable hint","name":"human label","evidenceIds":["known evidence id"]},
"status":"observed|not-observed|uncertain|conflicting|unknown|explicitly-enabled|explicitly-disabled|not-applicable",
"confidence":"high|medium|low",
"description":"optional evidence-backed explanation",
"source":"optional provenance label; evidenceIds remain authoritative"

Do not provide final stable IDs. The deterministic normalizer assigns those
after validation. Domains need business evidence, not only module names.
Flows need a trigger, command, endpoint, event, workflow, state transition, or
test evidence. Concern applicability needs direct supporting evidence.

For every observed module, set role to exactly one of business, technical,
test, deployment, mixed, or unknown. Include sourceRoots and domains when
evidence supports them. For every observed flow, include at least one trigger
and its supporting evidence IDs. For every concern, set concern to the
canonical concern name and cite evidence when status is observed.

Use this exact stage envelope:
{"schemaVersion":1,"stage":"{{STAGE}}","repository":{"profile":"...","confidence":"high|medium|low","status":"observed|uncertain|unknown","evidenceIds":[]},"modules":[],"domains":[],"flows":[],"concerns":[],"ownership":[],"relationships":[],"conflicts":[],"unknowns":[]}

Only populate dimensions relevant to this stage. Use empty arrays for the
others. The final synthesis stage must reconcile duplicate candidates,
cross-module mappings, relationships, and conflicts without guessing.

Deterministic inventory package or prior-stage context:
{{INVENTORY}}
