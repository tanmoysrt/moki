#!/bin/sh
# Installs the newest moki release in ~/.local/bin (or $MOKI_BIN).
#
#   curl -fsSL https://raw.githubusercontent.com/tanmoysrt/moki/develop/install.sh | sh
set -eu

repo="tanmoysrt/moki"
bin="${MOKI_BIN:-$HOME/.local/bin}"

fail() {
  echo "moki: $*" >&2
  exit 1
}

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "no build for $(uname -s). Use: go install github.com/$repo@latest" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no build for $(uname -m). Use: go install github.com/$repo@latest" ;;
esac

file="moki-$os-$arch"
url="https://github.com/$repo/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $file"
curl -fsSL -o "$tmp/$file" "$url/$file"
curl -fsSL -o "$tmp/checksums.txt" "$url/checksums.txt"

want="$(grep " $file\$" "$tmp/checksums.txt" | cut -d ' ' -f 1)"
if command -v sha256sum >/dev/null; then
  got="$(sha256sum "$tmp/$file" | cut -d ' ' -f 1)"
else
  got="$(shasum -a 256 "$tmp/$file" | cut -d ' ' -f 1)"
fi
[ -n "$want" ] && [ "$want" = "$got" ] || fail "the checksum of $file does not match"

mkdir -p "$bin"
install -m 755 "$tmp/$file" "$bin/moki"
echo "Installed $("$bin/moki" version) in $bin/moki"

case ":$PATH:" in
  *":$bin:"*) ;;
  *) echo "Add $bin to your PATH to run moki." ;;
esac
