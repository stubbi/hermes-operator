# `HermesClusterDefaults`: cluster-wide defaults

The defaulting webhook fills `nil` fields on every `HermesInstance` from
the cluster-scoped singleton named `cluster`. Explicit values on the
instance always win: `HermesClusterDefaults` never overrides.

Use this to:

- Centralise the operator's image repository + tag for hermes-agent.
- Mandate IRSA / Workload Identity annotations on every ServiceAccount.
- Mandate a default StorageClass + size for the PVC.
- Mandate observability (`serviceMonitor.enabled=true`) and networking
  (`networkPolicy.enabled=true`) without having to repeat them on every
  instance.

The CR name **must** be `cluster`. The validating webhook rejects any
other name with `WrongName` reason.

## Apply

```bash
kubectl apply -f clusterdefaults.yaml
```

## Verify

```bash
kubectl get hcd cluster -o jsonpath='{.status.conditions[?(@.type=="Active")]}'
# { "status":"True", "reason":"Applied", ... }

# Apply a minimal HermesInstance that omits image, storage, networking:
# they will all be filled by the defaults.
kubectl create namespace agents
kubectl apply -n agents -f - <<'YAML'
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: defaulted
spec:
  config:
    raw: |
      logging:
        level: info
YAML

kubectl get hi defaulted -n agents -o jsonpath='{.spec.image}'
# {"repository":"ghcr.io/stubbi/hermes-agent","tag":"1.4.2"}
```

## What this defaults

| Spec path | Default |
|---|---|
| `spec.image.repository` | `ghcr.io/stubbi/hermes-agent` |
| `spec.image.tag` | `1.4.2` |
| `spec.image.imagePullSecrets[]` | `[{name: ghcr-pull}]` |
| `spec.storage.persistence.storageClassName` | `gp3` |
| `spec.storage.persistence.size` | `10Gi` |
| `spec.security.serviceAccount.annotations` | `{eks.amazonaws.com/role-arn: arn:aws:iam::...}` |
| `spec.observability.serviceMonitor.enabled` | `true` |
| `spec.networking.networkPolicy.enabled` | `true` |

## Important: ordering

The defaulter runs once per admission, *before* the validator. Defaults
filled from `HermesClusterDefaults` are persisted to etcd as part of the
admitted object. Editing the singleton later does not retroactively
re-default existing objects: only new admissions pick up the new
defaults. This is intentional and matches how `LimitRange` works.

To force-resync, re-apply the affected instances with `kubectl replace`.

## Removing the singleton

```bash
kubectl delete hcd cluster
```

Existing `HermesInstance` resources are unaffected (their fields are
already filled). New ones fall back to the operator's built-in fallback
defaults (`ghcr.io/stubbi/hermes-agent:latest`, 10Gi default StorageClass,
no SA annotations).
