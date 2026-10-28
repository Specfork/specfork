#!/bin/sh
# Prints the Homebrew formula for a release (CLI-14):
#   scripts/brew-formula.sh <version, e.g. 1.2.3> <checksums.txt>
set -eu

version="${1:?usage: brew-formula.sh <version> <checksums.txt>}"
version="${version#v}"
sums="${2:?usage: brew-formula.sh <version> <checksums.txt>}"

sha() {
  awk -v f="specfork_$1.tar.gz" '$2 == f { print $1; exit }' "$sums"
}
for t in darwin_arm64 darwin_amd64 linux_arm64 linux_amd64; do
  [ -n "$(sha "$t")" ] || { echo "no checksum for specfork_$t.tar.gz in $sums" >&2; exit 1; }
done

cat <<FORMULA
class Specfork < Formula
  desc "Find, compose and build from app specs"
  homepage "https://specfork.dev"
  version "$version"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Specfork/specfork/releases/download/v$version/specfork_darwin_arm64.tar.gz"
      sha256 "$(sha darwin_arm64)"
    else
      url "https://github.com/Specfork/specfork/releases/download/v$version/specfork_darwin_amd64.tar.gz"
      sha256 "$(sha darwin_amd64)"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Specfork/specfork/releases/download/v$version/specfork_linux_arm64.tar.gz"
      sha256 "$(sha linux_arm64)"
    else
      url "https://github.com/Specfork/specfork/releases/download/v$version/specfork_linux_amd64.tar.gz"
      sha256 "$(sha linux_amd64)"
    end
  end

  def install
    bin.install "specfork"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/specfork --version")
  end
end
FORMULA
