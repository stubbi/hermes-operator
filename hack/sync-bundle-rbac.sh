#!/usr/bin/env bash
set -euo pipefail

# Compare/sync the rules in bundle/manifests/...clusterserviceversion.yaml
# against the kubebuilder-generated ClusterRole. Compares semantically (parses
# both sides through yq and compares the normalized rules array), so YAML
# style differences in the rest of the CSV don't produce false positives.
#
#   sync-bundle-rbac.sh           write the generated rules into the CSV
#   sync-bundle-rbac.sh --check   exit non-zero if the rules don't match

CSV=bundle/manifests/hermes-operator.clusterserviceversion.yaml
ROLE=config/rbac/role.yaml

if [ ! -f "$ROLE" ]; then
  echo "::error::$ROLE not found. Run 'make manifests' first." >&2
  exit 1
fi

# Normalize both sides through yq -P (pretty) so we compare canonical forms.
GENERATED=$(yq -P '.rules' "$ROLE")
EMBEDDED=$(yq -P '.spec.install.spec.clusterPermissions[0].rules' "$CSV")

if [ "${1:-}" = "--check" ]; then
  if [ "$GENERATED" != "$EMBEDDED" ]; then
    echo "::error::Bundle CSV RBAC drifted from $ROLE. Run 'make sync-bundle-rbac' locally." >&2
    diff <(echo "$GENERATED") <(echo "$EMBEDDED") >&2 || true
    exit 1
  fi
  echo "Bundle CSV RBAC matches $ROLE."
  exit 0
fi

# Mutate mode: replace the rules array in place.
TMP=$(mktemp)
yq eval \
  '.spec.install.spec.clusterPermissions[0].rules = load("'"$ROLE"'").rules' \
  "$CSV" > "$TMP"
mv "$TMP" "$CSV"

echo "Bundle CSV RBAC synced from $ROLE."
