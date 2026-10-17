#!/bin/sh
# Build the Claude Sessions Roku channel, and optionally sideload it.
#
#   ./build.sh http://192.168.1.20:8787 "$MANAGENTS_TOKEN"
#
# Writes config.json and dist/claude-sessions.zip. If ROKU_IP and ROKU_PASS
# (the developer-mode password) are set, it also installs the zip on the Roku.
set -eu
cd "$(dirname "$0")"

url="${1:-}"
token="${2:-}"
case "$url" in
  http://*) ;;
  *) echo "usage: $0 http://YOUR_COMPUTER_IP:8787 [TOKEN]" >&2; exit 2 ;;
esac
url="${url%/}"
if printf '%s' "$url$token" | grep -q '["\\[:space:]]'; then
  echo "url and token must not contain quotes, backslashes or spaces" >&2
  exit 2
fi

printf '{"url": "%s", "token": "%s"}\n' "$url" "$token" > config.json
mkdir -p dist
rm -f dist/claude-sessions.zip
zip -qr dist/claude-sessions.zip manifest config.json source components images
echo "built dist/claude-sessions.zip"

if [ -n "${ROKU_IP:-}" ] && [ -n "${ROKU_PASS:-}" ]; then
  echo "installing on $ROKU_IP..."
  curl -sS --fail --digest --user "rokudev:$ROKU_PASS" \
    -F "mysubmit=Install" -F "archive=@dist/claude-sessions.zip" \
    "http://$ROKU_IP/plugin_install" > /dev/null
  echo "installed. It should be open on your TV now."
fi
