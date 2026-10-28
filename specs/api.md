# API

**Status:** Live · **Prefix:** API

## Purpose

The one server behind specfork.dev: GitHub sign-in, scores, one-click claims. It is `specfork serve`, the same process as the hosted MCP server ([MCP](mcp.md)). It stores accounts, scores and counters, never spec text ([IDX-13](index.md)).

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| API-1 | `specfork serve` is one HTTP server answering on both `api.specfork.dev` and `mcp.specfork.dev`; every route below works on `api.`, and `/mcp`, `/stats.json` and `/healthz` also work on `mcp.`. It listens on `$PORT` (default 8080). | TestServeBothHosts |
| API-2 | All config comes from env: `DATABASE_URL`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `SESSION_SECRET` (≥ 32 bytes), `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`, `GITHUB_APP_INSTALLATION_ID`, `SCORE_MIN_ACCOUNT_AGE_DAYS` (default 30). A missing required variable stops startup with exit code 1 and an error naming the variable, never its value. | TestConfigRequired, TestConfigNoSecretInError |
| API-3 | `GET /healthz` returns `200 {"status":"ok"}` when Postgres answers a ping within 2 s, else `503 {"status":"unavailable"}`; no login, no other detail. | TestHealthz |
| API-4 | On start the server applies numbered SQL migrations in one transaction each and records them in `schema_migrations`; a second start applies nothing. | TestMigrateIdempotent |
| API-5 | Logs are JSON lines on stdout: time, level, route pattern, status, duration. Never tokens, cookies, query strings, GitHub logins, IDs or IP addresses. | TestLogNoPersonalData |
| API-6 | The image runs as a non-root user with a read-only root filesystem, holds only the static binary, and is built and signed by this repo's CI (synced `app-image.yml`), tagged with the commit SHA. | manual: `docker run --read-only --user 10001` serves `/healthz`; `cosign verify` passes |
| API-7 | CORS: `Access-Control-Allow-Origin: https://specfork.dev` with credentials on session routes; `*` without credentials on `/scores.json` and `/stats.json`. Every `PUT`, `POST` and `DELETE` also needs the header `X-Specfork: 1`, else `400`. | TestCORS, TestCSRFHeader |
| API-8 | `GET /auth/github/login?return=<path>` sets a random `state` cookie (10 min) and redirects to GitHub OAuth asking only for the scope `read:user`; `return` must be a path on `https://specfork.dev`, else it is ignored. | TestLoginRedirect, TestLoginReturnPath |
| API-9 | `GET /auth/github/callback` checks `state`, exchanges the code, reads the user's numeric ID and login, upserts that user with the sign-in time, discards the GitHub token without storing it, sets the session and redirects to `return`. A bad `state` or a GitHub error redirects with `?auth=failed` and creates nothing. | TestCallbackUpsert, TestCallbackBadState, TestTokenNotStored |
| API-10 | The session is an HMAC-signed cookie `__Host-session` (`HttpOnly`, `Secure`, `SameSite=Lax`, 30 days) holding the GitHub user ID and expiry; no server-side session table. A tampered or expired cookie counts as signed out. | TestSessionCookie, TestSessionTamper |
| API-11 | `GET /me` returns `{id, login}` for a signed-in user, else `401`. | TestMe |
| API-12 | `POST /logout` clears the session cookie and returns `204`. | TestLogout |
| API-13 | `DELETE /me` deletes the user row and all their scores in one transaction, clears the cookie and returns `204`; claim PRs already opened stay. | TestDeleteAccount |
| API-14 | The database holds only: `users(github_id, login, signed_in_at)`, `scores(github_id, spec, score, updated_at)`, `counters` ([MCP-10](mcp.md)) and `schema_migrations`. No email, no password, no token, no IP, no spec text. | TestSchemaColumns |
| API-15 | `PUT /scores/{scope}/{name}` with `{"score": 1..5}` (integer) sets the signed-in user's one score for that spec, replacing the earlier one; `{scope}/{name}` must be an `active` spec in the index, else `404`; a bad score is `400`; signed out is `401`. | TestPutScore, TestPutScoreUnknownSpec, TestPutScoreRange |
| API-16 | `DELETE /scores/{scope}/{name}` removes the signed-in user's score and returns `204`, also when there was none. | TestDeleteScore |
| API-17 | `GET /scores/{scope}/{name}/mine` returns `{score}` (or `{score: null}`) for the signed-in user; it is never cached. | TestMyScore |
| API-18 | A score is accepted only if the GitHub account is at least `SCORE_MIN_ACCOUNT_AGE_DAYS` days old, read from GitHub at that request and not stored; a younger account gets `403 {"error":"account_too_new"}`. | TestScoreAccountAge |
| API-19 | Each user may make at most 60 writes (`PUT`, `DELETE`, `POST /claim`) per hour; more get `429` with `Retry-After`. | TestWriteRateLimit |
| API-20 | `GET /scores.json` returns `{generated_at, scores: [{spec, average, count}]}` with `average` rounded to 2 decimals, only for specs with `count ≥ 1`, no user data; `Cache-Control: public, max-age=300`. | TestScoresJSON, TestScoresJSONNoUsers |
| API-21 | The server reads the site's `index.json` for active names and keeps it for 10 minutes; if it cannot be fetched it keeps serving the last copy, and answers `503` if there is none. | TestIndexCache |
| API-22 | `GET /claim/repos` (signed in) lists the user's own repos with topic `specfork` whose name is not in `names.json`: `{repo, repo_id, name, claimable, failures}`, where `name` is the README front matter `name`. Reads use the Specfork GitHub App, not the user's token. | TestClaimRepos |
| API-23 | `POST /claim` with `{"repo_id"}` (signed in) runs the REG-2 checks for that repo with the signed-in user as claimant, using the same code as the `claim` workflow; any failure returns `422` with the list and opens nothing. | TestClaimChecksServer |
| API-24 | When the checks pass, the server pushes a branch `claim/<name>` (the name as is, for example `claim/@ann/booking`) to `specfork/specfork` as the Specfork GitHub App, adds only the new entry to `registry/names.json`, opens a PR whose body has the line `Claimant: <github_id>`, and returns `201 {pr_url}`. | TestClaimOpensPR |
| API-25 | A claim for a name that already has an open claim PR from the bot returns `200` with that PR's URL and opens nothing new. | TestClaimIdempotent |
| API-26 | If GitHub fails or is rate limited during sign-in, a score check or a claim, the response is `502 {"error":"github_unavailable"}` and nothing is written. | TestGitHubDown |

API-27..29 are reserved and unused.

## Out of scope

- Email, passwords, other sign-in providers, profile pages, comments.
- Writing to `main` or merging: the server only opens PRs; the `claim` workflow merges ([REG-9](registry.md)).
- Moving the site off GitHub Pages ([IDX](index.md) decision).

## Decisions

- One app, two hostnames: `mcp.specfork.dev` stays as the MCP docs say; `api.specfork.dev` is the name for everything else, and `/mcp` there too.
- Scores are 1–5 per spec (not per version), shown with the latest version: texts change between versions, but people rate the recipe.
- The GitHub token is thrown away after sign-in: Specfork keeps nothing that could be leaked and used on GitHub. Claims use the GitHub App instead, so users grant `read:user` only.
- Stateless signed cookie: no session table to store or clean; delete-account needs no session cleanup.
- Site and API share the registrable domain `specfork.dev`, so a `SameSite=Lax` cookie works for calls from the static site.
- The minimum account age (30 days) is a cheap spam brake, not identity proof; it is a variable so it can change without a release.
