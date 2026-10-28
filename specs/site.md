# Site

**Status:** Live · **Prefix:** WEB

## Purpose

https://specfork.dev: find a spec, read it, and build it with your agent. Static pages generated from `index.json` and the specs at their indexed commits.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| WEB-1 | Pages: `/` (home), `/<scope>/<name>/` per indexed spec (scope without `@`), `/docs/`, `/docs/publish/`, `/docs/claim/`, `/docs/feedback/`, `/404.html`, plus `/index.json`, `/sitemap.xml`, `/robots.txt`, `/llms.txt`. | TestSitePages |
| WEB-2 | Home intro: "Recipes for software", one paragraph on what Specfork is, three steps (pick a recipe, fork and adjust, build with your agent) and links to the catalogue, `/docs/` and `/docs/publish/`. | TestSitePages |
| WEB-3 | Catalogue: a card per spec with name, summary, tags, latest version, requirement count at that version, licence and implementation count; frozen specs are marked; base specs (tag `base`) listed after the others. | TestSiteCatalogue |
| WEB-4 | Filters: a chip per tag with its spec count, and a search box matching every word against name, summary, tags and area titles; `?q=` and `?tag=` prefill both and stay in the URL. Without JavaScript every card is shown. | TestSiteCatalogue; manual: filter in a browser |
| WEB-5 | Spec page header: name, summary, tags, latest version, licence, repo, source credit (`derived_from`), and a "Build it" box with `npx specfork resolve <name>` and the MCP setup line `claude mcp add specfork -- npx -y specfork mcp`, each with a copy button. | TestSpecPage |
| WEB-6 | Spec content: the README body and every area at the latest version (head if untagged), areas in file order, each area an anchored section; every requirement row has the anchor `#<ID>`; in-spec links go to those anchors; global ID references link to the other spec's page and anchor when it is indexed. | TestSpecContent |
| WEB-7 | Content is shown only when the spec's licence is one of `CC-BY-4.0`, `CC-BY-SA-4.0`, `CC0-1.0`, `MIT`, `Apache-2.0`, `BSD-2-Clause`, `BSD-3-Clause`, `ISC`, `MPL-2.0`, `Unlicense`; otherwise the page links to the repo. Shown content carries "From <repo> at <commit>, licence <id>." | TestSpecContent |
| WEB-8 | Rendering is safe: raw HTML in specs is escaped; `javascript:`, `vbscript:` and `data:` links are dropped; external links get `rel="nofollow ugc noopener"`. | TestRenderSafety |
| WEB-9 | Bundle panel: the specs `specfork resolve` would bundle for the latest version, each with version and requirement count, and the total. | TestSpecPage |
| WEB-10 | Spec page also lists required-by, implementations (repo, stack, live URL) and the version history. | TestSpecPage |
| WEB-11 | Docs pages are rendered from `docs/*.md` in this repo. | TestSitePages |
| WEB-12 | Every page has a unique `<title>` and meta description (≤ 160 chars), a canonical URL on `https://specfork.dev`, Open Graph and Twitter card tags, and the 1200×630 share image `/og.png`. | TestSEO |
| WEB-13 | JSON-LD: the home page has `WebSite` with a `SearchAction` on `/?q={search_term_string}`; each spec page has `CreativeWork` with name, description, version, license, url, author and `isBasedOn` (derived_from). | TestSEO |
| WEB-14 | `sitemap.xml` lists home, docs and every spec page, with `lastmod` from the spec's `pushed_at`; `robots.txt` allows all and names the sitemap. | TestSEO |
| WEB-15 | `llms.txt`: what Specfork is, how to use it (CLI, MCP, `index.json`), and one line per spec with its page URL and summary. | TestSEO |
| WEB-16 | No third-party scripts, fonts, trackers or cookies (WEB-21 allows the first-party script); usable at 360 px; light and dark themes. | manual: network tab, phone width |
| WEB-17 | Feedback: `/docs/feedback/` and a "Feedback" link in the header and footer; GitHub issue forms on `specfork/specfork` for feedback, spec requests and spec problems, plus Discussions; every spec page links to the problem form with the spec name prefilled; an empty search links to the request form. | TestFeedbackLinks |
| WEB-18 | Spec pages and catalogue cards show the average score (1 decimal) and number of scores from `/scores.json` at site build; a spec without scores shows nothing, never 0. | TestSiteScores |
| WEB-19 | The spec page has a score box: signed out, a "Sign in with GitHub to score" link to `api.specfork.dev/auth/github/login`; signed in, five buttons (1–5) showing the user's score, plus "Remove my score" and "Sign out". Without JavaScript the box shows only the built average. | TestSiteScoreBox; manual: score and remove in a browser |
| WEB-20 | `/docs/claim/` has a one-click claim section: signed in, it lists the user's claimable repos ([API-22](api.md)) with a "Claim" button that calls `POST /claim` and links the PR, or shows the failures; the hand-written PR steps stay on the page. | TestSiteClaimBox; manual: claim a test repo |
| WEB-21 | The only script the site adds is first-party (`/assets/specfork.js`); it calls only `https://api.specfork.dev`, only when the page has a score or claim box, and the site sets no cookies itself. | TestSiteScriptFirstParty |
| WEB-22 | `/docs/privacy/` states what is stored (GitHub user ID, login, last sign-in time, scores, daily counters), what is not (email, passwords, tokens, IP, spec text), and how to delete the account; it is linked from the footer and the score box. | TestSitePages |
| WEB-23 | `https://specfork.dev/install.sh` serves the CLI-15 script as `text/plain`, byte for byte the file in this repo. | TestSitePages |
| WEB-24 | The README and `/docs/` lead with `npx specfork …`, and list `npm i -g specfork`, `brew install specfork/tap/specfork`, the `curl -fsSL https://specfork.dev/install.sh \| sh` line and `go install` as the other ways; `/llms.txt` names the same ways. | TestSitePages, TestSEO |

## Out of scope

- Usage numbers on pages: defined in MCP-9; shown only with real data, no empty counters.
- Comments on the site; accounts beyond sign-in for scores and claims ([API](api.md)).
- A feedback form without a GitHub account.

## Decisions

- Static pages: fast, cacheable, best for search; the hosted layer adds dynamic parts later.
- Spec content is rendered at build time from the indexed commit, credited, licence-permitting; it is a view, not a stored mirror.
- `/<scope>/<name>/` mirrors the spec name, like package registries.
