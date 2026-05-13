# Hermes Operator — Roadmap

> Public, non-binding roadmap. Things on this list may shift between minor
> releases. The v1 API stability contract in
> [`docs/api-versioning.md`](docs/api-versioning.md) applies to everything
> shipped: roadmap items do not introduce breaking changes to existing
> `hermes.agent/v1` surfaces.

## Shipped (v1.0.0)

All design-spec §1.G1 ("Full feature parity with openclaw-operator v0.32
adapted to hermes-agent's Python/uv runtime") items are in v1.0.0:

- `HermesInstance` CRD with the full spec surface in design §4: image,
  config, workspace, resources, security, storage, networking,
  observability, availability, probes, backup, restoreFrom, runtime,
  gateways (Telegram, Discord, Slack, WhatsApp, Signal), profileStore
  (Honcho), ollama, webTerminal, tailscale, autoUpdate, selfConfigure,
  migration (`fromOpenClaw`), scheduling, initContainers, sidecars,
  extraVolumes, envFrom, env, suspended.
- `HermesSelfConfig` CRD with SSA-driven application, namespace-scoped
  policy enforcement against the parent instance's `selfConfigure.protectedKeys`,
  and `Applied`/`Denied`/`Pending` condition phases.
- `HermesClusterDefaults` cluster-scoped singleton with the defaulting
  webhook that fills `nil` fields only (never overrides).
- StatefulSet-based runtime with explicit Kubernetes defaults set in every
  builder (no generation thrash on update — Reconcile Guard CI enforces).
- Default-deny NetworkPolicy + per-gateway allow rules.
- S3-compatible scheduled / on-delete / pre-update backups; declarative
  one-shot restore via `spec.restoreFrom`.
- OCI-registry-driven auto-update with channel-pinned polling, pre-update
  backup, and probe-failure rollback.
- One-shot OpenClaw → Hermes migration (from a sibling `OpenClawInstance`
  or an S3 backup snapshot).
- GitOps coexistence: SSA on the SelfConfig path means FluxCD/Argo can own
  the same `HermesInstance` for non-SelfConfig fields without flap.
- Distribution: Helm chart, OLM bundle (OperatorHub submission), plain
  kustomize manifests, Cosign-signed multi-arch (`amd64`+`arm64`) images
  with SBOM attestation, GoReleaser-managed release pipeline.
- Testing: unit, envtest, e2e (kind), conformance (negative + idempotency
  + upgrade matrix + GitOps + failure injection), benchmarks, gosec/Trivy,
  Reconcile Guard, Helm RBAC sync check.
- Documentation: design spec, API reference, condition catalogue
  (`docs/conditions.md`), API versioning policy (`docs/api-versioning.md`),
  deprecation policy (`docs/deprecations.md`), 9 worked examples under
  `examples/`, Grafana dashboard under `docs/grafana/`.

## Planned for v1.1+

The items below are committed in principle but not in shipping order. None
introduce breaking changes to `hermes.agent/v1`.

### `kubectl-hermes` plugin (krew)

A `kubectl` plugin (`kubectl hermes`) installable via
[krew](https://krew.sigs.k8s.io/). Initial commands:

- `kubectl hermes diag <instance>` — pull conditions + recent events +
  pod status + recent logs into a single triage report.
- `kubectl hermes shell <instance>` — `kubectl attach` shortcut for the
  optional web-terminal sidecar.
- `kubectl hermes snapshot <instance>` — manual one-shot backup outside
  the schedule.
- `kubectl hermes migrate-from-openclaw <openclaw-instance>` — generate a
  starter `HermesInstance` YAML with the right `migration.fromOpenClaw`
  sub-spec for the source.

Mirrors the shape of `kubectl-openclaw`, with hermes-specific commands.

### Grafana dashboard library

The single dashboard shipped in v1.0.0 (`docs/grafana/hermes-operator-overview.json`)
becomes a library: per-instance drilldown, per-gateway health, backup-
health, auto-update-health, and a cost dashboard (the latter requires the
"AI provider health monitoring" item below to publish metrics).

Distributed via the [grafana.com dashboard catalog](https://grafana.com/grafana/dashboards/)
under the `hermes-operator` org.

### AI provider health monitoring

The agent already knows which provider (OpenAI / Anthropic / local Ollama)
served each request and how long it took. v1.1 surfaces this through:

- New status sub-tree `status.providers[].*` on `HermesInstance` with last
  success/failure and latency p50/p95.
- Prometheus metrics `hermes_provider_request_seconds`,
  `hermes_provider_request_failures_total`, labelled by `provider`,
  `model`, and `instance`.
- A new optional condition `ProvidersHealthy` (non-breaking addition).

This is the foundation for the cost item below.

### Cost recommendations

Once provider metrics exist, the operator can flag obvious cost wins on a
periodic basis as Kubernetes Events on the instance:

- "Provider X served Y% of requests in the past 7 days at $Z/request;
  consider switching the schedule to local Ollama (sidecar already
  enabled)."
- "Provider X is timing out > 1%/h; consider a fallback in spec.providers
  ordering."

Recommendations are advisory — the operator never auto-changes provider
config. Cost data needs an explicit `spec.observability.cost.enabled=true`
opt-in for privacy reasons.

## Future (not on a release schedule)

Items the design spec calls out under §12 but does not commit to a
specific minor:

### Multi-cluster federation

A `HermesInstanceMirror` CRD that, when applied in a "primary" cluster,
keeps a 1:1 mirror running in N "secondary" clusters with automatic
failover. Non-trivial — depends on a clean way to share PVC contents
(velero-based or via the existing backup/restore pipeline).

Design constraint: must not require a control plane outside Kubernetes.
The implementation should be a self-contained controller that reconciles
mirrors using nothing but Kubernetes APIs (kube-apiserver in each cluster
+ a shared backup target).

### Scale-from-zero on incoming webhook event

For instances configured with `spec.suspended=true`, an external event
(a Telegram message, a Slack mention, a cron-style schedule) wakes the
instance, lets it process the work, and re-suspends after an idle window.

Requires a tiny "wake gateway" service (probably the same per-gateway
pods, configurable to keep running while the main agent scales to zero
and pass the event through on wake).

Note this is design §NG2 explicitly *not* about Modal/Daytona-style
hibernation — it stays Kubernetes-native.

### Per-gateway operator sub-modes

Splitting the gateways section into a separate CRD (`HermesGateway`)
under the same API group so a single hermes-agent instance can be
fronted by multiple gateway pods scaled independently. This is non-
breaking for `HermesInstance` (`spec.gateways` continues to work), it
adds a new CRD alongside.

## Non-goals

From design spec §1 — these are deliberately not on the roadmap:

- **Multi-cluster federation as a hard product feature** beyond the
  "future" item above. Single-cluster control loop only by default.
- **Modal / Daytona "hibernation" integration.** Kubernetes-native
  scale-to-zero (`spec.suspended` + the scale-from-zero future item) is
  the equivalent.
- **Generic "AgentInstance" operator** that manages other AI agents
  besides hermes-agent. Premature abstraction.
- **Public OpenClaw → Hermes data conversion guarantees** beyond what
  hermes-agent's own importer provides.

## How items move

- An item appears on this roadmap when there is a sketch in
  `docs/design-notes/` and at least one issue tagged `roadmap`.
- It moves to "Planned" when a milestone is created and a target minor
  is set in the milestone description.
- It moves to "Shipped" when it lands on `main` and is in the
  `CHANGELOG.md`.

## How to propose a new item

Open a GitHub Discussion under "Ideas" with the shape:

- **What:** one sentence.
- **Why:** the user problem.
- **Non-goal callout:** confirm it doesn't conflict with the non-goals
  list above.
- **API impact:** which surfaces it touches, expected non-breaking-vs-
  breaking footprint.

A maintainer either accepts it onto the roadmap with a milestone, or
adds it under "Future" with no milestone, or declines with a comment.
