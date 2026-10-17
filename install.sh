#!/bin/sh
# Installs the managents helper for the current user and registers it as a login service.
#
#   curl -fsSL https://github.com/tonylook/managents/releases/latest/download/install.sh | sh
#
# Running it again upgrades. Nothing needs sudo. Settings, all optional:
#   MANAGENTS_VERSION     release to install, for example v0.2.0 (default: the latest)
#   MANAGENTS_BIN_DIR     where the binary goes (default: ~/.local/bin)
#   MANAGENTS_BASE_URL    download from here instead of GitHub, for example file:///path/to/dist
#   MANAGENTS_NO_SERVICE  set to 1 to skip the service step
set -eu

repo=https://github.com/tonylook/managents
bin_dir=${MANAGENTS_BIN_DIR:-$HOME/.local/bin}
version=${MANAGENTS_VERSION:-latest}

say() {
	printf '%s\n' "$*"
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

# sha256_of prints the SHA-256 digest of a file.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1"
	else
		shasum -a 256 "$1"
	fi | cut -d ' ' -f 1
}

case "$(uname -s)/$(uname -m)" in
Darwin/*) platform=darwin_universal ;;
Linux/x86_64) platform=linux_amd64 ;;
Linux/aarch64 | Linux/arm64) platform=linux_arm64 ;;
*) fail "no download for $(uname -s)/$(uname -m). Windows: get managents_windows_amd64.zip from $repo/releases/latest" ;;
esac
asset=managents_$platform.tar.gz

if [ -n "${MANAGENTS_BASE_URL:-}" ]; then
	base=$MANAGENTS_BASE_URL
elif [ "$version" = latest ]; then
	base=$repo/releases/latest/download
else
	base=$repo/releases/download/v${version#v}
fi

command -v curl >/dev/null 2>&1 || fail "curl is required"

tmp=$(mktemp -d)
staged=
cleanup() {
	rm -rf "$tmp"
	if [ -n "$staged" ]; then
		rm -f "$staged"
	fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

say "Downloading $asset ($version)"
curl -fsSL -o "$tmp/$asset" "$base/$asset" || fail "cannot download $base/$asset"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "cannot download $base/checksums.txt"

expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "checksums.txt has no entry for $asset"
[ "$(sha256_of "$tmp/$asset")" = "$expected" ] || fail "checksum mismatch for $asset, not installing"

tar -xzf "$tmp/$asset" -C "$tmp" managents

# The binary is replaced by renaming a new file over it, never by overwriting in place: a
# running helper keeps its old file, and macOS kills a signed binary that is modified.
mkdir -p "$bin_dir"
staged=$(mktemp "$bin_dir/.managents.XXXXXX")
cp "$tmp/managents" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$bin_dir/managents"
staged=

managents=$bin_dir/managents
say "Installed $("$managents" version) to $managents"

if [ "${MANAGENTS_NO_SERVICE:-}" = 1 ]; then
	say "Skipped the service (MANAGENTS_NO_SERVICE=1). Run the helper with: managents run"
else
	"$managents" service install
	"$managents" service status
fi

case ":$PATH:" in
*":$bin_dir:"*) ;;
*) say "$bin_dir is not on your PATH. Add it: export PATH=\"$bin_dir:\$PATH\"" ;;
esac

say "Plug in your display. New board? run: managents flash"
