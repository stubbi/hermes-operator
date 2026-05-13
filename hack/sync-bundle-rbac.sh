#!/usr/bin/env bash
set -euo pipefail

# Extract rules from the kubebuilder-generated ClusterRole and splice them
# into the bundle CSV's clusterPermissions[0].rules block. Idempotent.

CSV=bundle/manifests/hermes-operator.clusterserviceversion.yaml
ROLE=config/rbac/role.yaml

if [ ! -f "$ROLE" ]; then
  echo "::error::$ROLE not found. Run 'make manifests' first." >&2
  exit 1
fi

# yq merges: replace the rules array on the first clusterPermissions entry.
TMP=$(mktemp)
yq eval \
  '.spec.install.spec.clusterPermissions[0].rules = load("'"$ROLE"'").rules' \
  "$CSV" > "$TMP"
mv "$TMP" "$CSV"

# In --check mode (CI), bail if the working tree is dirty after sync.
if [ "${1:-}" = "--check" ]; then
  if ! git diff --exit-code -- "$CSV"; then
    echo "::error::Bundle CSV RBAC drifted from $ROLE. Run 'make sync-bundle-rbac' locally." >&2
    exit 1
  fi
fi

echo "Bundle CSV RBAC synced from $ROLE."
