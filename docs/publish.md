# Publish a spec

Any public GitHub repo can be a Specfork spec. You keep the repo and the licence; Specfork indexes it and shows it with credit.

## 1. Write it

The repo needs a `README.md` that starts with front matter, and one `specs/*.md` file per area:

```yaml
---
specfork: 1
name: "@your-login/booking"
summary: One line, at most 200 characters.
license: CC-BY-4.0
tags: [saas]
requires:
  "@specfork/auth": "^0.3.0"
overrides:
  "@specfork/auth:AUTH-8": removed
---
```

Each area file has a title, `**Status:**` and `**Prefix:**`, a short purpose, and a requirements table:

```markdown
| ID | Requirement | Verified by |
|---|---|---|
| BOOK-1 | A booking has a start, an end and an organizer. | create without end → rejected |
```

One testable rule per row. `Verified by` is an acceptance check that doesn't depend on a stack: inputs and the expected outcome.

## 2. Check it

```bash
specfork lint .
```

### Optional: cheap-buildable

Specs precise enough for small, cheap models to build are easier to choose and to trust. `specfork lint --cheap .` also checks, per area:

- a `## Data model` section: every entity and field with type, required or not, limits (write `None.` if the area stores nothing);
- an `## Examples` section with worked examples, each as input → exact output;
- every `Verified by` is a `given … when … then …` scenario with concrete values, or a command with its expected result; no `manual`;
- at most 15 rows and 60 lines per area;
- no vague wording in a requirement (`usually`, `reasonable`, `etc`): state the default or behaviour.

This is a mechanical check. Whether a cheap model really builds the spec is measured separately: once builds are recorded, the site and `specfork info` show `builds with: <tier>` (the cheapest model tier that got every acceptance check green, with its cost), or `not yet measured`. Results belong to one version, so a new version starts unmeasured.

## 3. Release it

Add the topic `specfork` to the repo, then tag a version:

```bash
git tag v0.1.0 && git push --tags
```

## 4. Claim the name

Names are claimed by pull request: see [Claim a name](claim.md). The site lists the spec within the hour.

## Licences

Specs under an open licence (CC BY, CC0, MIT, Apache-2.0, BSD, ISC, MPL-2.0) are shown in full on the site, with credit. Others are listed with a link to the repo.

## Publish an implementation

Built an app from a spec? Add the topic `specfork-impl` and front matter to its README, and it appears on the spec's page:

```yaml
---
implements: "@specfork/scheduling@0.2.0"
stack: [go, sqlite]
url: https://your-live-app.example
---
```
