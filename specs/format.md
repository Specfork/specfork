# Format

**Status:** Live · **Prefix:** FMT

## Purpose

What makes a GitHub repo a Specfork spec, and how it describes itself.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| FMT-1 | A spec repo has the GitHub topic `specfork` and a root `README.md` whose YAML front matter has `specfork: 1`. | TestFrontMatterSchema, TestUnclaimed |
| FMT-2 | Front matter fields: `name` (required, `@scope/name`, `[a-z0-9-]`, ≤ 64 chars), `summary` (required, ≤ 200 chars), `license` (required, SPDX id), `requires` (map name → range), `base` (`{spec: name@version, relation: fork\|extends}`), `derived_from` (`{repo, commit, url, license}`), `overrides` (map global ID → `removed` or replacement text), `tags` (≤ 5, each `[a-z0-9-]`, ≤ 24 chars; e.g. `saas`, `personal`, `desktop`, `base`). Unknown fields → lint warning. | TestFrontMatterSchema, TestLintAreas, TestLintTags |
| FMT-3 | Requirements live in `specs/*.md`, one area per file, each with `**Status:**`, `**Prefix:**`, and a table `ID \| Requirement \| Verified by`. | TestLintAreas |
| FMT-4 | IDs are `PREFIX-n`; prefixes unique within a repo; IDs unique within a repo; never reused; a dropped row says `Removed`. | TestLintAreas |
| FMT-5 | `Verified by` is a stack-free acceptance check: inputs → expected outcome, a scenario name, or `manual: <what to look at>`. Never empty. | TestLintAreas, TestLintVerifiedByRequired |
| FMT-6 | A version is a git tag `vMAJOR.MINOR.PATCH` (semver); the index stores tag + commit SHA. Untagged repos resolve only as `@head`. | TestFieldsRead |
| FMT-7 | Global requirement ID = `name:ID`, e.g. `@specfork/auth:AUTH-3`. | TestBundleLayoutAndRewrite |
| FMT-8 | An implementation repo has topic `specfork-impl` and README front matter `implements: name@version`, `stack: [..]`, optional `url` (live deployment). The spec never lists its implementations. | TestDetectImplRepo, TestImplAttach |
| FMT-9 | References to another spec are global IDs in backticks (`` `@specfork/teams:TEAM-5` ``) or the bare name; never relative paths across repos. | TestLintAreas |
| FMT-10 | (Draft) Cheap-buildable: an area that stores or exchanges data has a `## Data model` section listing every entity with each field's type, whether required, and its limits. | TestLintCheapDataModel |
| FMT-11 | (Draft) Cheap-buildable: an area has a `## Examples` section with at least one worked example per non-trivial row group: concrete input values → exact expected output. | TestLintCheapExamples |
| FMT-12 | (Draft) Cheap-buildable: every `Verified by` is an executable scenario (`given … when … then …` with concrete values, or a command with expected exit code/output); `manual:` is not allowed. | TestLintCheapExecutable |
| FMT-13 | (Draft) Cheap-buildable: an area has ≤ 15 rows and ≤ 60 lines; larger areas are split. | TestLintCheapSize |
| FMT-14 | (Draft) Cheap-buildable: every optional field, limit, ordering and error case states its default or behaviour in the row or data model; none is left to the builder's judgement. | TestLintCheapDefaults |

## Out of scope

- Mandatory depth rules for all specs: FMT-10..14 apply only to specs that opt in (lint profile, see Decisions).
- Non-GitHub hosts.

## Decisions

- Front matter in README, not a separate manifest: one file fewer, and people read the README (Hugging Face model cards).
- Children declare parents (`base`, `implements`); the index inverts the links. No author maintains a list.
- FMT-10..14 are a separate lint profile ("cheap-buildable"), checked per spec and reported as pass/fail, not forced on every spec: existing seeds and hand-written specs stay valid. Rules 12 and 14 are partly judgement; the lint checks the mechanical part (no `manual:`, sections present, size, scenario shape), and build results ([IDX-10](index.md)) are the real test.
- Stack-free still holds: examples and scenarios use values and behaviour, never frameworks, languages or file layouts.
- Areas reuse this repo's own spec format, so a spec repo looks like any `specs/` folder.
