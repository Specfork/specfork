# Specfork

**Recipes for software.** Fork an app spec, change what you need, and let your coding agent build it. Specs don't depend on a stack: one recipe, many implementations.

**Browse:** https://specfork.dev

```bash
go install github.com/specfork/specfork/cmd/specfork@latest
specfork search booking
specfork resolve @specfork/scheduling
```

`resolve` writes `specfork-bundle/` (the spec and everything it requires, IDs made global, overrides applied) and `specfork.lock`. Point your agent at `specfork-bundle/BUNDLE.md`, or let it fetch specs itself: `claude mcp add specfork -- specfork mcp`. `specfork serve` runs the same tools as a hosted MCP server (streamable HTTP) with anonymous daily usage counters.

- [Use Specfork with your agent](https://specfork.dev/docs/)
- [Publish a spec](https://specfork.dev/docs/publish/)
- [Claim a name](https://specfork.dev/docs/claim/)

## How it works

- Specs live in their own repos; the index stores no spec text, and the site shows open-licence specs with credit ([IDX](specs/index.md), [WEB](specs/site.md)).
- Names are bound to GitHub repo and owner IDs, so a renamed repo is followed and a transferred one is frozen ([REG](specs/registry.md)).
- An hourly workflow crawls GitHub and publishes `index.json` and the site.
- `resolve` uses minimal version selection, pins commits in `specfork.lock`, and fetches files by SHA ([CMP](specs/compose.md)).
- Seed specs are extracted from open-source apps ([EXT](specs/extract.md)).

Specs: [specs/README.md](specs/README.md).
