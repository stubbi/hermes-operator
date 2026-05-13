# S3-compatible backups on kind (via MinIO)

A fully self-contained recipe for trying out the backup pipeline without a
cloud account: a single-replica MinIO deployment provides an S3-compatible
endpoint in-cluster, and a `HermesInstance` is configured to back up to it
every 10 minutes.

## Apply

```bash
kubectl create namespace agents

# 1. Stand up MinIO + its bucket.
kubectl apply -n agents -f minio.yaml

# 2. Apply credentials for MinIO + a bootstrap Job that creates the bucket.
kubectl apply -n agents -f s3-credentials.yaml
kubectl wait -n agents --for=condition=complete job/minio-mkbucket --timeout=60s

# 3. Apply the HermesInstance: it begins backing up immediately on first
#    reconcile, and again every 10 minutes per the schedule.
kubectl apply -n agents -f hermesinstance.yaml
```

## Verify

```bash
# Trigger a backup immediately (the on-create reconcile already did one,
# but you can run a manual one this way until kubectl-hermes ships).
kubectl annotate hi backup-s3 -n agents \
  hermes.agent/backup-now=$(date +%s) --overwrite

# Watch the backup Job appear.
kubectl get job -n agents -l app.kubernetes.io/instance=backup-s3 -w

# Inspect snapshots in MinIO.
kubectl run --rm -it --restart=Never -n agents \
  --image=minio/mc minio-shell -- /bin/sh
mc alias set local http://minio:9000 minio minio12345
mc ls local/hermes-backups/agents/backup-s3/
```

## What you get

| Resource | What |
|---|---|
| `Deployment/minio` | MinIO server, single replica, with PVC. |
| `Service/minio` | ClusterIP for the S3 endpoint at `minio:9000`. |
| `Job/minio-mkbucket` | Bootstrap Job that creates the `hermes-backups` bucket. |
| `Secret/hermes-s3-creds` | The credentials the operator uses. |
| `StatefulSet/backup-s3-*` | The hermes-agent pods. |
| `CronJob/backup-s3-backup` | Schedules backups every 10 minutes. |
| `Job/backup-s3-backup-*` | Per-run backup Job. |

## Tear down

```bash
# The finalizer hermes.agent/backup-on-delete will queue one final backup.
kubectl delete hi backup-s3 -n agents

# Then MinIO and the credentials.
kubectl delete -n agents -f s3-credentials.yaml -f minio.yaml
```
