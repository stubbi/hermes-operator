# OCI-registry auto-update with rollback

Auto-update polls the OCI registry on a `pollInterval`, picks the highest
tag in the channel (here, `1.x`: anything `1.*.*`), takes a pre-update
backup, and rolls the StatefulSet forward. If the new image fails
readiness probes more than `probeFailureThreshold` times within the
deadline, the operator rolls back automatically and records the failed
tag in `status.autoUpdate.lastFailedTag` to suppress retries.

## Prerequisites

```bash
kubectl create namespace agents

kubectl create secret generic hermes-s3-creds \
  -n agents \
  --from-literal=accessKey=REPLACE \
  --from-literal=secretKey=REPLACE
```

A reachable S3 bucket is required because `backupBeforeUpdate: true`
(default) takes a pre-update snapshot. See
[`backup-s3/`](../backup-s3/) for a kind-local MinIO setup if you need
one.

## Apply

```bash
kubectl apply -n agents -f hermesinstance.yaml
```

## Watching it roll forward

```bash
kubectl get hi auto-update -n agents -w
# NAME           READY   PHASE          IMAGE                                 AGE
# auto-update    True    Ready          ghcr.io/stubbi/hermes-agent:1.4.2     2m
# auto-update    False   Rolling        ghcr.io/stubbi/hermes-agent:1.4.2     3h
# auto-update    True    Ready          ghcr.io/stubbi/hermes-agent:1.4.3     3h1m

kubectl get hi auto-update -n agents \
  -o jsonpath='{.status.autoUpdate}' | jq
# {
#   "currentTag":         "1.4.3",
#   "lastConfirmedTag":   "1.4.3",
#   "lastCheckedAt":      "2026-05-12T13:00:00Z",
#   "targetTag":          "",
#   "lastFailedTag":      ""
# }
```

## Forcing a rollback test

To verify the rollback path without a real failure, point the channel at a
tag that you know will not become ready (for example, a tag whose entry
point exits immediately):

```bash
kubectl patch hi auto-update -n agents --type=merge -p '{
  "spec": {
    "autoUpdate": {
      "source": {
        "registry": "ghcr.io/stubbi/hermes-agent",
        "channel":  "broken-1.x"
      }
    }
  }
}'
```

Within `pollInterval * 2`, the operator rolls forward to
`broken-1.x.latest`, sees the probe failures, rolls back, and surfaces
`AutoUpdateRolledBack=True` (reason `RolledBackFrom_<tag>`).

## Clearing a suppressed failure

If `lastFailedTag` is set and you have manually fixed the upstream image
problem, clear the suppression with a status subresource patch:

```bash
kubectl patch hi auto-update -n agents \
  --subresource=status --type=json \
  -p='[{"op":"remove","path":"/status/autoUpdate/lastFailedTag"}]'
```

The next poll will retry the previously-failed tag.
