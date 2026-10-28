# Claim a name

A spec's name, like `@your-login/booking`, is claimed once and then belongs to that repo.

## How

The quick way: sign in with GitHub on this page, pick one of your repos with the `specfork` topic and press Claim. Specfork runs the checks below and opens the pull request for you; the same workflow merges it. You grant only `read:user`.

Or by hand. Open a pull request on [specfork/specfork](https://github.com/specfork/specfork) that adds one entry to `registry/names.json`:

```json
"@your-login/booking": {"repo_id": 123, "owner_id": 456, "repo": "your-login/booking", "claimed_at": "2026-10-22", "state": "active"}
```

Get the IDs with:

```bash
gh api repos/your-login/booking --jq '{repo_id: .id, owner_id: .owner.id}'
```

## What is checked

A workflow checks the pull request and merges it when everything matches:

- the name is free and well formed;
- the scope is your GitHub login, lower-cased;
- `repo_id` and `owner_id` belong to that repo;
- the repo has the `specfork` topic and its README names the same spec;
- you own the repo (or are an admin of the organisation that does).

If something doesn't match, it comments what to fix.

## After a claim

Names are tied to the repo's numeric IDs, so a renamed repo is followed automatically. If a repo is transferred or deleted, its name is frozen until it is claimed again. The `@specfork` scope is reserved for Specfork's own specs.
