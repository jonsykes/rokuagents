#!/bin/sh
# Build rokuagents.app: the menu bar app, with the managents helper and the Roku
# channel inside it.
#
#   ./build.sh            builds dist/rokuagents.app
#   ./build.sh --install  also copies it to ~/Applications and opens it
#
# Needs Go 1.26+ and Xcode (or its command-line tools) with Swift 6.
set -eu
cd "$(dirname "$0")"

version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
short=$(echo "$version" | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')
short=${short:-0.0.0}
app=dist/rokuagents.app
contents=$app/Contents

echo "building the helper..."
mkdir -p .build
(cd ../helper && go build -trimpath -ldflags "-s -w" -o ../macos/.build/managents ./cmd/managents)

echo "building the app..."
swift build -c release --product RokuAgents
bin=$(swift build -c release --show-bin-path)

rm -rf "$app"
mkdir -p "$contents/MacOS" "$contents/Resources/roku"
cp "$bin/RokuAgents" "$contents/MacOS/RokuAgents"
cp .build/managents "$contents/MacOS/managents"
sed -e "s/SHORT_VERSION/$short/" -e "s/BUILD_VERSION/$version/" Info.plist > "$contents/Info.plist"
for item in manifest source components images; do
  cp -R "../roku/$item" "$contents/Resources/roku/"
done

# Ad-hoc signatures: enough to run on this Mac. The helper is signed first,
# since the app's signature covers it.
codesign --force --options runtime --sign - "$contents/MacOS/managents"
codesign --force --options runtime --sign - "$app"
codesign --verify --strict "$app"
echo "built $app ($version)"

if [ "${1:-}" = "--install" ]; then
  mkdir -p "$HOME/Applications"
  osascript -e 'quit app "rokuagents"' >/dev/null 2>&1 || true
  rm -rf "$HOME/Applications/rokuagents.app"
  cp -R "$app" "$HOME/Applications/"
  open "$HOME/Applications/rokuagents.app"
  echo "installed ~/Applications/rokuagents.app"
fi
