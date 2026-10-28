# Backup & Restore

The `hermes-operator` ships an S3-compatible backup subsystem with three trigger paths:

1. **Scheduled**: via `spec.backup.schedule` (cron expression).
2. **On delete**: via `spec.backup.onDelete = true` (`hermes.agent/backup-on-delete` finalizer).
3. **Pre-update**: automatic when `spec.autoUpdate.backupBeforeUpdate = true` (default).

All three paths stream a `tar` of the agent data directory into a [restic](https://restic.net) repository at the root of the bucket, under a deterministic snapshot key. restic deduplicates, compresses and encrypts the data. The format is documented in [`docs/backup-format.md`](backup-format.md).

The backup, prune and restore containers run in the default `restic/restic` image (Alpine, busybox `tar`) with a read-only root filesystem and an `emptyDir` at `/tmp` for restic's scratch files. No GNU tar, `zstd` or `jq` is required.

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
| `S3_ACCESS_KEY_ID` | Access key (passed to restic as `AWS_ACCESS_KEY_ID`) |
| `S3_SECRET_ACCESS_KEY` | Secret key (passed to restic as `AWS_SECRET_ACCESS_KEY`) |
| `RESTIC_PASSWORD` | Recommended. Password of the restic repository. |

Every key of the Secret is also injected into the backup containers, so other restic settings (for example `RESTIC_PASSWORD_FILE` or `AWS_SESSION_TOKEN`) pass straight through.

The repository is created (`restic init`) by the first backup. restic always encrypts with a repository password. **If the Secret has no `RESTIC_PASSWORD`, the operator uses a fixed default password that is public in this repository**, logs a warning, and confidentiality then rests on bucket access control alone. Set `RESTIC_PASSWORD` before the first backup and keep it safe: without it the snapshots cannot be restored. To switch an existing repository from the default password, add a key with `restic key add` and then set `RESTIC_PASSWORD`.

For a plain HTTP endpoint (for example an in-cluster S3 server), put the scheme into `endpoint` (`http://minio.minio.svc:9000`); without a scheme restic uses HTTPS.
## Snapshot keys

```
<pathPrefix><namespace>/<instance-name>/<timestamp>.tar             (success)
<pathPrefix><namespace>/<instance-name>/failed/<timestamp>.tar       (failed)
```

The key is the file name of the snapshot inside the restic repository (`restic backup --stdin-filename`), not a raw S3 object. `<timestamp>` is RFC 3339 with colons and dots replaced by `-` for filesystem safety: `2026-05-10T03-00-00Z`. Each snapshot also carries the restic host `<pathPrefix><namespace>/<instance-name>` and the tags `<kind>` (`scheduled`, `onDelete`, `preUpdate`), `hermes-format=2`, `instance-uid=<uid>` and `timestamp=<timestamp>`.

List the snapshots of an instance with:

```bash
restic -r s3:<endpoint>/<bucket> snapshots --host <pathPrefix><namespace>/<instance-name>
```

Format 1 snapshots (`.tar.zst`, written by custom backup images before this format) stay restorable when `spec.backup.image` contains `zstd`.

## Manual restore

```yaml
spec:
  restoreFrom: "prod/agents/my-hermes/2026-05-10T03-00-00Z.tar"
```

On the next reconcile, the operator injects an `init-restore` init container into the StatefulSet PodTemplate. It runs `restic dump` for the snapshot with that key and extracts it onto the PVC. When the init container exits 0, `status.restoredFrom` is latched. The field becomes immutable thereafter: see [API stability](#api-stability).

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
3. When the Job succeeds, the finalizer is removed via **`r.Patch`** (`client.MergeFrom`), not `r.Update`. This is critical: `r.Update` bumps `metadata.generation` and replaces the pod on the next reconcile. Lesson #437 from openclaw-operator.
4. Kubernetes GC'es the CR + cascades to owned resources.

### Bounded grace window

A failing or stuck final backup must not make the instance **permanently undeletable** (#93). The operator blocks deletion for at most a grace window (`finalBackupDeadline`, **30 minutes** from `deletionTimestamp`); after that it gives up: it records `status.backup.lastFailureReason = FinalBackupDeadlineExceeded`, emits a `Warning FinalBackupAbandoned` event, and **releases the finalizer so deletion proceeds**. The final snapshot will not have been taken, so PVC data may be lost; the event makes this visible for post-mortems.

### Skipping the final backup

To delete immediately (without waiting out the grace window) when the final backup is hanging or the bucket is unreachable:

```bash
kubectl annotate hermesinstance/<name> hermes.agent/skip-final-backup=true --overwrite
```

A `Warning` event is recorded so post-mortem reviewers see it. **Use this only if you accept data loss.**

## History pruning

A second CronJob (`<name>-backup-prune`) runs daily at 04:17 UTC and runs `restic forget --prune --keep-last <historyLimit>` on the instance's `scheduled`, `onDelete` and `preUpdate` snapshots, and `--keep-last <failedHistoryLimit>` on its `failed` snapshots. Retention is scoped to the instance by the restic host, so instances sharing a bucket never prune each other's snapshots.

## Common pitfalls

| Symptom | Cause | Fix |
|---|---|---|
| Final backup Job fails with `S3 credentials secret missing key`. | Secret missing `S3_ACCESS_KEY_ID` or `S3_SECRET_ACCESS_KEY`. | Patch the Secret. The CR stays in deletion until the next reconcile picks up the new Secret. |
| Scheduled CronJob runs but no snapshot appears. | Likely a network policy blocking egress to S3 endpoint. | Add an egress rule under `spec.networking.egress`. |
| `kubectl delete` blocks (up to the 30-minute grace window, then auto-releases). | Final backup Job failing repeatedly. | `kubectl describe job <name>-backup-final` for logs; fix the cause, or use the skip annotation to release immediately. After the grace window the finalizer auto-releases (`FinalBackupAbandoned` event) and deletion proceeds without a final snapshot. |
| `status.restoredFrom` stays empty after `init-restore` exited 0. | Pod restarted before the operator observed the terminated state. | Force reconcile: `kubectl annotate hermesinstance <name> poke=$(date +%s) --overwrite`. |

## API stability

`spec.restoreFrom` is immutable after `status.restoredFrom == spec.restoreFrom`. The validating webhook rejects updates that change the field once latched. This prevents accidental re-restore on pod restart (where users sometimes "fix" by re-applying the manifest from Git).
