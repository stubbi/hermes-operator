# Snapshot Format

Every hermes-operator snapshot is a single file in a [restic](https://restic.net) repository at the root of the configured bucket: a `tar` of the agent data directory, streamed in with `restic backup --stdin --stdin-filename <key>`. restic handles deduplication, compression and encryption.

The current format is **2**.

## Layout (inside the tar)

```
./                        # everything on the agent data PVC
./skills/
./profiles/
./config.yaml
./db/...                  # FTS5 session memory
```

## Metadata

Metadata is stored on the restic snapshot, not inside the archive:

| Field | Where | Meaning |
|---|---|---|
| key | snapshot path | `<pathPrefix><namespace>/<name>/<timestamp>.tar`. This is the value for `spec.restoreFrom`. |
| instance | `host` | `<pathPrefix><namespace>/<name>`. Scopes retention to one instance. |
| kind | tag | `scheduled`, `onDelete`, `preUpdate` or `failed`. |
| `hermes-format=2` | tag | The snapshot format version. |
| `instance-uid=<uid>` | tag | The HermesInstance's `metadata.uid` at backup time. |
| `timestamp=<ts>` | tag | RFC 3339 with `:` and `.` replaced by `-`. |

## Compression

restic compresses with zstd (repository format 2, `--compression auto`). No compressor is needed in the backup image.

## Encryption

restic always encrypts the repository with a password. Set `RESTIC_PASSWORD` in the credentials Secret; without it the operator falls back to a fixed default password that is public, so confidentiality then rests on bucket access control. Bucket side encryption (SSE-S3, SSE-KMS) can be layered on top.

## Cross-instance restore

To restore one instance's snapshot into another:

```yaml
spec:
  restoreFrom: "<source-snapshot-key>"
```

Both instances must use the same bucket and the same `RESTIC_PASSWORD`.

## Format history

| Version | Key suffix | Contents |
|---|---|---|
| 1 | `.tar.zst` | zstd compressed tar plus a `meta.json` sidecar. Required GNU tar and `zstd` in the backup image, so it never ran in the default `restic/restic` image. |
| 2 | `.tar` | Plain tar compressed by restic, metadata in restic tags. Runs in the default image. |

Format 1 snapshots remain restorable: the restore container pipes `.tar.zst` keys through `zstd -d`, which requires `spec.backup.image` to contain `zstd`.
