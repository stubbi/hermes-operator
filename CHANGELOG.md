# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0](https://github.com/stubbi/hermes-operator/releases/tag/v1.0.0) (2026-05-12)

First public release. The Kubernetes operator for
[nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent),
shipping with full feature parity to openclaw-operator v0.32 adapted to
hermes-agent's Python/uv runtime, plus hermes-specific surfaces:
multi-platform gateways (Telegram/Discord/Slack/WhatsApp/Signal), a
Honcho profile-store companion, an SSA-based `HermesSelfConfig` API for
agent-initiated mutations, and a one-shot OpenClaw → Hermes migration
path.

The v1 stability contract — API versioning policy, deprecation policy,
exhaustive condition catalogue, conversion-webhook scaffolding — is in
place from day one. See
[docs/api-versioning.md](docs/api-versioning.md) and
[docs/deprecations.md](docs/deprecations.md).

Inspired by [openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator).
Concrete lessons baked in: SSA from day one on the SelfConfig path
(openclaw #433), explicit Kubernetes defaults set in every builder
(generation-thrash regressions never shipped), finalizer mutations via
`r.Patch` rather than `r.Update` (openclaw #437), foreign-annotation
preservation (openclaw #446), zombie-process reaper (openclaw #471),
namespace-scoped RBAC opt-in (openclaw #469), ClusterRole aggregation
labels (openclaw #479), and read-only root filesystem with explicit
writable subPaths (openclaw #458).

### Highlights

- **CRDs (`hermes.agent/v1`):** `HermesInstance` (namespaced),
  `HermesSelfConfig` (namespaced, SSA-applied), `HermesClusterDefaults`
  (cluster-scoped singleton `cluster`).
- **Workload:** StatefulSet (single replica by default; opt-in HPA),
  default-deny NetworkPolicy + per-gateway allow rules, PDB
  auto-managed when `replicas > 1`, read-only root filesystem with
  writable `emptyDir`s for `/tmp` and `~/.config`.
- **Multi-platform gateways:** Telegram, Discord, Slack, WhatsApp,
  Signal — each with its own Secret reference, rotatable independently.
- **Day-2 operations:** S3-compatible backups (scheduled / on-delete /
  pre-update), declarative one-shot restore (`spec.restoreFrom`),
  OCI-registry-driven auto-update with probe-failure rollback, one-shot
  OpenClaw → Hermes migration (sibling or S3 source).
- **GitOps coexistence:** SSA on the SelfConfig path under field manager
  `hermes.agent/selfconfig`; FluxCD/Argo own the same instance for
  other fields without flap.
- **Distribution:** Helm chart, OLM bundle (OperatorHub submission),
  plain kustomize manifests, multi-arch (`amd64`+`arm64`) Cosign-signed
  images with SPDX SBOM attestation.
- **Testing:** unit, envtest, e2e (kind), conformance (negative +
  idempotency + upgrade matrix + GitOps + failure injection),
  benchmarks, gosec + Trivy, Reconcile Guard CI, Helm RBAC sync check.
- **Documentation:** [design spec](docs/superpowers/specs/2026-05-12-hermes-operator-design.md),
  [API reference](docs/api-reference.md),
  [condition catalogue](docs/conditions.md),
  [API versioning policy](docs/api-versioning.md),
  [deprecation policy](docs/deprecations.md),
  [9 worked examples](examples/),
  [Grafana dashboard](docs/grafana/),
  [public roadmap](ROADMAP.md).

### Supported Kubernetes versions

1.28, 1.29, 1.30, 1.31, 1.32.

### Known limitations / deferred items

- **`examples/` directory** will be populated with 9 worked YAML recipes
  in v1.1. The directory structure and README index are committed; the
  individual example files are a follow-up cycle.
- **Grafana dashboard library** (`docs/grafana/`) will expand to per-
  instance drilldown and per-gateway health dashboards in v1.1. The
  operator-overview dashboard is in v1.0.0.
- **OperatorHub submission** requires manual steps (submission PR to
  OperatorHub community-operators repo). The OLM bundle is committed and
  tested; the actual submission is a human-in-the-loop step after this
  release.

### Inspiration and prior art

This is a clean-room operator built specifically for hermes-agent. The
[openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator)
project — which shipped a similar lifecycle operator for OpenClaw,
evolving through v0.5 → v0.32 with substantial production feedback —
served as the reference for the *shape* of this product: which surfaces
matter, which lessons stick, and which guardrails are non-negotiable.
The hermes-specific surfaces (Python/uv runtime, multi-platform
gateways, Honcho, SSA-based SelfConfig with `profiles` action, declarative
migration importer) are new. The v1 stability contract is also new; it
is the single most important thing this operator does differently from
the v0.x grind.

For the full list of openclaw lessons that informed v1, see
[docs/superpowers/specs/2026-05-12-hermes-operator-design.md](docs/superpowers/specs/2026-05-12-hermes-operator-design.md)
§1.G3 and §7.2 ("Reconciliation rules").
