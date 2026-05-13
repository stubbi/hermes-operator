#!/usr/bin/env bash
set -euo pipefail

# Render the chart, extract the verbs, compare against the kubebuilder-generated role.
generated=$(yq '.rules' config/rbac/role.yaml | yq -s 'sort_by(.apiGroups, .resources)')
rendered=$(helm template hermes-operator charts/hermes-operator | yq 'select(.kind=="ClusterRole") | .rules' | yq -s 'sort_by(.apiGroups, .resources)')

if [ "$generated" != "$rendered" ]; then
    echo "::error::Helm chart ClusterRole drifted from kubebuilder-generated role." >&2
    diff <(echo "$generated") <(echo "$rendered") >&2 || true
    exit 1
fi
