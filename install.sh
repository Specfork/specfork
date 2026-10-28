#!/bin/sh
# Install the Specfork CLI (CLI-15):
#   curl -fsSL https://specfork.dev/install.sh | sh
#
# SPECFORK_INSTALL  directory to install into (default ~/.local/bin)
# SPECFORK_VERSION  release to install, e.g. 0.2.0 (default: the latest)
#
# Never uses sudo. Refuses to install a binary whose SHA-256 is not in checksums.txt.
set -eu

# SPECFORK_RELEASES is for tests and mirrors.
releases="${SPECFORK_RELEASES:-https://github.com/Specfork/specfork/releases}"
dir="${SPECFORK_INSTALL:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'specfork install: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported OS $(uname -s); on Windows use: npm i -g specfork" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

if [ -n "${SPECFORK_VERSION:-}" ]; then
  base="$releases/download/v${SPECFORK_VERSION#v}"
else
  base="$releases/latest/download"
fi

if command -v curl >/dev/null 2>&1; then
  get() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  get() { wget -q "$1" -O "$2"; }
else
  fail "needs curl or wget"
fi

if command -v sha256sum >/dev/null 2>&1; then
  sum() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
  sum() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  fail "needs sha256sum or shasum to check the download"
fi

asset="specfork_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Downloading $asset"
get "$base/$asset" "$tmp/$asset" || fail "could not download $base/$asset"
get "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"

want="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "$asset is not listed in checksums.txt"
got="$(sum "$tmp/$asset")"
[ "$got" = "$want" ] || fail "checksum mismatch for $asset (expected $want, got $got); nothing installed"

tar -xzf "$tmp/$asset" -C "$tmp" specfork || fail "could not unpack $asset"
mkdir -p "$dir" || fail "cannot create $dir; set SPECFORK_INSTALL to a directory you can write to"
cp "$tmp/specfork" "$dir/.specfork.new" || fail "cannot write to $dir; set SPECFORK_INSTALL to a directory you can write to"
chmod 755 "$dir/.specfork.new"
mv -f "$dir/.specfork.new" "$dir/specfork"

say "Installed $dir/specfork"
case ":$PATH:" in
  *":$dir:"*) ;;
  *)
    say ""
    say "$dir is not on your PATH. Add it with:"
    say "  export PATH=\"$dir:\$PATH\""
    say "and put that line in your shell profile (~/.profile, ~/.zshrc or ~/.bashrc)."
    ;;
esac
