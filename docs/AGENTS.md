# Documentation Agent Guide

The root [AGENTS.md](../AGENTS.md) invariants apply. These rules cover
`docs/`, the root Markdown files, the translated READMEs, the agent guides,
and the skills in `.claude/skills/`.

## Where Each Topic Belongs

[Documentation rules](README.md#documentation-rules) is the single list of
where each topic belongs. In short: user-visible behavior in `docs/user/` and
`README.md`, system boundaries in `docs/architecture/`, runtime instructions
in `docs/operations/`, local developer workflow in `docs/development/`,
durable design reasons in `docs/decisions/`, and history only in
`docs/history/`.

## Writing Rules

- Docs describe the current system and current decisions in present tense. Do
  not narrate change with words such as "previously", "no longer", "now", "as
  before", "was removed", or "retired".
- Do not add migration stories, before/after comparisons, plans, roadmaps,
  TODOs, dated measurement logs, or PR/issue/commit references to reference
  pages.
- When behavior changes, rewrite the affected sections so each page states
  the current behavior directly, and remove every statement the change
  contradicts.
- Keep one authoritative statement for each rule and link to it from
  summaries.
- English is canonical. Translated READMEs summarize the English README and
  link back to the English documentation.
- Use reserved domains, `192.0.2.0/24` addresses, `/path/to/music`, and
  synthetic identifiers in public examples.
- Keep `.env.example`, configuration docs, the Makefile, CI, and release docs
  consistent.

## Agent Guides and Skills

- The root `AGENTS.md` holds the invariants every change obeys, validation,
  and commit rules, within about 5K characters. An area guide holds that
  area's rules and a "Which page to open" table, within about 4K characters.
- A multi-step procedure done occasionally belongs in a skill under
  `.claude/skills/`; its body links to the doc that owns each rule.
- State a rule once and link to its owning doc page instead of restating it.
- A new area guide gets a row in the root routing table and an entry in
  `agentGuideFiles` in `scripts/check-doc-links.mjs`.

## Checks

`make docs-check` validates link targets and heading anchors. Keep heading
text stable, since links use its anchors.
