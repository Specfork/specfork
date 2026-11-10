# Catalog

**Status:** Draft · **Prefix:** CAT

## Purpose

Grow the catalog by 100 specs, each modeled on what a well-known closed-source app does, so people who look for an open version of an app they know find a recipe.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| CAT-1 | The planned list has exactly 100 entries in `catalog/list.md`; each has a generic name, a category, and a one-line kind of product that inspired it. | TestCatalogListShape: given the file, when parsed, then 100 entries with all three fields and ≥ 8 categories |
| CAT-2 | Entries are chosen by how often people search for an open alternative; each records its demand evidence (source and date), and the list is ordered by it within a category. | TestCatalogListEvidence: given an entry without evidence, when checked, then it fails |
| CAT-3 | No spec is written until the owner has approved the list; the approval is the merged pull request that adds `catalog/list.md`. | manual: first batch PR is opened after the list PR is merged |
| CAT-4 | Specs are written and published in batches of ten, one pull request per batch, in list order; a batch pull request touches only its ten specs. | TestCatalogBatch: given a batch PR, when checked, then it has 10 distinct list entries and no other spec |
| CAT-5 | Each spec follows the Specfork format and `specfork lint` reports no errors for it. | `specfork lint` on each spec exits 0 |
| CAT-6 | Each spec's README has one line `Inspired by apps like …` naming the kind of product; it says the spec is not affiliated with or endorsed by any owner ([EXT-10](extract.md)). | TestCatalogInspiredLine: given a README without the line or the disclaimer, when linted, then it fails |
| CAT-7 | Spec names, `summary` and `tags` are generic: no product name or trademark (`kanban-board`, not a brand); a product may be named only in the inspired-by line. | TestCatalogGenericNames: given name `@specfork/trello-clone`, when checked against the brand list, then it fails |
| CAT-8 | Specs are written in Specfork's own words from what the product publicly does; they contain no copied text and no screenshots, logos or other assets. | manual: review of each batch; no image files in a spec repo |
| CAT-9 | Specs are stack-independent, and generic parts (auth, teams, webhooks, …) are `require`d from base specs ([EXT-4](extract.md)). | TestCatalogNoStack: given a spec naming a framework or language, when checked, then it fails |
| CAT-10 | A batch is published only after all ten specs pass CAT-5..9 and the owner has approved it; each published spec is a repo `specfork/<name>` with topic `specfork`, tag `v0.1.0`, claimed as `@specfork/<name>`, and found by `specfork search` and `resolve`. A failing batch stays unpublished. | TestCatalogPublish: given a published name, when `search` and `resolve` run, then both return it at v0.1.0 |

## Out of scope

- Reverse-engineering closed-source apps or using non-public information.
- Rebuilding each spec ([EXT-7](extract.md)); proof can follow later.

## Decisions

- The list is its own approved artifact so the owner reviews 100 names once, not inside ten batches.
- Quality over count: the 100 is reached only through batches that pass; a short batch waits rather than ships.
- Naming a product is allowed only in the inspired-by line, because it is what searchers type, and it states a fact rather than using the brand as a name.
