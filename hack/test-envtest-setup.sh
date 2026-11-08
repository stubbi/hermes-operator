#!/usr/bin/env bash
# Exercise the make test gate without installing tools or starting a cluster.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/setup-envtest" <<'SCRIPT'
#!/bin/sh
printf 'setup\n' >> "$EVENTS"
case "$CASE" in
  failure) if [ ! -e "$EVENTS.first" ]; then : > "$EVENTS.first"; exit 1; fi; printf '%s\n' /fake/envtest/assets ;;
  empty) exit 0 ;;
  success) printf '%s\n' /fake/envtest/assets ;;
esac
SCRIPT
cat > "$tmp/go" <<'SCRIPT'
#!/bin/sh
printf 'go:%s:%s\n' "$1" "${KUBEBUILDER_ASSETS-}" >> "$EVENTS"
case "$1" in list) echo example/package ;; test) test "$KUBEBUILDER_ASSETS" = /fake/envtest/assets ;; esac
SCRIPT
chmod +x "$tmp/setup-envtest" "$tmp/go"
for scenario in failure empty success; do
  export CASE="$scenario" EVENTS="$tmp/$scenario.events"
  : > "$EVENTS"
  if PATH="$tmp:$PATH" make -s -C "$repo" -o manifests -o generate -o fmt -o vet -o envtest test ENVTEST="$tmp/setup-envtest" > "$tmp/$scenario.log" 2>&1; then status=0; else status=$?; fi
  case "$scenario" in
    failure|empty)
      if [ "$status" -eq 0 ] || grep -Eq '^go:(test|list):' "$EVENTS"; then
        echo "$scenario: setup must fail before any go command starts" >&2; cat "$tmp/$scenario.log" >&2; exit 1
      fi ;;
    success)
      if [ "$status" -ne 0 ] || [ "$(grep -c '^setup$' "$EVENTS")" -ne 1 ] || ! grep -q '^go:test:/fake/envtest/assets$' "$EVENTS"; then
        echo "success: expected one setup and go test with the assets path" >&2; cat "$tmp/$scenario.log" >&2; cat "$EVENTS" >&2; exit 1
      fi ;;
  esac
done
echo 'envtest setup gate: all cases passed'
