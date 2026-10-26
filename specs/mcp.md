# MCP

**Status:** Live · **Prefix:** MCP

## Purpose

Let any agent find and resolve specs through MCP.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| MCP-1 | `specfork mcp` serves MCP over stdio. Claude Code: `claude mcp add specfork -- specfork mcp`. | TestMCPHandshake |
| MCP-2 | Tools: `search_specs(query, limit≤20)`, `get_spec(name, version?)` → metadata + area list, `resolve_spec(name, range?)` → bundle as one Markdown document with `BUNDLE.md` first. | TestMCPTools |
| MCP-3 | Uses the same index and resolve code as the CLI. | TestMCPTools, TestInfo |
| MCP-4 | Removed: usage counters move out of scope until the server is hosted (see MCP-7). | — |
| MCP-5 | Removed: superseded by MCP-1 (stdio first). | — |
| MCP-6 | A hosted server at `https://mcp.<domain>/mcp` speaks MCP over streamable HTTP, no login, with exactly the tools of MCP-2 and the same index and resolve code. | TestHostedHandshake, TestHostedTools |
| MCP-7 | Each tool call adds 1 to a counter keyed `(UTC day, tool, spec name)`; `search_specs` has an empty spec name; failed calls and names not in the index count under `unknown`. Nothing else is stored or logged: no IP, user agent, headers, query text or session ID. | TestCounterKey, TestCounterNoPersonalData |
| MCP-8 | `GET /stats.json` returns the daily counters (`day`, `tool`, `spec`, `count`) for the last 90 days, public and cacheable for 5 minutes. | TestStatsJSON |
| MCP-9 | Spec pages and the catalogue show resolves per spec (last 30 days and total) from `/stats.json` at site build; with no data the numbers are omitted, never shown as 0. | TestSiteCounters |

## Out of scope

- Write tools (publish, claim).
- Per-user or per-agent statistics, rate-limit identity, accounts.
- Counting stdio (`specfork mcp`) use: it stays local and unreported.

## Decisions

- Stdio first: works today with no hosting; the hosted server reuses the same tools.
- Counters are the only state: a daily aggregate cannot be traced to a person, so no consent banner is needed; the privacy statement is published on `/docs/`.
- Host: Cloudflare Workers with a D1 table for counters; DNS is already at Cloudflare (IDX), and the host is outside GitHub Pages/Actions as IDX requires before anything commercial.
- Domain: `mcp.specfork.dev`, the existing domain; a later rename changes the hostname, and stdio users are unaffected.
- Site stays static: counters enter at build via `/stats.json` (daily rebuild), not by browser calls, keeping WEB-16 (no third-party scripts).
