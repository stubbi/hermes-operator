# Backup & Restore

The `hermes-operator` ships an S3-compatible backup subsystem with three trigger paths:

1. **Scheduled** — via `spec.backup.schedule` (cron expression).
2. **On delete** — via `spec.backup.onDelete = true` (`hermes.agent/backup-on-delete` finalizer).
3. **Pre-update** — automatic when `spec.autoUpdate.backupBeforeUpdate = true` (default).

All three paths produce a `tar.zst` snapshot of `/home/hermes/.hermes/` plus a `meta.json` sidecar, written to S3 under a deterministic key. The format is documented in [`docs/backup-format.md`](backup-format.md).

## Configuration

```yaml
spec:
  backup:
    s3:
      bucket: hermes-backups
      endpoint: s3.amazonaws.com         # any S3-compatible: R2, B2, MinIO
      region: us-east-1
      pathPrefix: prod/
      credentialsSecretRef:
        name: hermes-s3-creds            # Secret with S3_ACCESS_KEY_ID + S3_SECRET_ACCESS_KEY
    schedule: "0 3 * * *"                # optional
    onDelete: true                       # optional
    historyLimit: 30                     # successful snapshots to retain
    failedHistoryLimit: 3                # failed snapshots to retain under failed/
```

The Secret must live in the same namespace as the `HermesInstance` and contain:

| Key | Value |
|---|---|
| `S3_ACCESS_KEY_ID` | Access key |
| `S3_SECRET_ACCESS_KEY` | Secret key |

## Snapshot keys

```
<pathPrefix><namespace>/<instance-name>/<timestamp>.tar.zst             (success)
<pathPrefix><namespace>/<instance-name>/failed/<timestamp>.tar.zst       (failed)
```

`<timestamp>` is RFC 3339 with colons and dots replaced by `-` for filesystem safety: `2026-05-10T03-00-00Z`.

## Manual restore

```yaml
spec:
  restoreFrom: "prod/agents/my-hermes/2026-05-10T03-00-00Z.tar.zst"
```

On the next reconcile, the operator injects an `init-restore` init container into the StatefulSet PodTemplate. It downloads + extracts the snapshot to the PVC at `/home/hermes/.hermes/`. When the init container exits 0, `status.restoredFrom` is latched. The field becomes immutable thereafter — see [API stability](#api-stability).

### Empty-PVC guard

`init-restore` refuses to overwrite a non-empty destination. To override (only for disaster-recovery, after manually wiping the PVC):

```bash
kubectl set env statefulset/<name> -c init-restore HERMES_RESTORE_FORCE=1
```

The operator removes this env var on the next reconcile, so it is a one-shot override.

## Disaster recovery walkthrough

1. **Lose a node.** Pod will reschedule (StatefulSet replicas=1, PVC is RWO, so a node-affinity volume binding may pin reschedule). If the PV is gone, you need a new PVC.
2. **Create a fresh HermesInstance** with the same name but `spec.restoreFrom = <key>`. The init container will restore into the new PVC.
3. **Verify status:**
   ```bash
   kubectl get hermesinstance my-hermes -o yaml | yq '.status.restoredFrom'
   ```
4. **Once latched, `spec.restoreFrom` is immutable.** If you need to re-restore, delete the instance, delete the PVC, and recreate.

## On-delete finalizer

When `spec.backup.onDelete = true`, the operator adds the `hermes.agent/backup-on-delete` finalizer. On `kubectl delete`:

1. The CR enters `DeletionTimestamp != nil` but is not GC'd.
2. The operator creates a one-shot `<name>-backup-final` Job.
3. When the Job succeeds, the finalizer is removed via **`r.Patch`** (`client.MergeFrom`), not `r.Update`. This is critical — `r.Update` bumps `metadata.generation` and replaces the pod on the next reconcile. Lesson #437 from openclaw-operator.
4. Kubernetes GC'es the CR + cascades to owned resources.

### Skipping the final backup

If the final backup is hanging or the bucket is unreachable:

```bash
kubectl annotate hermesinstance/<name> hermes.agent/skip-final-backup=true --overwrite
```

A `Warning` event is recorded so post-mortem reviewers see it. **Use this only if you accept data loss.**

## History pruning

A second CronJob (`<name>-backup-prune`) runs daily at 04:17 UTC and runs `restic forget --keep-last <historyLimit>` on the successful snapshot tags, and `--keep-last <failedHistoryLimit>` on the `failed/` prefix.

## Common pitfalls

| Symptom | Cause | Fix |
|---|---|---|
| Final backup Job fails with `S3 credentials secret missing key`. | Secret missing `S3_ACCESS_KEY_ID` or `S3_SECRET_ACCESS_KEY`. | Patch the Secret. The CR stays in deletion until the next reconcile picks up the new Secret. |
| Scheduled CronJob runs but no snapshot appears. | Likely a network policy blocking egress to S3 endpoint. | Add an egress rule under `spec.networking.egress`. |
| `kubectl delete` hangs forever. | Final backup Job failing repeatedly. | `kubectl describe job <name>-backup-final` for logs; either fix or use the skip annotation. |
| `status.restoredFrom` stays empty after `init-restore` exited 0. | Pod restarted before the operator observed the terminated state. | Force reconcile: `kubectl annotate hermesinstance <name> poke=$(date +%s) --overwrite`. |

## API stability

`spec.restoreFrom` is immutable after `status.restoredFrom == spec.restoreFrom`. The validating webhook rejects updates that change the field once latched. This prevents accidental re-restore on pod restart (where users sometimes "fix" by re-applying the manifest from Git).
