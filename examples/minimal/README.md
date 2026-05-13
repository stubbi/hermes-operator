# Minimal `HermesInstance`

Smallest possible instance: an image, a 10Gi PVC, and the operator's
default-deny NetworkPolicy. No gateways enabled, no Honcho, no backup,
no auto-update. Useful as a smoke test.

## Apply

```bash
kubectl create namespace agents
kubectl apply -n agents -f hermesinstance.yaml
```

## Verify

```bash
kubectl get hi -n agents
# NAME        READY   PHASE   IMAGE                                AGE
# minimal     True    Ready   ghcr.io/stubbi/hermes-agent:1.4.2    30s

kubectl describe hi minimal -n agents | grep -A1 "Ready"
# Type:   Ready
# Status: True
```

## What you get

- `StatefulSet/minimal` with one replica running hermes-agent.
- `Service/minimal` (ClusterIP).
- `PersistentVolumeClaim/minimal-data` (10Gi, default StorageClass).
- `NetworkPolicy/minimal-network` (default-deny ingress and egress except
  DNS).
- `ConfigMap/minimal-config` with an empty config (the agent uses
  built-in defaults).
- `ServiceAccount/minimal` for the pod.

## Tear down

```bash
kubectl delete hi minimal -n agents
```

The finalizer for backup-on-delete is **not** active (no backup configured),
so deletion completes immediately. The PVC's `persistentVolumeReclaimPolicy`
is whatever the default StorageClass dictates.
