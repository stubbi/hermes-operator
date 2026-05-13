# Auto-Update

The `hermes-operator` can poll an OCI registry and roll the StatefulSet's image forward automatically. Auto-update is **opt-in**: `spec.autoUpdate.enabled` defaults to `false`.

## Configuration

```yaml
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: "1.4.0"                          # MUST be a concrete semver; do not use `latest`
  autoUpdate:
    enabled: true
    pollInterval: 1h                       # min 15m, max 168h
    backupBeforeUpdate: true              # default true; requires spec.backup.s3 set
    source:
      registry: ghcr.io/stubbi/hermes-agent  # defaults to spec.image.repository
      channel: "1.x"                       # Masterminds/semver constraint; defaults to "<major>.x"
    rollback:
      enabled: true
      probeFailureThreshold: 3            # consecutive Unhealthy/FailedMount events within the 5m window
```

## Semver channels

The channel uses [Masterminds/semver](https://github.com/Masterminds/semver) constraint syntax:

| Channel | Matches |
|---|---|
| `1.x` | any 1.y.z, no prereleases |
| `>=1.4 <2` | 1.4.0 and up, but no 2.x |
| `~1.4` | 1.4.0–1.4.x |
| `1.4.x` | exactly 1.4.0–1.4.x |
| `*` | any tag (use only for non-production) |

**Prereleases are excluded by default** (`1.5.0-rc1` does not match `1.x`). To opt in, use an explicit constraint with the prerelease, e.g. `>=1.5.0-rc1 <2`.

## Rollout flow

```
poll → list tags → HighestMatching(channel) → compare to currentRunningTag
  │
  ├─ no change → set ConditionAutoUpdated=True (reason=UpToDate)
  │
  └─ newer tag T:
        ├─ if T == status.autoUpdate.lastFailedTag → skip, reason=SuppressedKnownFailure
        ├─ take pre-update backup (BackupReconciler.RunOneShot)
        ├─ patch StatefulSet container[0].image (NOT spec.image.tag)
        ├─ annotate `hermes.agent/autoupdate-target=T`
        ├─ set status.autoUpdate.targetTag = T, rolloutDeadline = now+5m
        └─ watch readiness for 5m
              ├─ ReadyReplicas==1, UpdatedReplicas==1 → success: lastSuccessTag=T, condition=Confirmed
              └─ ProbeFailures >= threshold OR past deadline → rollback:
                    ├─ patch STS container[0].image = lastSuccessTag
                    ├─ status.autoUpdate.lastFailedTag = T
                    └─ ConditionAutoUpdateRolledBack=True, reason=RolledBackFrom_T
```

## Why `spec.image.tag` is not patched

The operator deliberately rolls the StatefulSet PodTemplate forward instead of patching `spec.image.tag`. Reasons:

1. **GitOps coexistence.** `spec.image.tag` is what the user sees in Git. If the operator patched it, FluxCD/Argo would either revert the change (causing thrash) or accept it (causing Git/cluster drift). Neither is acceptable. By rolling the STS PodTemplate, the operator owns the "in-flight target" view while the user owns the "intended" view via `spec.image.tag`.
2. **Drift is observable.** `status.autoUpdate.currentTag` reports the actual running tag; `spec.image.tag` reports the intended floor. A discrepancy is a signal, not a bug.
3. **Rollback is local.** A rollback only mutates the STS PodTemplate — no cross-resource ordering, no need to wait for the user to update Git.

To "promote" a confirmed auto-update tag into the spec, the user updates `spec.image.tag` in Git and commits. The operator will observe that `currentRunningTag` already matches and no-op.

## ETag caching

The OCI registry client caches tag lists by ETag. The minimum re-fetch interval is `spec.autoUpdate.pollInterval` (with a global floor of 15 minutes). The client uses `go-containerregistry`'s `remote.List` which honours `If-None-Match`; on `304 Not Modified` the cached list is returned.

This is intentional — pulling a 1000-tag list on every reconcile is rude. In production we observed ~5 round-trips/day per instance on a 1h poll interval.

## Rollback semantics

A rollback is a controller-driven STS image revert plus a `LastFailedTag` record. The controller will not retry the same tag automatically. To force a retry (e.g. after fixing a regression in the registry):

```bash
kubectl patch hermesinstance my-hermes --subresource=status --type=merge -p '{"status":{"autoUpdate":{"lastFailedTag":""}}}'
```

## Common pitfalls

| Symptom | Cause | Fix |
|---|---|---|
| Auto-update never picks up the new tag. | Channel constraint excludes it, e.g. tag is `2.0.0` but channel is `1.x`. | Update the channel. |
| Rollback loop. | `lastFailedTag` is cleared automatically only when a new tag becomes available. Manually clear if needed (see above). | Pin `spec.image.tag` to a known-good and disable autoUpdate temporarily. |
| Pre-update backup fails. | S3 unreachable, credentials wrong. | Fix Secret; the controller retries indefinitely. Disable `backupBeforeUpdate` only as a last resort. |
| `spec.image.tag` and `status.autoUpdate.currentTag` disagree. | Expected — see [Why spec.image.tag is not patched](#why-specimagetag-is-not-patched). | Update `spec.image.tag` in Git once the confirmed tag is acceptable. |

## Disabling auto-update

`spec.autoUpdate.enabled = false` is the supported way to disable. The controller no-ops immediately; any in-flight rollout completes the current readiness window naturally (it does not abandon mid-rollout, to avoid leaving the STS PodTemplate at an indeterminate state).
