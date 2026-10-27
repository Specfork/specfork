# Use Specfork with your agent

Specfork gives your coding agent a complete, testable description of an app to build: every requirement is numbered and has an acceptance check, and everything the app builds on (sign-in, teams, webhooks) comes along in one bundle.

## Install

```bash
go install github.com/specfork/specfork/cmd/specfork@latest
```

This needs Go 1.25 or newer. The command is `specfork`.

## Find a spec

```bash
specfork search booking
specfork info @specfork/scheduling
```

Or browse the [catalogue](/#specs).

## Get the bundle

```bash
specfork resolve @specfork/scheduling
```

This writes `specfork-bundle/` and `specfork.lock` in the current folder:

- `specfork-bundle/BUNDLE.md` lists every spec in the bundle, dependencies first, with versions, commits and licences.
- One folder per spec holds its README and area files. Requirement IDs are global (`@specfork/auth:AUTH-3`) so nothing collides.
- `specfork.lock` pins the exact commits; run `resolve` again to get the same bundle, or `--update` to move to newer versions.

Then tell your agent: *"Build the app described in `specfork-bundle/BUNDLE.md`. Turn every Verified by check into a test."*

## Let the agent fetch specs itself (MCP)

```bash
claude mcp add specfork -- specfork mcp
```

The agent gets three tools: `search_specs`, `get_spec` and `resolve_spec` (the whole bundle as one Markdown document). Any MCP-capable agent can run `specfork mcp` the same way.

Agents that cannot install the CLI can use a hosted server instead. `specfork serve` runs one over streamable HTTP at `/mcp` with the same three tools and no login; point a client at it, e.g. `claude mcp add --transport http specfork <server-url>/mcp`.

### What the hosted server counts

The hosted server adds 1 to a daily counter for each tool call, keyed by day (UTC), tool and spec name. Failed calls and unknown names count as `unknown`. It stores and logs nothing else: no IP address, user agent, headers, search text or session ID. The counters are public at `/stats.json` (last 90 days) and show on spec pages as resolves per spec. Local `specfork mcp` is never counted.

## Change what you need

Specs are recipes, not rules:

- **Small changes:** write your own spec that requires the original and overrides single requirements, e.g. `"@specfork/auth:AUTH-8": removed`. See [Publish a spec](publish.md).
- **Big changes:** fork the spec's GitHub repo and edit it.

## For tools

The whole catalogue is one file: [index.json](/index.json). A short summary for AI tools is at [llms.txt](/llms.txt).
