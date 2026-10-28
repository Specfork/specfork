# Registry

**Status:** Live: REG-1..REG-7 · **Prefix:** REG

## Purpose

Specfork owns the names. A name points at one GitHub repo, bound by IDs, claimed by pull request.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| REG-1 | Names live in `registry/names.json` in `specfork/specfork`: `{"names": {name: {repo_id, owner_id, repo, claimed_at, state}}}`; `state` is `active` or `frozen`. | TestRegistrySchema |
| REG-2 | A claim is a PR that only adds entries to `names.json`. The `claim` workflow (`pull_request_target`, never runs PR code) checks each entry: the name is free and well-formed; `repo_id`/`owner_id` match the repo; the repo has topic `specfork`; its README front matter `name` equals the claimed name; the PR author is the repo owner (user repos) or has `admin` on it (org repos). All pass → squash-merged; else a comment lists the failures. | TestClaimChecks |
| REG-3 | The scope `@x` must equal the repo owner's GitHub login, lower-cased, at claim time. `@specfork` is reserved for seeds: those claims are never auto-merged. Scopes that clash with site paths (`@docs`, `@r`, `@api`, `@assets`, `@s`) are refused. | TestClaimScope |
| REG-4 | Names bind to numeric `repo_id` and `owner_id`, never to the `owner/repo` URL. | TestBindIDs |
| REG-5 | The crawler looks repos up by `repo_id`; if the repo is gone or its `owner_id` differs (transfer, delete, recreate), the index marks the name `frozen`: listed, not resolvable, until a new claim PR. | TestFreezeOnIDChange |
| REG-6 | A renamed repo (same `repo_id`) is followed automatically: the index shows its current `owner/repo`. | TestRenameFollows |
| REG-7 | Names are never reassigned to another repo without a manual review in the PR. | manual: PR has `reassign` label and a maintainer approval |
| REG-8 | A signed-in user can claim a name without editing JSON: [API-24](api.md) opens the claim PR as the Specfork GitHub App; a hand-written PR (REG-2) keeps working. | TestClaimOpensPR |
| REG-9 | The `claim` workflow accepts a PR from the Specfork App when its body has one line `Claimant: <github_id>`; REG-2's "PR author is the owner/admin" is then checked for that claimant ID, not the bot. The line is ignored on PRs from anyone else. | TestClaimBotClaimant, TestClaimBotLineIgnored |
| REG-10 | An App PR may change only `registry/names.json` and only add entries; any other change is refused with a comment and no merge. REG-2 and REG-3 apply unchanged. | TestClaimBotOnlyNames |
| REG-11 | `names.json` stays the only source of truth for names: the server stores no names, never pushes to `main`, and never edits or removes an existing entry. | TestClaimOpensPR, TestSchemaColumns |
| REG-12 | A claimant who is not the owner (user repos) or an `admin` (org repos) gets the failure list from the server ([API-23](api.md)) and no PR; the workflow would refuse it too. | TestClaimChecksServer, TestClaimBotClaimant |

## Out of scope

- Accounts beyond what [API](api.md) stores; the PR flow still needs no server.
- Verified org badges (DNS TXT).

## Decisions

- Registry as a file in a git repo (Obsidian community plugins, Homebrew taps): free, auditable history, no database.
- IDs over URLs: blocks repojacking, where a freed username is re-registered and the repo recreated.
- Renamed from SpecHub to Specfork (2026-10-22): `spechub` on GitHub belongs to an unrelated organisation and Postman has a pending "Postman Spec Hub" trademark in the same field. Code, site and names live in the `specfork` GitHub organisation; `@specfork` is the reserved seed scope.
- No bot commits: freezes and renames are computed by the crawler into the index, not written back to `names.json`.
