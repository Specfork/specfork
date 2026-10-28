# Specs

Behaviour changes start here: change the requirement rows first, code in the same PR.
New area → copy `_template.md`.

One requirement = one testable line. IDs never reused; dropped → `Removed`.
`Verified by` = test name, command, or `manual` (say what to look at).
`Draft` = agreed, not built; `Verified by` then names the planned check.

Short but technical: interfaces, fields, formulas, limits. ~30 lines per spec.

| Spec | Prefix | Covers |
|---|---|---|
| [format.md](format.md) | FMT | Spec repo layout, README front matter, versions, implementation repos |
| [registry.md](registry.md) | REG | Scoped names, claims by PR, binding to repo/owner IDs, freezes |
| [index.md](index.md) | IDX | Crawler, `index.json`, change detection, rate limits |
| [site.md](site.md) | WEB | specfork.dev: catalogue, spec pages with content, docs, SEO |
| [compose.md](compose.md) | CMP | `requires`, version selection, global IDs, overrides, lockfile, bundle |
| [cli.md](cli.md) | CLI | `specfork search`, `info`, `resolve`, `lint` |
| [mcp.md](mcp.md) | MCP | MCP server for agents, usage counters |
| [api.md](api.md) | API | `specfork serve` backend: GitHub sign-in, scores, one-click claims |
| [extract.md](extract.md) | EXT | Seed pipeline: extract from open-source app, factor bases, rebuild, publish |
