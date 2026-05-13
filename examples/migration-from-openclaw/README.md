# Migrating from OpenClaw → Hermes

A one-shot, declarative migration path from openclaw-operator. The hermes-
agent image ships with an importer (`hermes-agent migrate from-openclaw`)
that the operator wires up as an init container when `spec.migration.fromOpenClaw`
is set. The field becomes immutable after `MigrationCompleted=True`.

Two sources are supported:

1. **Sibling in-cluster `OpenClawInstance`** — when openclaw-operator is
   still installed in the cluster (typical for an in-place cutover).
2. **S3 backup snapshot** — when openclaw is already gone and you only
   have the snapshot.

You cannot combine `spec.migration.fromOpenClaw` with `spec.restoreFrom`
on the same instance — the validator rejects this combination because
the order of operations is ambiguous. To do both, run two instances and
join the data manually.

## Mode A: sibling `OpenClawInstance`

```bash
kubectl create namespace agents

# Assumes my-openclaw is already running in the agents namespace.
kubectl apply -n agents -f from-sibling.yaml

# Watch the migration init container.
kubectl logs -f -n agents migrated-from-sibling-0 \
  -c init-migrate-from-openclaw

kubectl get hi migrated-from-sibling -n agents \
  -o jsonpath='{.status.conditions[?(@.type=="MigrationCompleted")]}'
# { "status":"True", "reason":"MigrationCompleted", ... }
```

`mode: copy` (default) leaves the source `OpenClawInstance` untouched.
`mode: move` marks the source as terminated by setting an annotation
(`openclaw.rocks/migrated-to=<hermes-uid>`) so subsequent operator
reconciles on the openclaw side know to stop scheduling work. Deleting
the openclaw CR after migration is your responsibility.

## Mode B: S3 backup

```bash
kubectl create namespace agents

kubectl create secret generic oc-s3-creds \
  -n agents \
  --from-literal=accessKey=REPLACE \
  --from-literal=secretKey=REPLACE

kubectl apply -n agents -f from-backup.yaml
kubectl logs -f -n agents migrated-from-backup-0 \
  -c init-migrate-from-openclaw
```

## What gets migrated

The hermes-agent importer migrates:

- The workspace tree (everything under `~/.openclaw/workspace` becomes
  `~/.hermes/workspace`).
- The session-memory SQLite (mapped through a schema shim — the importer
  upgrades FTS5 indexes in place).
- Skills (anything in `~/.openclaw/skills` that has a `hermes-compatible: true`
  marker in its `metadata.yaml`; everything else is logged and skipped).
- Config — `~/.openclaw/config.yaml` is translated through a known field
  map.

It does **not** migrate:

- `~/.openclaw/credentials/` (paths differ; recreate via Secret refs in the
  new instance).
- Honcho profiles (openclaw does not run Honcho; nothing to migrate).
- Container-internal state in `/tmp`, `/root/.cache`, etc.

## Verifying after migration

```bash
kubectl exec -n agents migrated-from-sibling-0 \
  -- hermes-agent status --json | jq .lastImport
# {
#   "completed_at": "2026-05-12T13:00:00Z",
#   "source":       "openclaw:my-openclaw",
#   "skills_imported": 7,
#   "skills_skipped":  2,
#   "workspace_files": 142,
#   "session_rows":   8324
# }
```
