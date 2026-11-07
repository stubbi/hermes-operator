#!/usr/bin/env bash
set -euo pipefail

# Render the chart in default and watchNamespaces modes and compare the
# manager RBAC against the kubebuilder-generated role. Needs helm, python3
# and PyYAML. See hack/check_helm_rbac.py for what is asserted.
cd "$(dirname "$0")/.."
exec python3 hack/check_helm_rbac.py
