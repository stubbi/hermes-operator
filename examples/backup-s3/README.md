# S3-compatible backups on kind

A fully self-contained recipe for trying out the backup pipeline without a
cloud account: a single-replica S3-compatible server (Versity S3 Gateway, the
upstream MinIO images are no longer publicly pullable) provides an endpoint
in-cluster at `minio:9000`, and a `HermesInstance` is configured to back up to it
every 10 minutes.

## Apply

```bash
kubectl create namespace agents

# 1. Apply the credentials (S3 keys + restic repository password).
kubectl apply -n agents -f s3-credentials.yaml

# 2. Stand up the S3 server + a bootstrap Job that creates the bucket.
kubectl apply -n agents -f minio.yaml
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

# List the snapshots in the restic repository.
kubectl run --rm -it --restart=Never -n agents \
  --env=AWS_ACCESS_KEY_ID=minio --env=AWS_SECRET_ACCESS_KEY=minio12345 \
  --env=RESTIC_PASSWORD=change-me-example-only \
  --image=restic/restic:0.16.4 restic-shell -- \
  -r s3:http://minio:9000/hermes-backups snapshots
```

## What you get

| Resource | What |
|---|---|
| `Deployment/minio` | S3-compatible server (Versity S3 Gateway), single replica, with PVC. |
| `Service/minio` | ClusterIP for the S3 endpoint at `minio:9000`. |
| `Job/minio-mkbucket` | Bootstrap Job that creates the `hermes-backups` bucket. |
| `Secret/hermes-s3-creds` | S3 credentials and the restic repository password. |
| `StatefulSet/backup-s3-*` | The hermes-agent pods. |
| `CronJob/backup-s3-backup` | Schedules backups every 10 minutes. |
| `Job/backup-s3-backup-*` | Per-run backup Job. |

## Tear down

```bash
# The finalizer hermes.agent/backup-on-delete will queue one final backup.
kubectl delete hi backup-s3 -n agents

# Then the S3 server and the credentials.
kubectl delete -n agents -f s3-credentials.yaml -f minio.yaml
```
