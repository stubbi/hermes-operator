# OpenClaw → Hermes Migration

The operator supports a one-shot migration from a sibling OpenClawInstance (or its S3 backup) into a new HermesInstance. The migration is driven by an init container that runs `hermes-agent migrate from-openclaw` against the source.

> **Verify the upstream CLI shape before relying on this guide.** Run
> `docker run --rm ghcr.io/nousresearch/hermes-agent:latest hermes-agent migrate from-openclaw --help`
> to confirm. If the args differ, update `internal/resources/migration_init.go` accordingly.

## Two source modes (mutually exclusive)

### A. In-cluster ref

```yaml
spec:
  migration:
    fromOpenClaw:
      mode: copy                          # or "move"
      source:
        openclawInstanceRef:
          name: my-openclaw
          namespace: agents
```

The operator mounts `my-openclaw-data` (the OpenClaw PVC, by OpenClaw's deterministic name convention) read-only at `/mnt/openclaw` in the migration init container. The hermes-agent CLI reads from there and writes to `/home/hermes/.hermes`.

The operator's ServiceAccount is granted `get;list;watch` on `openclawinstances.openclaw.rocks` and `get` on PVCs so the read-only mount works across CRD groups.

### B. S3 backup

```yaml
spec:
  migration:
    fromOpenClaw:
      mode: copy
      source:
        backupRef:
          s3:
            bucket: openclaw-backups
            endpoint: s3.amazonaws.com
            region: us-east-1
            key: prod/my-openclaw/2026-05-11.tar.zst
            credentialsSecretRef:
              name: oc-s3-creds
```

The init container downloads + extracts the snapshot to an `emptyDir` mounted at `/mnt/openclaw`, then runs the importer.

The Secret must contain `S3_ACCESS_KEY_ID` and `S3_SECRET_ACCESS_KEY`.

## Mode: `copy` vs `move`

| Mode | Behaviour |
|---|---|
| `copy` (default) | Source is untouched. |
| `move` | After successful migration, the operator emits a `Warning` event recommending `kubectl delete openclawinstance <name>`. **The operator does NOT delete the source automatically.** Cross-CRD-group deletion is too dangerous to do silently. |

## Status

```yaml
status:
  migration:
    completed: true
    finishedAt: "2026-05-12T14:33:21Z"
    sourceVersion: "openclaw-v0.32.1"
  conditions:
    - type: MigrationCompleted
      status: "True"
      reason: MigrationCompleted
      message: "OpenClaw -> Hermes migration completed at 2026-05-12T14:33:21Z"
```

## Immutability

`spec.migration.fromOpenClaw` is **immutable** once `status.migration.completed = true`. The validating webhook rejects updates. This is intentional — migration is one-shot. To re-migrate, delete the HermesInstance and re-create.

## Restore + Migrate are mutually exclusive

You cannot set both `spec.restoreFrom` AND `spec.migration.fromOpenClaw` on the same instance. The validator rejects the combination. Reason: the combined order of operations is ambiguous (which source wins for overlapping files?). To both restore and migrate, do them as two separate instances and join the data manually.

## Common pitfalls

| Symptom | Cause | Fix |
|---|---|---|
| Init container exits 1 with "source not found". | OpenClaw PVC name doesn't match `<name>-data`. | Older openclaw versions used different PVC names; check `kubectl get pvc -n <ns>` and either rename or use S3 mode. |
| Permission denied reading source PVC. | RBAC missing for `pvc/get` in the source namespace. | Update the operator's RoleBinding to include the source namespace (Helm value `watchNamespaces` doesn't grant this — it's a separate scope). |
| Migration appears to succeed but `~/.hermes` is empty. | The upstream importer's CLI flag changed between versions. | Verify the CLI shape (see the warning at the top of this doc) and adjust the init-container args. |

## End-to-end test

A build-tagged e2e at `test/e2e/migration_test.go` exercises the full path. It requires a sibling OpenClawInstance and is skipped by default. Run with:

```bash
go test -tags=migration ./test/e2e/...
```
