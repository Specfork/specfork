# CLI

**Status:** Live · **Prefix:** CLI

## Purpose

`specfork`, for people and agents.

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| CLI-1 | `specfork search <query>` prints matching names, summary, latest version, stars, implementation count; `--json` for machines. Matches name and summary, case-insensitive, all words. | TestSearch |
| CLI-2 | `specfork info <name>` prints versions, requires, required-by, implementations with live URLs, licence, attribution; `--json`. | TestInfo |
| CLI-3 | `specfork resolve <name>[@range] [--out dir]` writes the bundle ([CMP-7](compose.md)) and `specfork.lock` in the current directory; default out `./specfork-bundle`. | TestResolve |
| CLI-4 | `specfork lint [dir]` checks [FMT-1..5, FMT-9](format.md) on a local spec repo; exit 1 on errors; warnings don't fail. | TestLint |
| CLI-5 | The index is fetched from `https://specfork.dev/index.json` (overridable by `SPECFORK_INDEX`, a URL or file path), cached in the user cache dir for 10 minutes. | TestIndexFetch |
| CLI-6 | Spec files are fetched from `raw.githubusercontent.com` at the resolved commit SHA; no API calls, no token needed. | TestFetchBySHA |
| CLI-7 | Written in Go, no cgo; `go install github.com/specfork/specfork/cmd/specfork@latest` always works (other ways to install: CLI-12..15). | CI: `go build ./...` with `CGO_ENABLED=0` |
| CLI-8 | `specfork mcp` runs the MCP server on stdio ([MCP](mcp.md)). | TestMCPTools |
| CLI-9 | Exit codes: 0 ok, 1 lint errors or not found, 2 usage error, 3 network or index error. | TestExitCodes |
| CLI-10 | Releases are versioned: release-please opens a release PR from Conventional Commits; merging it creates the `v*` tag, `CHANGELOG.md` and a GitHub release. The release workflow approves the CI runs GitHub holds on its release PR (MajidBabaeifar/ci REPO-5), so the PR gets `ci-ok`. | manual: merge a release PR, check tag and release |
| CLI-11 | Each GitHub release has static binaries (`CGO_ENABLED=0`) for linux, darwin and windows on amd64 and arm64, a `checksums.txt` (SHA-256) and GitHub build-provenance attestations. | CI: release workflow; manual: `gh attestation verify` on one asset |
| CLI-12 | `npx specfork <args>` and `npm i -g specfork` run the matching prebuilt binary: the `specfork` package depends optionally on `@specfork/cli-<os>-<arch>` packages (one per CLI-11 target), a small launcher picks the one for the host and passes arguments, stdin, stdout and exit code through; no postinstall script, no download at install or run time. | TestNpmLauncher; CI: `npm pack` of every package, run `--version` on the host one |
| CLI-13 | All npm packages carry the release version and are published from the release workflow with npm trusted publishing (OIDC, provenance on), with no stored npm token; platform packages go first, `specfork` last. | CI: release workflow; manual: `npm view specfork dist.attestations` |
| CLI-14 | `brew install specfork/tap/specfork` installs the release binary from `Specfork/homebrew-tap`; each release updates the formula (version, URLs, SHA-256) automatically. | CI: tap update step; manual: `brew install` on macOS and Linux |
| CLI-15 | `install.sh` detects OS and architecture, downloads the matching CLI-11 asset, refuses to install unless its SHA-256 matches `checksums.txt`, installs to `$SPECFORK_INSTALL` or `~/.local/bin`, never uses `sudo`, and prints how to add the directory to `PATH` when it is missing. `SPECFORK_VERSION` pins a release; default is the latest. | TestInstallScript (sh, fake release server): ok, bad checksum, unsupported OS, custom dir |
| CLI-16 | `specfork --version` prints the release version (`dev` for a source build) and exits 0. | TestVersion |
| CLI-17 | `specfork add <name>[@range] [--out dir]` is an alias of `specfork resolve` with identical flags, output and exit codes; `resolve` stays. | TestAddAlias |

## Out of scope

- `specfork claim` (opens the registry PR).
- Scoop, winget and homebrew-core: later; the npm package covers Windows for now.

## Decisions

- Fetch by SHA, not tag: tags can be moved; SHAs can't.
- Go, not TypeScript: one static binary, installable from GitHub with no package-registry account; the crawler and MCP server share the same code. Packaging (npm, brew, curl) only changes how the binary is delivered.
- Per-platform npm packages, not a postinstall download: works with `--ignore-scripts`, offline caches and locked-down CI.
- Release workflows are synced copies of the ones in `ci` (public repo).
