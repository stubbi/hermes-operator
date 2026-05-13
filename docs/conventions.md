# Hermes Operator: Engineering Conventions

> Referenced by Plans 2-7. Plan 1 establishes the patterns; this doc names them.

## Code layout

- `api/v1/<kind>_types.go`: CRD types. One file per kind.
- `internal/controller/<kind>_controller.go`: orchestration only. No resource construction.
- `internal/resources/<resource>.go` + `<resource>_test.go`: pure builder funcs. One file per resource type.
- `internal/webhook/<kind>_<webhook>.go`: validator + defaulter implementations (added in Plan 2).
- `config/crd/bases/`: generated CRD YAML, committed.
- `charts/hermes-operator/templates/crds/`: synced from `config/crd/bases/` via `make sync-chart-crds` (CI-enforced).
- `test/e2e/`: kind cluster tests. `test/conformance/`: negative + idempotency + upgrade-path tests (Plan 6).

## Naming

- Deterministic resource names: `Build<Resource>(inst)` and `<Resource>Name(inst)` always live together.
- `<Resource>Name(inst)` returns e.g. `inst.Name + "-data"` (PVC), `inst.Name + "-config"` (ConfigMap). The Service and StatefulSet are named `inst.Name`.
- Plan 2+ may add new resources; pick a short, deterministic suffix and document it here.

## Labels

Every operator-managed resource carries the labels in `resources.LabelsForInstance(inst)`:

| Label | Value |
|---|---|
| `app.kubernetes.io/name` | `hermes-agent` |
| `app.kubernetes.io/instance` | `<inst.Name>` |
| `app.kubernetes.io/managed-by` | `hermes-operator` |
| `app.kubernetes.io/part-of` | `hermes.agent` |

## Reconciliation rules (CI-enforced where noted)

1. **`controllerutil.CreateOrUpdate` exclusively** for managed resources. Bare `r.Update()` / `r.Create()` outside the PVC path is grep-banned. Exception: `// reconcile-guard:allow` with justification.
2. **Server-Side Apply (SSA)** for the `HermesSelfConfig` reconciler from day one (lesson #433/#439). Field manager = `hermes.agent/selfconfig`. *(Plan 4)*
3. **Set every k8s server-side default explicitly in builders.** RevisionHistoryLimit, ProgressDeadlineSeconds, RestartPolicy, DNSPolicy, SchedulerName, TerminationGracePeriodSeconds, TerminationMessagePath/Policy, ImagePullPolicy on every container, SuccessThreshold on every probe, DefaultMode on volume sources, SessionAffinity:None on Service, PodManagementPolicy, UpdateStrategy, PersistentVolumeClaimRetentionPolicy on StatefulSet. Skipping these is what produced openclaw's generation-thrash bugs.
4. **Preserve third-party annotations + labels on update** via `resources.MergePreservingForeign` with prefix `hermes.agent/` (lesson #446/#447).
5. **Finalizer add/remove uses `r.Patch()` with a JSON patch**, never `r.Update()` (lesson #437).
6. **Preserve server-assigned fields on update**: Service.ClusterIP/ClusterIPs, etc. PVCs are immutable: only create, never update.
7. **Status updates are a separate transaction** (`r.Status().Update`) from spec/metadata.
8. **Owner refs on every managed resource** via `controllerutil.SetControllerReference`.

## Idempotency

Every reconciler must satisfy: applying the same spec twice produces the same generation/resourceVersion on each managed resource. The envtest suite in `internal/controller/` includes an idempotency canary test; do not skip it.

## Commit messages

Conventional commits required. Release-please uses `feat:` and `fix:` for the changelog. Acceptable prefixes: `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `refactor:`, `test:`, `build:`, `perf:`.

## Git worktrees

Always use `git worktree` when working on a separate branch:

```bash
git worktree add ../hermes-operator-<suffix> -b <branch> main
# work, commit, push; then:
git worktree remove ../hermes-operator-<suffix>
```

Never `git checkout` or `git switch` in the main working tree.

## Go style

- Use `0o644` (not `0644`) for octal literals: `gocritic.octalLiteral` enforces this.
- Wrap errors: `fmt.Errorf("context: %w", err)`.
- Use `resources.Ptr[T]` for short-lived pointer literals.
- No em/en dashes in code, comments, strings: use regular `-` / `--`.
- `make fmt` before committing.

## CRD type changes: generation workflow

After modifying `api/v1/*_types.go`:

1. `make generate` (regenerates `zz_generated.deepcopy.go`).
2. `make manifests` (regenerates CRD YAML in `config/crd/bases/`).
3. `make sync-chart-crds` (copies CRD YAML into the Helm chart).
4. Commit the generated files.

## Documentation drift

When adding or changing CRD fields, update both:
- `README.md`: user-facing overview and feature table.
- `docs/api-reference.md`: exhaustive field-level reference (added in Plan 2).

Both must stay in sync with the types.

## Testing strategy (full picture in Plan 6)

- **Unit** in `internal/resources/*_test.go`: pure, fast, no envtest.
- **envtest** in `internal/controller/*_test.go`: reconcile against fake apiserver.
- **E2E** in `test/e2e/`: kind cluster, real resources.
- **Conformance** in `test/conformance/`: negative, idempotency, upgrade-path, GitOps coexistence, failure injection. *(Plan 6)*
- **Benchmarks** in `*_bench_test.go`. *(Plan 6)*

## v1 stability: non-negotiable

- API group `hermes.agent`, version `v1`. No `v1alpha1` spoke.
- New optional fields with `omitempty` and sane defaults: non-breaking.
- Field removal requires `hermes.agent/v2` + conversion webhook + ≥6 months overlap.
- Deprecation: godoc `// Deprecated:`, webhook warning, CHANGELOG + `docs/deprecations.md` entry, target removal ≥2 minors out.

## Explicit Kubernetes defaults (extended)

Plan 1 listed StatefulSet / Service / Probe defaults. Plan 2 adds these:

| Resource | Field | Default value |
|---|---|---|
| HorizontalPodAutoscaler | `spec.metrics[].resource.target.type` | `Utilization` (set explicitly) |
| Ingress | `spec.rules[].http.paths[].pathType` | `Prefix` (set when nil) |
| ServiceMonitor | `spec.endpoints[].scheme` | `http`; `https` when `metrics.secure=true` (must agree -- lesson #435/#440) |
| NetworkPolicy | `spec.policyTypes` | both `Ingress` and `Egress` explicitly (k8s defaults to only `Ingress` when omitted) |
| PodDisruptionBudget | one of `MinAvailable` / `MaxUnavailable` | when neither set, `MaxUnavailable: 1` |
| Role | `apiGroups` | empty string `""` for core resources, explicit other groups |

## Well-known egress endpoints

The operator's default-deny `NetworkPolicy` allows only DNS to kube-dns out of the box. Each `spec.gateways.<platform>.enabled: true` adds an egress allow for the upstream's well-known endpoints. CNI plugins that support FQDN peers (Cilium, Calico with `dns` selector) should match the hostnames below; plugins without FQDN support fall back to a port-only rule (443/TCP to any destination), which is wider than ideal: document the trade-off when shipping the cluster.

| Gateway | Hostnames | Port | Protocol | Notes |
|---|---|---|---|---|
| Telegram | `api.telegram.org` | 443 | TCP | Long-poll OR webhook. Webhook also needs ingress from Telegram's IP ranges to the agent's webhook URL: out of scope for the egress NetworkPolicy. |
| Discord | `discord.com`, `gateway.discord.gg` | 443 | TCP | gateway.discord.gg is the WebSocket endpoint. |
| Slack | `slack.com`, `wss-primary.slack.com` | 443 | TCP | wss-primary.slack.com is the Socket Mode endpoint. |
| WhatsApp (Meta Cloud) | `graph.facebook.com` | 443 | TCP | Provider-specific. Twilio users should replace with `api.twilio.com`. |
| Signal (chat.signal.org) | `chat.signal.org` | 443 | TCP | Self-hosted signal-cli-rest-api deployments should supplement via `spec.networking.egress`. |
| Honcho (sibling) | sibling pod selector | 8000 | TCP | In-namespace pod-selector peer, not internet. |

The operator does NOT cover ingress from those providers (Telegram, Slack webhook callbacks, etc.): surface that via `spec.networking.ingress` or a dedicated Ingress object in your cluster.
