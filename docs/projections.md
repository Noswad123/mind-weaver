# Domains and projections

MindWeaver uses Markdown as the source of truth and SQLite as a derived
projection/cache.

```text
Markdown notes
  -> parsed and validated by mw
  -> indexed into SQLite
  -> exposed as JSON query/projection commands
  -> rendered by CLIs, TUIs, PWAs, or native apps
```

This keeps the knowledge model in notes while still making structured views fast
and app-friendly.

## Domains

Domains are labels declared in note frontmatter:

```yaml
---
id: "Recipe: Margarita [foodie:1]"
domains: [recipe]
tags: [recipe]
---
```

Domains can describe note shape, topic, workflow, or language. They are stored in
SQLite through the `note_domains` table during ingestion.

### Structural domains

Structural domains describe the shape of a note:

- `recipe`
- `glossary`
- `vocabulary`
- `task-index`
- `programming-concept`
- `checklist`
- `protocol`

### Scope/topic domains

Scope domains describe context or subject area:

- `aviation`
- `biology`
- `japanese`
- `portuguese`
- `spanish`
- `go`
- `rust`
- `swift`
- `mrna`

Not every structural domain needs a scope domain. For current purposes,
`recipe` means culinary recipe, so recipe notes use:

```yaml
domains: [recipe]
```

not:

```yaml
domains: [recipe, cooking]
```

Use scope domains when they add real distinction, such as:

```yaml
domains: [glossary, aviation]
domains: [vocabulary, japanese]
domains: [protocol, biology, mrna]
domains: [checklist, aviation]
```

## Projections

A projection is a structured view extracted from Markdown and stored in SQLite.
The projection should be owned by the Go engine, not the UI.

For example, the SwiftUI app should not decide what a todo or recipe is. It
should ask `mw` for a JSON projection:

```bash
mw query todos
mw query projection recipe
```

This boundary keeps clients simple and consistent:

```text
Go mw:
- parse markdown
- validate domains
- extract projections
- write SQLite
- expose JSON commands

Swift/PWA/UI:
- render projection JSON
- trigger engine commands
- avoid duplicating parser/validation logic
```

## Current projection commands

```bash
mw query domains
mw query todos
mw query recipes
mw query projection recipe
mw query ingredients
mw query ingredients --mentions
mw query ingredients --unresolved
```

`mw query projection recipe` returns a JSON array. Count items with:

```bash
mw query projection recipe | jq 'length'
```

## Scope filtering

Projection queries support a multi-domain `--scope` flag:

```bash
mw query projection recipe --scope some-domain
mw query projection recipe --scope domain-a,domain-b
mw query projection recipe --scope domain-a --scope domain-b
```

Scope means: return projections whose source note contains **all** requested
scope domains.

Example future uses:

```bash
mw query projection vocabulary --scope japanese
mw query projection glossary --scope aviation
mw query projection protocol --scope biology,mrna
```

Only the `recipe` projection is implemented today. Scope support exists so that
future projections can use composable domain intersections without creating
domain-specific command explosions.

## Recipe projection

Recipe notes use:

```yaml
domains: [recipe]
```

Expected shape:

```markdown
## Ingredients

- 1.5 oz Tequila
- 0.5 oz Triple Sec

## Instructions

1. Shake and strain into a glass.
```

On ingest, MindWeaver extracts:

- recipe metadata
- ingredient mentions
- best-effort quantity/unit/name parts
- instructions
- canonical ingredient rows

The recipe projection tables are intentionally a projection, not the source of
truth. Markdown remains authoritative.

### Ingredient curation

Ingredient extraction is best-effort. Ingredient canonicalization should be a
workflow, not a blocker.

Useful commands:

```bash
mw query ingredients
mw query ingredients --mentions
mw query ingredients --unresolved
```

The long-term model is:

```text
recipe_ingredient_mentions = raw extracted facts from notes
ingredients                = canonical ingredient registry
ingredient_aliases         = mappings from raw/alternate names to canonical ingredients
```

## Projection maturity ladder

Do not create custom tables for every domain immediately. Let complexity earn
its way in.

1. **Plain notes**: indexed by notes/tags/links/domains only.
2. **Validated notes**: domain YAML validates shape.
3. **Generic projection**: JSON payload/fields for browsing and filtering.
4. **Operational projection**: custom tables when state, writeback, sync,
   scheduling, or non-trivial query behavior is required.

Examples:

- `todos` are operational: they need completion state, dashboard writeback, and archive behavior.
- `recipe` is partly structured: it benefits from recipe and ingredient projection tables.
- `vocabulary + japanese` may eventually become operational if it grows spaced repetition.
- `protocol + biology + mrna` may begin as generic projection and later gain protocol-specific workflow.
