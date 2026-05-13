# FluxCD + `HermesSelfConfig` coexistence

A `HermesInstance` reconciled by FluxCD (server-side apply, field manager
`kustomize-controller`) plus a hermes-agent that creates `HermesSelfConfig`
resources to mutate allowed fields (field manager `hermes.agent/selfconfig`).

The point of this example is to demonstrate that:

1. FluxCD does not undo the agent's mutations on its next sync interval.
2. The agent does not undo FluxCD's spec changes when it next reconciles
   its `HermesSelfConfig`.
3. Both can simultaneously change the same `HermesInstance` without flap.

This works because both writers use Server-Side Apply with disjoint field
managers, and the operator's SelfConfig controller writes only the allowed
fields (others are protected via `spec.selfConfigure.protectedKeys`).

## Prerequisites

You need a kind cluster with FluxCD installed and the
`stubbi/hermes-operator` chart deployed. Assume a Git repository
`github.com/example/agents-gitops` containing the manifests in this
directory.

```bash
flux install
flux create source git agents-gitops \
  --url=https://github.com/example/agents-gitops \
  --branch=main \
  --interval=1m
flux create kustomization agents-gitops \
  --source=GitRepository/agents-gitops \
  --path=./examples/gitops-fluxcd \
  --prune=true \
  --interval=1m
```

## What FluxCD owns

Field manager `kustomize-controller`:

- `spec.image.*`
- `spec.security.*`
- `spec.storage.*`
- `spec.backup.*`
- `spec.networking.*`
- `spec.gateways.*` (token secret refs change here; new gateway is
  added here)
- Everything under `metadata.labels` and `metadata.annotations` that
  Flux owns.

## What the agent owns

Field manager `hermes.agent/selfconfig`:

- `spec.config.raw.schedules`
- `spec.runtime.extraPip`
- `spec.workspace.files` entries under `notes/` and `learned/`
- `spec.env` entries for `FINANCE_TZ`, `LEARNED_*`

These match `spec.selfConfigure.allowedActions` and the **complement** of
`spec.selfConfigure.protectedKeys` in the manifest below.

## Verifying no flap

```bash
# 1. Initial apply via Flux.
flux reconcile kustomization agents-gitops

# 2. Have the agent create a HermesSelfConfig that touches an allowed field.
kubectl apply -n agents -f - <<'YAML'
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: learn-schedule
spec:
  instanceRef: gitops-hermes
  patchConfig:
    schedules:
      morning-brief: "0 8 * * *"
YAML

# 3. Force a Flux re-sync. The schedules entry must survive.
flux reconcile kustomization agents-gitops --with-source

# 4. Inspect managed fields.
kubectl get hi gitops-hermes -n agents -o jsonpath='{.metadata.managedFields}' | jq
# Expect two distinct manager entries:
#   { "manager": "kustomize-controller",  "operation": "Apply", ... }
#   { "manager": "hermes.agent/selfconfig","operation": "Apply", ... }
```

If a flap occurred, Flux would report a drift on its next interval and the
`schedules.morning-brief` entry would disappear. With SSA + disjoint field
managers, it does not.
