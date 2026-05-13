# Hermes Operator: Status Condition Catalogue

> Every condition the operator emits, what it means, what reason codes go with
> it, and how to debug it. This catalogue is part of the v1 stability contract
> (`docs/api-versioning.md` §"Status condition catalogue"). Conditions are
> additive across v1.x; reason codes are stable; both can be relied on by
> dashboards and consumers.

## How to read this catalogue

- Conditions follow the [Kubernetes meta/v1 Condition shape](https://kubernetes.io/docs/reference/using-api/api-concepts/#typical-status-properties):
  `type`, `status` (`True`/`False`/`Unknown`), `reason` (single PascalCase
  token), `message` (human-readable), `lastTransitionTime`, and
  `observedGeneration`.
- The aggregate `Ready` condition (HermesInstance only) is computed from the
  subsystem conditions: `Ready=True` iff every subsystem the spec activates
  reports `True`. The exact formula is in the "Aggregate Ready" subsection
  below.
- `(absent)` in a table means the condition is not set at all (the feature
  is not configured). Consumers MUST treat absence as "not applicable", not
  as failure.
- Reason codes are public API. Renames follow `docs/deprecations.md`.

---

## `HermesInstance` (`hermes.agent/v1`, short `hi`)

### Aggregate `Ready`

`Ready` is the rollup condition surfaced in the printer column `READY`. It
is computed at the end of every reconcile:

| Status | Reason | When |
|---|---|---|
| True | `AllSubsystemsReady` | Every other condition that is set on the object reports `True`. Includes `StorageReady`, `ConfigReady`, `SecretsReady`, `NetworkPolicyReady` (when network policy is enabled), `RBACReady`, `GatewayReady` (when any gateway is enabled), `ProfileStoreReady` (when Honcho is enabled), `WebhookReady`. Auto-update/backup/restore/migration conditions do **not** suppress Ready; they are advisory. |
| False | `SubsystemsPending` | At least one subsystem condition is `False`. `message` lists the failing subsystems comma-separated. |
| False | `Suspended` | `spec.suspended=true`. The instance is intentionally scaled to zero; Ready is `False` so dashboards page on accidental suspension, not on intentional ones. The `message` says "Suspended by spec.suspended=true". |
| Unknown | `Reconciling` | Set on first reconcile before subsystems have all reported. Transitions out within seconds. |

Troubleshooting: `kubectl describe hi <name>` shows every subsystem condition.
The failing ones drive the `message` of `Ready`.

### `StorageReady`

The PVC backing `~/.hermes` is bound and matches spec.

| Status | Reason | When |
|---|---|---|
| True | `PVCBound` | The PVC for `~/.hermes` is `Bound` and its `spec.resources.requests.storage` matches the desired size. |
| False | `PVCPending` | The PVC exists but has `status.phase=Pending`: typically because no `StorageClass` can provision the requested size in the current AZ. |
| False | `PVCMismatch` | The bound PVC has a different `storageClassName` or `accessModes` than the spec asks for. The validator blocks new instances in this state; this condition fires on legacy instances created before validation tightened. |
| False | `ExistingClaimNotFound` | `spec.storage.persistence.existingClaim` references a PVC that does not exist in the namespace. |
| (absent) |: | `spec.storage.persistence.enabled=false`. The instance runs with an `emptyDir`. |

Troubleshooting: `kubectl get pvc -l app.kubernetes.io/instance=<name>` and check
`kubectl describe pvc <pvc>`.

### `ConfigReady`

The agent's `~/.hermes/config.yaml` ConfigMap is built and reflects the spec.

| Status | Reason | When |
|---|---|---|
| True | `ConfigGenerated` | The operator-owned ConfigMap exists, the SHA of its `config.yaml` key matches `status.observedConfigHash`, and (if `spec.config.configMapRef` is set) the referenced ConfigMap resolves. |
| False | `ConfigMapRefMissing` | `spec.config.configMapRef.name` does not exist in the namespace. |
| False | `MergeFailure` | `spec.config.mergeMode=merge` failed to merge raw + ref (YAML conflict at a non-leaf node). The `message` includes the conflicting JSON path. |
| False | `UnknownKey` | A key in `spec.config.raw` is not in the operator's known-schema. This is degraded-to-warning by default; flipped to `False` when `spec.config.strict=true`. |

Troubleshooting: `kubectl get cm <name>-config -o yaml` shows the generated
config. `kubectl describe hi <name>` shows the merge error if any.

### `SecretsReady`

All Secret references in the spec resolve.

| Status | Reason | When |
|---|---|---|
| True | `AllSecretsResolved` | Every Secret referenced by `spec.envFrom`, `spec.gateways.*.secretRef`, `spec.backup.s3.credentialsSecretRef`, `spec.security.imagePullSecrets`, `spec.profileStore.secret`, and `spec.tailscale.authKey.secretRef` exists in the namespace and has the keys the schema expects. |
| False | `SecretNotFound` | A referenced Secret does not exist. `message` lists `name=<x>, expectedBy=<spec.path>`. |
| False | `SecretKeyMissing` | The Secret exists but is missing a key the schema requires (e.g. `accessKey` for S3 creds). |
| False | `SecretRBACDenied` | The operator's ServiceAccount cannot `get` the Secret. Common in namespace-scoped installs. |

Troubleshooting: `kubectl get secret <name>` and `kubectl auth can-i get secret/<name> --as=system:serviceaccount:<ns>:hermes-operator`.

### `NetworkPolicyReady`

The default-deny + allow-list NetworkPolicy is in place.

| Status | Reason | When |
|---|---|---|
| True | `Applied` | The NetworkPolicy named `<instance>-network` exists, has owner-ref pointing at the instance, and matches the spec (deny-all baseline + allow rules derived from `spec.gateways` and `spec.networking.egress`). |
| False | `PolicyEngineMissing` | The cluster has no NetworkPolicy enforcer. The operator detects this by looking for known CNI annotations at startup. Falls back to warning if user has explicitly acknowledged via `spec.networking.networkPolicy.acknowledgeNoEnforcer=true`. |
| (absent) |: | `spec.networking.networkPolicy.enabled=false`. |

Troubleshooting: `kubectl get netpol -n <ns>` and verify the CNI supports
NetworkPolicy.

### `RBACReady`

The agent's ServiceAccount and (when SelfConfig is enabled) the Role+RoleBinding that lets the agent create `HermesSelfConfig` are in place.

| Status | Reason | When |
|---|---|---|
| True | `Applied` | The SA exists and (when `spec.selfConfigure.enabled=true`) the namespace-scoped Role and RoleBinding granting `create` on `hermesselfconfigs` exist with owner-ref. |
| False | `SAAnnotationDrift` | The SA exists but its annotations have drifted from `spec.security.serviceAccount.annotations` because a third party overwrote them. The operator preserves third-party annotations on update; if the operator-owned set is missing it sets this status and re-applies. |
| False | `RoleMissing` | `spec.selfConfigure.enabled=true` but the Role/RoleBinding could not be created (most often a webhook-rejected name conflict). |

Troubleshooting: `kubectl get sa,role,rolebinding -l app.kubernetes.io/instance=<name>`.

### `GatewayReady`

Per-platform gateway wiring (Telegram/Discord/Slack/WhatsApp/Signal).

| Status | Reason | When |
|---|---|---|
| True | `AllEnabledGatewaysWired` | Every gateway with `enabled=true` has its token Secret resolved, its generated config emitted into the agent ConfigMap, and its Service/Ingress allowances applied. |
| False | `TokenSecretMissing` | At least one enabled gateway's `secretRef` does not resolve. `message` names the gateways. |
| False | `IngressUnsupportedForPlatform` | A gateway requested an Ingress (e.g. Slack's events webhook) but `spec.networking.ingress.enabled=false`. The webhook normally rejects this combination; the condition fires on legacy resources. |
| (absent) |: | No gateway has `enabled=true`. |

Troubleshooting: `kubectl describe hi <name>` and check the per-gateway sub-status in `status.gateways[].*`.

### `ProfileStoreReady`

Honcho profile-store companion deployment.

| Status | Reason | When |
|---|---|---|
| True | `HonchoReady` | The Honcho Deployment reports `availableReplicas >= 1`, its Service exists, its PVC (if persistence enabled) is bound, and the Secret holding the API key is present. |
| False | `HonchoPending` | The Honcho Deployment is rolling out. |
| False | `HonchoImagePullBackOff` | The Honcho image cannot be pulled. The operator distinguishes this from generic `HonchoPending` so dashboards alert on it. |
| False | `HonchoSecretMissing` | `spec.profileStore.secret` references a Secret that does not exist. |
| (absent) |: | `spec.profileStore.enabled=false`. |

Troubleshooting: `kubectl get deploy,svc,pvc,secret -l app.kubernetes.io/instance=<name>,app.kubernetes.io/component=honcho`.

### `BackupReady`

State of scheduled backups (from Plan 5, restated here for the catalogue).

| Status | Reason | When |
|---|---|---|
| True | `Scheduled` | A backup CronJob is configured and the most recent run succeeded. `status.backup.lastSuccessfulSnapshotKey` is populated. |
| False | `S3CredentialsMissing` | `spec.backup.s3.credentialsSecretRef` does not resolve. |
| False | `PersistenceDisabled` | `spec.storage.persistence.enabled=false`: scheduled backups require persistence. |
| False | `LastRunFailed` | The most recent backup Job exited non-zero. `status.backup.lastFailureReason` carries the detail. |
| (absent) |: | `spec.backup.schedule` is empty. |

Troubleshooting: `kubectl get cj,job -l app.kubernetes.io/instance=<name>,backup=true` and `kubectl logs job/<last-run>`.

### `RestoreApplied`

Terminal: once `True`, immutable for the lifetime of the instance.

| Status | Reason | When |
|---|---|---|
| True | `RestoreCompleted` | `status.restoredFrom == spec.restoreFrom`. |
| False | `Restoring` | The `init-restore` init container is in progress. |
| False | `RestoreFailed` | The `init-restore` init container exited non-zero. The `message` includes the exit code and the last line of `kubectl logs`. |
| (absent) |: | `spec.restoreFrom` is unset. |

Troubleshooting: `kubectl logs <instance>-0 -c init-restore` and inspect the snapshot key in S3.

### `AutoUpdated`

Outcome of the most recent auto-update cycle.

| Status | Reason | When |
|---|---|---|
| True | `UpToDate` | The current tag is the highest in `spec.autoUpdate.source.channel`. No rollout needed. |
| True | `Confirmed` | A rollout completed and passed the readiness watch window. `status.autoUpdate.lastConfirmedTag` is populated. |
| False | `RolloutInFlight` | A rollout is currently being watched. `status.autoUpdate.targetTag` carries the candidate. |
| False | `RolledBack` | The most recent rollout failed; image reverted. The `message` references the failed tag. |
| False | `NoMatchingTag` | No tag in the registry matches the channel pattern. |
| False | `SuppressedKnownFailure` | The highest matching tag equals `status.autoUpdate.lastFailedTag`: auto-update declines to retry a tag that has already failed. Manual intervention (clear `lastFailedTag` via subresource patch) is required. |
| (absent) |: | `spec.autoUpdate.enabled=false`. |

Troubleshooting: `kubectl get hi <name> -o jsonpath='{.status.autoUpdate}'` for the full sub-status.

### `AutoUpdateRolledBack`

Present only after a rollback. The reason embeds the failed tag.

| Status | Reason | When |
|---|---|---|
| True | `RolledBackFrom_<tag>` | A rollback completed. The message describes why (deadline elapsed or `probeFailureThreshold` reached). |

The condition is removed on the next successful `AutoUpdated=True` (reason=`Confirmed`) cycle, so it acts as a one-shot signal. Dashboards typically alarm on the transition `(absent) → True`.

### `MigrationCompleted`

Terminal: once `True`, immutable for the lifetime of the instance.

| Status | Reason | When |
|---|---|---|
| True | `MigrationCompleted` | The `init-migrate-from-openclaw` init container exited 0. |
| False | `MigrationFailed` | The migration init container exited non-zero. The `message` includes the exit code and a short tail of the init container's stderr. |
| (absent) |: | `spec.migration.fromOpenClaw` is unset. |

Troubleshooting: `kubectl logs <instance>-0 -c init-migrate-from-openclaw`.

### `WebhookReady`

Reflects the operator's ability to serve the admission webhooks for this CR. This condition fires when the webhook serving cert is invalid or the webhook server is unreachable: it is a *cluster-level* failure surfaced per-instance so consumers do not have to know about the operator's pod state.

| Status | Reason | When |
|---|---|---|
| True | `WebhookHealthy` | The operator's webhook server returned `200` to its own self-check probe within the last `RequeueAfter`. |
| False | `CertExpired` | The webhook serving cert's `notAfter` is in the past. cert-manager (when enabled) usually rotates before this fires; when it fires, manual intervention is required. |
| False | `EndpointUnreachable` | The webhook Service has no Endpoints: usually because the operator Pod is not Ready. |
| Unknown | `SelfCheckPending` | First reconcile before the self-check has run. |

Troubleshooting: `kubectl get validatingwebhookconfiguration,mutatingwebhookconfiguration | grep hermes` and `kubectl logs -n hermes-operator deploy/hermes-operator-controller-manager`.

---

## `HermesSelfConfig` (`hermes.agent/v1`, short `hsc`)

Phase derives from these conditions: `Applied → Applied`, `Denied → Denied`, otherwise `Pending`.

### `Applied`

| Status | Reason | When |
|---|---|---|
| True | `SSASuccess` | The SSA patch against the parent `HermesInstance` (and workspace ConfigMap, for `addWorkspaceFiles`) completed without an SSA conflict. `status.appliedAt` and `status.appliedFields` are populated. |
| False | (transient) | Transitioning. The next reconcile will move to `Applied=True` or `Denied=True`. |

### `Denied`

| Status | Reason | When |
|---|---|---|
| True | `PolicyViolation` | The request touched a path on the parent instance's `selfConfigure.protectedKeys` allowlist, or `selfConfigure.enabled=false`. `status.denyReason` carries the human-readable detail. The operator also emits a `Warning` Event with reason `PolicyViolation` on the parent instance so `kubectl describe hi` shows it. |
| True | `InstanceNotFound` | `spec.instanceRef` refers to a `HermesInstance` that does not exist in the namespace. |
| True | `InstanceTerminating` | The parent instance has a `deletionTimestamp` set. |
| True | `SSAConflict` | The SSA patch lost a field-ownership conflict to a different field manager. `denyReason` lists the conflicting path and the other manager's name (e.g. `kustomize-controller`). The user typically resolves this by changing the SelfConfig to a different field or by force-taking ownership manually. |

### `Pending`

| Status | Reason | When |
|---|---|---|
| True | `AwaitingInstanceReady` | The parent instance is not yet `Ready=True`. The SelfConfig reconciler defers application until it is. Prevents racing the initial bring-up. |
| True | `RateLimited` | More than 5 SelfConfigs per minute for the same instance: back off. Reset after the burst window passes. |

Troubleshooting: `kubectl get hsc -n <ns>` shows the phase. `kubectl describe hsc <name>` shows `status.denyReason` or the conditions detail.

---

## `HermesClusterDefaults` (`hermes.agent/v1`, short `hcd`, cluster-scoped singleton `cluster`)

### `Active`

| Status | Reason | When |
|---|---|---|
| True | `Applied` | The singleton `cluster` exists, passes validation, and the defaulting webhook is using it on every admission. `status.observedGeneration == metadata.generation`. |
| False | `WrongName` | A `HermesClusterDefaults` exists with a name other than `cluster`. The validating webhook rejects new ones; this condition exists for legacy resources created before the webhook was installed. |
| (absent) |: | No `HermesClusterDefaults` exists in the cluster. The defaulter falls back to its built-in fallback defaults. |

### `Invalid`

| Status | Reason | When |
|---|---|---|
| True | `SchemaViolation` | A field on the singleton fails server-side validation (e.g. negative quantity, malformed cron). The defaulter ignores invalid fields and uses fallback values for them: the rest of the singleton still applies. `message` lists the offending JSON paths. |
| True | `ImagePullSecretMissing` | `spec.registry.pullSecretName` does not resolve in the operator's namespace. Defaulter skips that field. |

The two conditions can both be `True` simultaneously: `Active=True` (defaults are applied) and `Invalid=True` (some fields are skipped). Dashboards key on `Invalid` for alerting.

---

## Reason-code naming convention

For consistency across CRs and to make grep work in dashboards:

- **PascalCase**, no spaces, no slashes (allow underscore only for value-carrying reasons like `RolledBackFrom_<tag>`).
- **One token per cause.** "PVCBoundAndSizeMatches" is wrong; the reason is `PVCBound`, the size match is implicit.
- **Reasons are added, never repurposed.** Adding `S3RegionInvalid` as a new reason for `BackupReady=False` is non-breaking. Changing what `S3CredentialsMissing` means is breaking.
- **Reasons that embed values** use `_` as the separator (the only allowed underscore use).
