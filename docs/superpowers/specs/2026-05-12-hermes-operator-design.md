# hermes-operator v1: Design

- **Status:** Approved (2026-05-12)
- **Owner:** stubbi (jannes@aqora.io)
- **Repo:** `stubbi/hermes-operator` (private at start)
- **Target ship:** v1.0.0: full feature parity with openclaw-operator v0.32 + hermes-specific bits, no v0.x grind
- **Inspired by:** [openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator), iterated through v0.5 → v0.32 with substantial production feedback
- **Manages:** [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent): Python-based self-improving multi-platform AI agent

## 1. Context & Goals

Hermes-agent is a Python 3.11+ AI agent that fronts multiple messaging platforms (Telegram, Discord, Slack, WhatsApp, Signal), self-improves through a built-in learning loop, persists session memory via FTS5, models users via Honcho dialectic profiles, and runs scheduled automations via a native cron scheduler. Upstream ships as a `uv`-installable Python package plus CLI; there is no canonical container image yet.

The goal is a Kubernetes operator that deploys and manages hermes-agent instances with production-grade security, observability, and lifecycle management: *out of the gate* at v1.0, not after the v0.x grind openclaw-operator went through.

### Goals

- **G1: Full feature parity with openclaw-operator v0.32** adapted to hermes's Python/uv runtime and multi-platform gateway shape.
- **G2: v1 API stable from day one.** No `v1alpha1` spoke. Public versioning, deprecation, and conversion policies published with v1.0.
- **G3: Lessons baked in.** Openclaw issues #437, #446, #433, #471, #479, #458, #469, and the rest of the v0.x bug log informed concrete guardrails (Reconcile Guard CI, SSA for SelfConfig from day one, explicit k8s defaults in builders, zombie-process reaper, namespace-scoped RBAC option, etc.).
- **G4: GitOps coexistence.** SSA-based SelfConfig means FluxCD/Argo can manage the instance while the agent self-mutates allowed fields without flap.
- **G5: One declarative migration path from openclaw.** `spec.migration.fromOpenClaw` uses hermes-agent's built-in importer to drain a sibling OpenClawInstance or its S3 backup.
- **G6: Distribution from day one:** Helm chart, OLM bundle (OperatorHub submission), plain manifests, signed multi-arch container images with SBOMs.

### Non-goals

- **NG1: Multi-cluster federation.** Deferred; single-cluster control loop only.
- **NG2: Modal / Daytona "hibernation" integration.** K8s-native scale-to-zero (`spec.suspended`) is the equivalent; we don't reach into external serverless platforms.
- **NG3: Generic "AgentInstance" operator.** Hermes is the only runtime in v1. No premature abstraction.
- **NG4: kubectl plugin (krew) for v1.0.** Nice to have, not blocking; mirrors `kubectl-openclaw` later.
- **NG5: Public OpenClaw → Hermes data conversion guarantees beyond what hermes-agent's importer itself provides.**

## 2. Project basics

| | |
|---|---|
| Repo | `stubbi/hermes-operator` (private) |
| Go module | `github.com/stubbi/hermes-operator` |
| Go version | 1.24 |
| Framework | kubebuilder v4 / controller-runtime |
| Kubernetes | ≥ 1.28 (CI matrix: 1.28, 1.29, 1.30, 1.31, 1.32) |
| License | Apache-2.0 (operator). Independent of hermes-agent's MIT. |
| Operator image | `ghcr.io/stubbi/hermes-operator` (multi-arch amd64+arm64, Cosign-signed, SBOM attested) |
| Agent image | `ghcr.io/stubbi/hermes-agent` (operator's responsibility to build/publish from the upstream Python package; default `spec.image.repository`) |
| Conventional commits | `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `refactor:`, `test:` (release-please uses `feat:`/`fix:` for changelog) |

## 3. CRD surface

| CRD | Scope | API | Short | Purpose |
|---|---|---|---|---|
| `HermesInstance` | Namespaced | `hermes.agent/v1` | `hi` | The agent: spec for a single hermes-agent deployment. |
| `HermesSelfConfig` | Namespaced | `hermes.agent/v1` | `hsc` | Agent-initiated mutations, validated against the parent instance's `selfConfigure` policy, applied via SSA. |
| `HermesClusterDefaults` | Cluster (singleton; name must be `cluster`) | `hermes.agent/v1` | `hcd` | Cluster-wide defaults and policy applied by the defaulting webhook. |

- **Categories:** `hermes`, `agents`. `kubectl get agents` lists all three.
- **Storage version:** `v1` only. No spoke. Conversion-webhook scaffolding is in place from day one so future `v2` can land without re-plumbing.
- **Status conventions:** every CR uses `meta.SetStatusCondition` for conditions plus `observedGeneration` and subsystem readiness flags (StorageReady, ConfigReady, SecretsReady, GatewayReady, ProfileStoreReady on HermesInstance). Condition catalogue published in `docs/conditions.md`.

## 4. `HermesInstance` spec

Top-level shape, each a typed sub-spec:

```yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: my-hermes
spec:
  image:             # repository, tag, digest, imagePullPolicy, imagePullSecrets
  config:            # YAML config for ~/.hermes/config.yaml (raw / configMapRef / mergeMode)
  workspace:         # initial files & directories seeded into ~/.hermes
  resources:         # cpu / memory requests + limits
  security:          # podSecurityContext, container security, RBAC, CA bundle, SA annotations (IRSA/WI)
  storage:           # PVC: enabled, size, storageClassName, accessModes, existingClaim
  networking:        # Service, Ingress, NetworkPolicy (deny-all default)
  observability:     # metrics, ServiceMonitor, logging
  availability:      # PDB, HPA, topologySpreadConstraints
  probes:            # liveness, readiness, startup overrides
  backup:            # S3 target, schedule, onDelete, preUpdate, history limits
  restoreFrom:       # snapshot key to restore into a new instance
  runtime:           # Python version, uv, ffmpeg, ripgrep, extra apt/pip
  gateways:          # platform bindings: telegram, discord, slack, whatsapp, signal
  profileStore:      # Honcho companion (enabled, image, persistence, secret)
  ollama:            # optional local LLM sidecar
  webTerminal:       # optional kubectl-attach style terminal sidecar
  tailscale:         # optional Tailscale Serve / Funnel
  autoUpdate:        # opt-in OCI registry polling, rollback on failed probes
  selfConfigure:     # allowlist policy for HermesSelfConfig mutations
  migration:         # fromOpenClaw: declarative one-shot migration
  scheduling:        # nodeSelector, tolerations, affinity, priorityClassName
  initContainers:    # arbitrary additional init containers
  sidecars:          # arbitrary additional sidecars
  extraVolumes:
  extraVolumeMounts:
  envFrom:           # secretRef / configMapRef list for env injection
  env:               # explicit env vars
  suspended:         # scale-to-zero
```

### 4.1 Hermes-specific deltas from `OpenClawInstance`

| Area | Hermes change | Why |
|---|---|---|
| `runtime` | Replaces openclaw's `runtimeDeps`. Defaults: `python: "3.11"`, `uv: latest`, `ffmpeg.enabled: true`, `ripgrep.enabled: true`. Init container runs `uv sync` against a lockfile bundled in the agent image. | Hermes is Python-native; pnpm/Node init containers don't apply. FFmpeg + ripgrep are hard dependencies of hermes-agent. |
| `gateways` | First-class section with `telegram`, `discord`, `slack`, `whatsapp`, `signal`. Each takes one or more `secretRef`s for tokens, exposes `enabled`, and contributes generated config + Service/Ingress allowances. | Hermes is a multi-platform gateway; tokens live in many secrets and must be auditable/rotatable independently. |
| `profileStore` | New section for Honcho companion (Deployment + Service + PVC, optional). | Honcho is hermes-shaped, no openclaw analogue. |
| `selfConfigure.allowedActions` | `[skills, config, envVars, workspaceFiles, profiles]`: adds `profiles` (Honcho snapshot persistence). | Hermes self-improves natively; SelfConfig is the audited cluster-side surface. |
| `migration.fromOpenClaw` | Optional one-shot migration via hermes-agent's built-in importer. Source: in-cluster `OpenClawInstanceRef` or S3 backup snapshot. | Hermes-agent already ships the importer; surfacing it declaratively turns the OpenClaw → Hermes transition into a single resource change. |
| No `chromium` sub-spec | Removed. If browser automation is needed, use generic `sidecars`. | Not first-class for hermes-agent. |
| No `BOOTSTRAP.md` injection toggle | Removed. | Openclaw-specific. |
| No `OPENCLAW_DISABLE_BONJOUR` equivalent | Removed. | Hermes has no mDNS pairing. |
| `config.format` | `yaml` only. | `~/.hermes/config.yaml` is canonical. |

### 4.2 Things kept verbatim (proven in openclaw)

- **StatefulSet** (single replica by default; HPA opt-in via `availability.hpa`). Preserves identity through restarts.
- **Default-deny `NetworkPolicy`** baseline + explicit allow rules derived from `gateways` and `networking.egress`.
- **PodDisruptionBudget** always created when `replicas > 1`.
- **Read-only root filesystem** by default; writable `emptyDir`s for `/tmp`, `~/.config` writable subPath on the PVC (openclaw lesson #458).
- **Prometheus metrics + ServiceMonitor** with `metrics.secure` consistency between operator flag and ServiceMonitor scheme (lesson #435/#440).
- **Validating webhook** for spec sanity + provider/secret cross-warnings.
- **Finalizer for backup-on-delete**, set via `r.Patch` (JSON patch), never `r.Update` (lesson #437).
- **Owner refs on every managed resource**; `controllerutil.CreateOrUpdate` exclusively (Reconcile Guard CI enforced).

## 5. `HermesSelfConfig` spec

Agent-driven, audited mutation API. The agent creates one of these to persist a learned skill, env var, config patch, workspace file, or Honcho profile snapshot. The operator validates against the parent instance's `selfConfigure` policy, then applies via SSA.

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: install-finance-skill
spec:
  instanceRef: my-hermes              # required, same namespace
  addSkills:
    - source: "git+https://github.com/foo/finance-skill@v1.2.0"
  patchConfig:                        # JSON-merge-patch into ~/.hermes/config.yaml
    schedules:
      morning-brief: "0 8 * * *"
  addEnvVars:
    - name: FINANCE_TZ
      value: Europe/Berlin
  addWorkspaceFiles:
    - path: "notes/finance.md"
      content: "..."
  addProfileSnapshot:
    profileID: "user-42"
    data: "..."
status:
  conditions:
    - type: Applied | Denied | Pending
  appliedAt: <ts>
  denyReason: ""                      # populated when denied
```

**Reconciler:** validate → SSA-patch the `HermesInstance` (allowed fields only) and the workspace ConfigMap (for `addWorkspaceFiles`) → set status. SSA field manager = `hermes.agent/selfconfig`, so GitOps controllers writing to *other* fields on the same instance keep working without flap.

## 6. `HermesClusterDefaults` spec

Cluster-scoped singleton (name **must** be `cluster`; webhook rejects any other name). Provides cluster-wide defaults applied by the defaulting webhook when an instance leaves a field `nil`. Examples:

```yaml
apiVersion: hermes.agent/v1
kind: HermesClusterDefaults
metadata:
  name: cluster
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: "1.4.2"
  registry:
    pullSecretName: ghcr-pull
  storage:
    storageClassName: gp3
    size: 10Gi
  security:
    serviceAccount:
      annotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::123:role/hermes
  observability:
    serviceMonitor:
      enabled: true
  networking:
    networkPolicy:
      enabled: true
```

ClusterDefaults *only* fill `nil` fields; an explicit value on the instance always wins. This is the inverse of "policy overrides": for hard enforcement, the validating webhook is the right mechanism, not defaults.

## 7. Reconciliation architecture

### 7.1 Code layout

```
api/v1/                       hub types + webhook impls + groupversion_info.go
  hermesinstance_types.go
  hermesselfconfig_types.go
  hermesclusterdefaults_types.go
  webhook_hermesinstance.go
  webhook_hermesselfconfig.go
  webhook_hermesclusterdefaults.go
  zz_generated.deepcopy.go

internal/controller/          orchestration only: no resource construction here
  hermesinstance_controller.go
  hermesselfconfig_controller.go
  hermesclusterdefaults_controller.go
  backup.go, restore.go, autoupdate.go, s3.go, metrics.go
  *_test.go (envtest suite)

internal/resources/           pure builder funcs, one file per resource
  statefulset.go service.go configmap.go secret.go networkpolicy.go pdb.go
  hpa.go ingress.go rbac.go servicemonitor.go prometheusrule.go pvc.go
  honcho.go gateways.go runtime_init.go selfconfig_apply.go common.go
  resources_test.go resources_bench_test.go

internal/webhook/             validator + defaulter implementations
config/crd/bases/             generated CRD YAML (committed)
charts/hermes-operator/       Helm chart (CRDs templated, RBAC auto-synced)
bundle/                       OLM bundle for OperatorHub
test/e2e/                     kind-cluster tests
test/conformance/             negative + idempotency + upgrade-matrix + GitOps tests
cmd/manager/                  entrypoint
docs/                         api-reference.md, conditions.md, api-versioning.md, deprecations.md, examples/, images/
hack/                         boilerplate, kustomize bins, env helpers
```

**Hard separation:** controllers orchestrate the `CreateOrUpdate` dance; *all* resource construction lives in pure `Build<Resource>(*HermesInstance, *HermesClusterDefaults) *<Resource>` functions in `internal/resources/`. Builders are unit-tested without envtest in <2s.

### 7.2 Reconciliation rules (CI-enforced by `Reconcile Guard` job)

1. **Only `controllerutil.CreateOrUpdate`** for managed resources. Bare `r.Update()` is grep-banned. Exception: `// reconcile-guard:allow` with justification comment.
2. **Server-Side Apply (SSA)** for the `HermesSelfConfig` reconciler from day one (lesson #433/#439). Field manager = `hermes.agent/selfconfig`.
3. **Set every server-side k8s default explicitly** in builders: `RevisionHistoryLimit`, `ProgressDeadlineSeconds`, `RestartPolicy`, `DNSPolicy`, `SchedulerName`, `TerminationGracePeriodSeconds`, `TerminationMessagePath`, `TerminationMessagePolicy`, `ImagePullPolicy` on every container, `SuccessThreshold` on every probe, `DefaultMode` on volume sources, `SessionAffinity: None` on Service. Skipping these is what produced openclaw's generation-thrash bugs.
4. **Preserve third-party annotations and labels** on update (lesson #446/#447). Merge function whitelists operator-owned keys (`hermes.agent/*`, well-known kubebuilder/k8s labels) for overwrite; everything else is preserved.
5. **Finalizer add/remove uses `r.Patch()` with a JSON patch**, never `r.Update()` (lesson #437: finalizer add bumped generation and replaced the pod on first reconcile).
6. **Preserve server-assigned fields** on update: `Service.ClusterIP/ClusterIPs`, `PVC.VolumeName`, etc. PVCs are never updated, only created (immutable per k8s).
7. **Status updates are separate transactions** from spec/metadata (`r.Status().Update`).
8. **Owner references on every managed resource** so deletion cascades.
9. **Reconcile result discipline:** `RequeueAfter: 5m` for drift detection; `Requeue: true` only when an immediate retry is meaningful; on error return `(ctrl.Result{}, err)` and let exponential backoff handle it.

### 7.3 Webhook design

**Defaulter:** populates `nil` fields from `HermesClusterDefaults` (singleton). ClusterDefaults fills `nil` only: explicit values on the instance always win.

**Validator (instance):**
- Required fields (`image.repository` when no clusterDefault, valid storage size, exactly-one of `config.raw` / `config.configMapRef`).
- Gateway/secret cross-checks: warn (not deny) if `gateways.telegram.enabled` and no resolvable token secret exists.
- Unknown top-level config keys → warning (lesson from openclaw v0.10 provider-aware warnings).
- Immutable fields after creation: `storage.persistence.storageClassName`, `storage.persistence.accessModes`, `metadata.name`.
- Forbid `selfConfigure.enabled: true` with `selfConfigure.protectedKeys` empty: must be explicit allowlist.

**Validator (selfconfig):** denies any request that touches a `protectedKeys` path; logs deny with reason as a k8s Event so it surfaces in `kubectl describe`.

**Validator (clusterdefaults):** name must be `cluster`; rejects everything else.

### 7.4 Operational guardrails carried from openclaw

- **Zombie-process reaper** (lesson #471, still OPEN on openclaw): agent container ships with tini as PID 1; `shareProcessNamespace: false` by default.
- **Multi-namespace + namespace-scoped RBAC** opt-in in Helm chart (lesson #469/#470). Default = cluster-scoped; opt-in `watchNamespaces: [...]` with scoped Role/RoleBinding.
- **ClusterRole aggregation labels** on user-facing roles so they fold into Kubernetes `admin`/`edit`/`view` automatically (lesson #479/#480). `kubectl auth can-i create hermesinstances --as=jane` returns the right answer with no extra config.
- **Init containers mount the full data volume** for hostPath PVCs (lesson #450).
- **Read-only root FS with explicit writable subPaths** for `~/.config`, `/tmp` (lesson #458).

## 8. Day-2 operations

### 8.1 Auto-update (opt-in, OCI-registry polling)

```yaml
spec:
  autoUpdate:
    enabled: true
    source:
      registry: ghcr.io/stubbi/hermes-agent
      channel: "1.x"                    # semver range; default = same major as current
    pollInterval: 1h
    rollback:
      enabled: true
      probeFailureThreshold: 3          # roll back after N readiness probe failures post-rollout
    backupBeforeUpdate: true            # default true; records snapshotID in status
```

Controller: every `pollInterval`, list tags via OCI registry, pick the highest tag in the channel, compare to current. If newer: take a pre-update backup → patch `spec.image.tag` via a status-tracked annotation (not user spec) → watch readiness for 5 minutes → on failure restore previous tag and record the failed version in status to suppress retry.

### 8.2 Backup / Restore (S3-compatible)

```yaml
spec:
  backup:
    s3:
      bucket: hermes-backups
      endpoint: s3.amazonaws.com        # any S3-compatible (R2, MinIO, etc.)
      region: us-east-1
      pathPrefix: prod/
      credentialsSecretRef:
        name: hermes-s3-creds
    schedule: "0 3 * * *"               # cron, optional
    onDelete: true                      # back up before allowing finalizer to release
    preUpdate: true                     # back up before autoUpdate rollout
    historyLimit: 30
    failedHistoryLimit: 3
  restoreFrom: "prod/my-hermes/2026-05-10T03-00.tar.zst"
```

- Finalizer `hermes.agent/backup-on-delete` holds CR deletion until the backup Job finishes.
- Backups run as one-shot Jobs using a `restic`/`rclone`-style image that snapshots the PVC.
- `restoreFrom` triggers an init-container restore on a fresh PVC; once `status.restoredFrom == spec.restoreFrom`, the field is locked (immutable thereafter: preventing accidental re-restore on restart).
- Snapshot manifest: `tar.zst` of the PVC root + `meta.json` (instance UID, hermes-agent version, k8s version, timestamp).

### 8.3 Migration from OpenClaw

```yaml
spec:
  migration:
    fromOpenClaw:
      source:
        # Option A: in-cluster sibling OpenClawInstance
        openclawInstanceRef:
          name: my-openclaw
          namespace: agents
        # Option B: from a backup snapshot
        backupRef:
          s3:
            bucket: openclaw-backups
            key: prod/my-openclaw/2026-05-11.tar.zst
            credentialsSecretRef:
              name: oc-s3-creds
      mode: copy                        # copy | move; move marks source as terminated
```

On first reconcile, if `migration.fromOpenClaw` is set and `status.migrationCompleted == false`: run a migration init container that mounts the source PVC (or downloads the snapshot), invokes `hermes-agent migrate from-openclaw`, writes results onto the hermes PVC, sets status. The field becomes immutable after success.

## 9. Distribution

| Channel | What | Notes |
|---|---|---|
| **Helm chart** | `charts/hermes-operator/` | CRDs templated under `templates/crds/` so `helm upgrade` propagates schema changes. `make sync-chart-crds` enforced by CI. RBAC auto-derived from kubebuilder markers via a CI check. Values: `watchNamespaces`, `createRBAC`, `logLevel`, `metrics.secure`, `webhook.certManager.enabled`. |
| **OLM bundle** | `bundle/` | OperatorHub submission via the `community-operators` repo. Bundled CSV, descriptors per CRD, OpenAPI schemas, alm-examples. Auto-submission workflow on minor releases. |
| **Plain manifests** | `config/` (kustomize) | `kustomize build config/default` produces a single YAML for `kubectl apply` users. |
| **Container images** | `ghcr.io/stubbi/hermes-operator`, `ghcr.io/stubbi/hermes-agent` | Multi-arch (amd64+arm64), Cosign-signed (keyless OIDC), SBOM attested, uploaded to release assets. |

### 9.1 Release pipeline

- Conventional commits (`feat:` / `fix:`) on main → `release-please` opens a release PR.
- Merging the release PR bumps `CHANGELOG.md`, `.release-please-manifest.json`, `Chart.yaml`, `appVersion`, then tags `vX.Y.Z` (via PAT so downstream workflows fire).
- Tag triggers GoReleaser: binaries + multi-arch images (draft release) → Cosign sign → SBOM generate+attest → SBOM uploaded → release published.

## 10. Testing strategy

This is where v1 earns its name: substantially stronger than openclaw's v0.x baseline.

1. **Unit tests** (`internal/resources/*_test.go`): every builder is pure, no envtest, runs <2s.
2. **envtest integration tests** (`internal/controller/*_test.go`): reconcile loops against a fake apiserver; happy-path create/update/delete per CRD.
3. **E2E tests** (`test/e2e/`): kind cluster, real reconciliation, asserts on real resources. Required on every PR.
4. **Conformance suite** (`test/conformance/`): **new for v1, ships day one:**
   - **Negative tests**: every webhook-rejection path asserts the rejection.
   - **Idempotency**: 10 reconciles in a row must not change `metadata.generation` or `resourceVersion` after the first. This is the test that would have caught openclaw's #437 before it shipped.
   - **Upgrade-path matrix**: install v1.0.0 → create resources → upgrade operator to vCurrent → assert resources unchanged. Runs for every minor release.
   - **GitOps coexistence**: apply an instance via FluxCD with SSA, have a fake agent create a SelfConfig touching different fields, assert no flap and both reconcilers converge.
   - **Failure injection**: kill the manager mid-reconcile, assert eventual consistency.
5. **Performance benchmarks** (`*_bench_test.go` in both resources and controller). Tracked over time; CI fails on >20% regression.
6. **Security scans**: gosec + Trivy CRITICAL/HIGH on every PR.
7. **Reconcile Guard**: grep-banned patterns: `r.Update()` on managed resources, missing SSA on selfconfig path, finalizer-only `r.Update` on the CR.
8. **Helm RBAC Sync**: diff kubebuilder RBAC markers vs Helm ClusterRole; fail on drift.

**CI matrix:** k8s 1.28, 1.29, 1.30, 1.31, 1.32. Drop the oldest as Kubernetes EOLs it.

## 11. v1 stability commitments

The thing that makes this "v1, not v0.1." Each commitment is documented in a dedicated file shipped with v1.0.

### 11.1 API versioning policy (`docs/api-versioning.md`)

- `hermes.agent/v1` will not have *breaking* changes for the lifetime of v1.x.
- New optional fields are non-breaking (`omitempty` + sane defaults).
- Field removal requires `hermes.agent/v2` + a conversion webhook + ≥6 months overlap.
- Status field semantics are stable; new conditions are additive.

### 11.2 Deprecation policy (`docs/deprecations.md`)

A field is deprecated by:

1. `// Deprecated:` godoc + `+kubebuilder:validation:Description` warning.
2. Webhook warning on use.
3. Entry in `CHANGELOG.md` and `deprecations.md` with target removal version (≥2 minors out, ≥6 months).

### 11.3 Conversion-webhook scaffolding

Set up at v1.0 with v1 as both hub and storage, no spokes yet. When v2 lands, the conversion plumbing is in place: no retrofit.

### 11.4 Status condition catalogue (`docs/conditions.md`)

Every condition documented with reason codes. Status consumers (dashboards, kubectl plugins) can rely on the catalogue across all v1.x.

### 11.5 Supported Kubernetes versions

Declared in `README.md`. Drop happens on minor releases only, never patch.

### 11.6 Versioning surfaces

- **Operator chart version**: semver of the chart itself.
- **Operator appVersion**: semver of the operator image (chart `appVersion`).
- **Hermes-agent image version**: `spec.image.tag` or `HermesClusterDefaults.spec.image.tag`. Decoupled from operator version; operator advertises supported agent versions in `README.md`.

## 12. Post-v1.0 roadmap (non-binding)

- Multi-cluster federation.
- Scale-from-zero on incoming webhook event.
- `kubectl-hermes` plugin (krew).
- Grafana dashboard library.
- AI provider health monitoring and cost recommendations.

Nothing on this list is breaking-API for v1.x.

## 13. Open questions

*None as of approval (2026-05-12). Append here if anything surfaces during implementation planning.*
