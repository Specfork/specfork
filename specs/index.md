# Index

**Status:** Live · **Prefix:** IDX

## Purpose

Find spec and implementation repos on GitHub and publish one `index.json`. The index stores no spec text; the site ([WEB](site.md)) renders it.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| IDX-1 | The `index` workflow runs hourly, on pushes to `main` and on demand: `specfork crawl` searches `topic:specfork fork:true` and `topic:specfork-impl fork:true`, and looks up every repo in `names.json` by `repo_id`. | TestCrawlSources |
| IDX-2 | Search results over 1,000 (GitHub's cap per query) are split by `created:` date ranges until each range is under the cap. | TestSearchPartition |
| IDX-3 | Per spec repo it reads: `repo_id`, `owner_id`, current `owner/repo`, `stargazers_count`, `pushed_at`, topics, front matter `tags`; default-branch head SHA and every `v*` semver tag with its commit SHA; at each of those commits the README front matter and the list of `specs/*.md` paths. A tag whose front matter lacks `specfork: 1` or names a different spec than the head is ignored. No other file contents. | TestFieldsRead, TestOldNameTagsIgnored |
| IDX-4 | Only names `active` in the registry, bound to the repo's IDs and not frozen ([REG-5](registry.md)) are resolvable; other spec repos appear under `unclaimed`. | TestUnclaimed |
| IDX-5 | Change detection: the previous `index.json` is fetched from the site; a spec whose head SHA and tag set are unchanged is copied from it without further reads. | TestSkipUnchanged |
| IDX-6 | `index.json`: `{generated_at, specs: [{name, repo, repo_id, owner_id, state, summary, license, tags, stars, pushed_at, derived_from, base, head_sha, head: {requires, overrides, files}, versions: [{version, sha, requires, overrides, files}], implementations: [{repo, stack, url, version}]}], unclaimed: [repo]}`; versions sorted newest first. | TestIndexSchema |
| IDX-7 | Implementations attach to a spec by their `implements` name; unknown names are dropped. | TestImplAttach |
| IDX-8 | The crawler uses one token, honours `X-RateLimit-Remaining` per `X-RateLimit-Resource` (stops when `core` < 100 or `search` < 2, keeping the previous entries for unread specs), and never scrapes HTML. | TestRateLimitStop, TestRateLimitPerResource |
| IDX-9 | Removed: superseded by [WEB](site.md). | — |
| IDX-10 | (Draft) Build results are records `{spec, version_sha, tier, model, checks_passed, checks_total, cost_usd, ran_at}`, published by the runner to `builds.json` in this repo; the crawler reads that file only. Tiers come from `tiers.json` (ordered cheapest first). A record whose `version_sha` matches no indexed version is dropped. | TestBuildsRead, TestBuildUnknownSha |
| IDX-11 | (Draft) A version's `builds_with` is the cheapest tier with a record where `checks_passed == checks_total` and `checks_total > 0`; with the lowest `cost_usd` among that tier's records as `cost_usd`. No such record → `builds_with` is `null` (never "none" or a guess). | TestBuildsWith, TestBuildsWithNone |
| IDX-12 | (Draft) `index.json` versions gain `builds_with: {tier, cost_usd} \| null` and `builds: [{tier, model, checks_passed, checks_total, cost_usd}]`; `specfork info` ([CLI-2](cli.md)) and the spec page ([WEB](site.md)) show "builds with: <tier>" or "not yet measured". | TestIndexSchema, TestInfoBuildsWith |

## Out of scope

- Webhooks via a GitHub App, GH Archive: when hourly polling stops fitting the rate limit.
- Running the builds: nothing in IDX-10..12 starts one. Measuring begins only when Majid says.

## Decisions

- Results are keyed by commit SHA, so a new version starts "not yet measured"; old results never carry over to changed text.
- Tiers are data (`tiers.json`), not code: which models count as "cheap" can change without a release.
- Published to Pages, not committed: no bot commits in `main`; the previous index on the site is the crawler's memory.
- The site is served by GitHub Pages at the custom domain `specfork.dev` (DNS at Cloudflare, records in `deploy/dns/`); `specfork.github.io/specfork` redirects there.
- GitHub Pages and Actions fit GitHub's terms while Specfork is non-commercial and the crawl publishes this project's own site (≈25 API calls/hour). Before charging for anything, move the site and crawler to paid hosting: Pages is not for running a business.
