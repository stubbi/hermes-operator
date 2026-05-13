# Snapshot Format

Every hermes-operator snapshot is a `tar.zst` archive of `/home/hermes/.hermes/` plus a `meta.json` sidecar. The format is stable across v1.x.

## Layout (inside the tar)

```
./                        # everything under /home/hermes/.hermes
./skills/
./profiles/
./config.yaml
./db/...                  # FTS5 session memory
meta.json                 # sidecar (not under ./)
```

## `meta.json`

```json
{
  "instance_uid": "9d3d8a7b-91a7-4c2e-8e3a-7c2e8b1d8a91",
  "hermes_agent_version": "1.4.2",
  "k8s_version": "1.32",
  "timestamp": "2026-05-10T03-00-00Z",
  "format_version": 1
}
```

| Field | Meaning |
|---|---|
| `instance_uid` | The HermesInstance's `metadata.uid` at backup time. Used by the operator to detect cross-instance restores. |
| `hermes_agent_version` | The running `hermes-agent` version. Read from the container env `HERMES_AGENT_VERSION` (Plan 3 sets this). |
| `k8s_version` | The host cluster's k8s minor version. Informational. |
| `timestamp` | RFC 3339 with `:` and `.` replaced by `-` for filesystem safety. |
| `format_version` | Currently `1`. Bumped when the layout changes incompatibly. |

## Compression

`zstd -T0 -19` (long-range, max compression, all cores). Typical compression ratio on hermes data is 5–8× (FTS5 indexes compress especially well).

## Encryption

Encryption is **not** built into the snapshot format. Two options:
1. **Bucket-side encryption** (SSE-S3, SSE-KMS) — recommended.
2. **Restic native encryption** — set `RESTIC_PASSWORD` in the credentials Secret. The operator passes it through. The default builders do not enable this; opt in by adding the env var.

## Cross-instance restore

To restore one instance's snapshot into another:

```yaml
spec:
  restoreFrom: "<source-snapshot-key>"
```

The operator does **not** rewrite `meta.json.instance_uid`. The hermes-agent runtime will see a new UID on the running instance and treat the imported data as foreign. This is intentional — if you don't want that, do the restore manually with `mc cp` + extract + manual edit.

## Format evolution

When `format_version` is bumped:

- Old snapshots remain restorable (backward compatibility is a v1.x stability commitment).
- The operator's init container at runtime version N can read all `format_version` ≤ N.
- Cross-version downgrades (newer snapshot, older operator) are unsupported.
