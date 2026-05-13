# Honcho profile store

Hermes-agent uses [Honcho](https://github.com/plastic-labs/honcho) to keep
dialectic user profiles per chat partner. This example enables the optional
Honcho companion managed by the operator: a Deployment + Service + PVC +
Secret colocated with the `HermesInstance` it serves.

## Prerequisites

```bash
kubectl create namespace agents

kubectl create secret generic hermes-honcho \
  -n agents --from-literal=apiKey=$(openssl rand -hex 32)
```

## Apply

```bash
kubectl apply -n agents -f hermesinstance.yaml
```

## Verify

```bash
kubectl get deploy,svc,pvc -n agents \
  -l app.kubernetes.io/instance=honcho,app.kubernetes.io/component=honcho
# NAME                          READY   UP-TO-DATE   AVAILABLE   AGE
# deployment.apps/honcho-honcho   1/1     1            1           45s
# NAME                       TYPE        CLUSTER-IP      PORT(S)
# service/honcho-honcho      ClusterIP   10.96.123.45   8000/TCP
# NAME                                STATUS   VOLUME    CAPACITY
# persistentvolumeclaim/honcho-honcho   Bound    pvc-...   5Gi

kubectl get hi honcho -n agents -o jsonpath='{.status.conditions[?(@.type=="ProfileStoreReady")]}'
# { "type":"ProfileStoreReady", "status":"True", "reason":"HonchoReady", ...}
```

The agent reads `HONCHO_API_KEY` from the configured Secret automatically.

## Persistence

The Honcho PVC is separate from the hermes-agent PVC. It survives operator
upgrades, instance edits, and pod restarts. It is deleted when you delete
the `HermesInstance` (via owner-ref garbage collection), so back it up
externally if you need to retain profiles across instances.
