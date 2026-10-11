#!/usr/bin/env bash
set -euo pipefail

# Compare/sync the bundle CSV RBAC against the kubebuilder-generated roles.
#
# The generated ClusterRole is split by scope (#126):
#   clusterPermissions  cluster-scoped resources only (CRDs with scope: Cluster)
#   permissions         namespaced resources plus the leader election rules
#
# OLM grants `permissions` as Roles in each target namespace for OwnNamespace
# and SingleNamespace installs, and lifts them to a ClusterRole for
# AllNamespaces installs, so a namespaced install no longer carries a
# cluster-wide secrets grant. The manager follows the same scope through
# WATCH_NAMESPACE (olm.targetNamespaces).
#
# Comparison is semantic (both sides normalized through yq -P). The mutate
# mode rewrites only the RBAC block as text so the rest of the CSV keeps its
# formatting (a whole-file yq round trip reflows the folded descriptions).
#
#   sync-bundle-rbac.sh           write the generated rules into the CSV
#   sync-bundle-rbac.sh --check   exit non-zero if the rules don't match

CSV=bundle/manifests/hermes-operator.clusterserviceversion.yaml
ROLE=config/rbac/role.yaml
LEADER_ROLE=config/rbac/leader_election_role.yaml
SA=hermes-operator-controller-manager

for f in "$ROLE" "$LEADER_ROLE"; do
  if [ ! -f "$f" ]; then
    echo "::error::$f not found. Run 'make manifests' first." >&2
    exit 1
  fi
done

# Plural names of the cluster-scoped CRDs, comma separated.
CS=$(yq ea -N 'select(.spec.scope == "Cluster") | .spec.names.plural' config/crd/bases/*.yaml | paste -sd, -)
export CS

# A rule's resource is cluster-scoped when its base name (before any
# /subresource) is a cluster-scoped CRD plural.
SPLIT='(strenv(CS) | split(",")) as $cs
  | .rules
  | map(.resources |= map(select((split("/") | .[0]) as $b | $cs | any_c(. == $b) | SELECTOR)))
  | map(select(.resources | length > 0))'

GEN_CLUSTER=$(yq -P "${SPLIT//SELECTOR/.}" "$ROLE")
GEN_NS=$(yq -P "${SPLIT//SELECTOR/not}" "$ROLE")
GEN_LEADER=$(yq -P '.rules' "$LEADER_ROLE")
GEN_PERMS=$(printf '%s\n%s\n' "$GEN_NS" "$GEN_LEADER" | yq -P '.')

if [ "${1:-}" = "--check" ]; then
  status=0
  EMB_CLUSTER=$(yq -P '.spec.install.spec.clusterPermissions[0].rules' "$CSV")
  EMB_PERMS=$(yq -P '.spec.install.spec.permissions[0].rules' "$CSV")
  if [ "$GEN_CLUSTER" != "$EMB_CLUSTER" ]; then
    echo "::error::Bundle CSV clusterPermissions drifted from the cluster-scoped part of $ROLE. Run 'make sync-bundle-rbac' locally." >&2
    diff <(echo "$GEN_CLUSTER") <(echo "$EMB_CLUSTER") >&2 || true
    status=1
  fi
  if [ "$GEN_PERMS" != "$EMB_PERMS" ]; then
    echo "::error::Bundle CSV permissions drifted from the namespaced part of $ROLE plus $LEADER_ROLE. Run 'make sync-bundle-rbac' locally." >&2
    diff <(echo "$GEN_PERMS") <(echo "$EMB_PERMS") >&2 || true
    status=1
  fi
  # The manager must follow the OperatorGroup scope, otherwise a namespaced
  # install would try to watch all namespaces with only namespaced Roles.
  WATCH_REF=$(yq '.spec.install.spec.deployments[0].spec.template.spec.containers[] | select(.name == "manager") | .env[] | select(.name == "WATCH_NAMESPACE") | .valueFrom.fieldRef.fieldPath' "$CSV")
  if [ "$WATCH_REF" != "metadata.annotations['olm.targetNamespaces']" ]; then
    echo "::error::Bundle CSV manager container must set WATCH_NAMESPACE from metadata.annotations['olm.targetNamespaces']." >&2
    status=1
  fi
  [ "$status" -eq 0 ] && echo "Bundle CSV RBAC matches $ROLE and $LEADER_ROLE."
  exit "$status"
fi

indent() { sed 's/^/            /'; }
BLOCK=$(mktemp)
{
  echo "      clusterPermissions:"
  echo "        - serviceAccountName: $SA"
  echo "          rules:"
  echo "$GEN_CLUSTER" | indent
  echo "      permissions:"
  echo "        - serviceAccountName: $SA"
  echo "          rules:"
  echo "$GEN_PERMS" | indent
} > "$BLOCK"

# Replace everything from "clusterPermissions:" (or "permissions:") up to
# "deployments:" under .spec.install.spec with the generated block.
TMP=$(mktemp)
awk -v block="$BLOCK" '
  /^      (clusterPermissions|permissions):$/ && !done { while ((getline line < block) > 0) print line; skip = 1; done = 1; next }
  skip && /^      deployments:$/ { skip = 0 }
  !skip { print }
' "$CSV" > "$TMP"
mv "$TMP" "$CSV"
rm -f "$BLOCK"

echo "Bundle CSV RBAC synced from $ROLE and $LEADER_ROLE."
