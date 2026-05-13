# Hermes Operator

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/stubbi/hermes-operator"><img src="https://goreportcard.com/badge/github.com/stubbi/hermes-operator" alt="Go Report Card"></a>
  <a href="https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml"><img src="https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
  <a href="https://github.com/stubbi/hermes-operator/actions/workflows/e2e.yaml"><img src="https://github.com/stubbi/hermes-operator/actions/workflows/e2e.yaml/badge.svg" alt="E2E"></a>
  <a href="https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml"><img src="https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml/badge.svg" alt="Conformance"></a>
  <a href="https://github.com/stubbi/hermes-operator/releases/latest"><img src="https://img.shields.io/github/v/release/stubbi/hermes-operator" alt="Release"></a>
  <a href="#supported-kubernetes-versions"><img src="https://img.shields.io/badge/kubernetes-1.28--1.32-blue" alt="Kubernetes versions"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/stubbi/hermes-operator" alt="Go version"></a>
  <a href="https://api.securityscorecards.dev/projects/github.com/stubbi/hermes-operator"><img src="https://api.securityscorecards.dev/projects/github.com/stubbi/hermes-operator/badge" alt="OpenSSF Scorecard"></a>
  <a href="https://artifacthub.io/packages/search?repo=hermes-operator"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/hermes-operator" alt="Artifact Hub"></a>
</p>

Kubernetes operator for [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent): a Python-based self-improving multi-platform AI agent. Declarative spec,
opinionated security defaults, S3 backups, OCI-registry auto-update,
SSA-based GitOps coexistence, and a one-shot migration path from
openclaw-operator.

`hermes-operator` ships as v1.0.0 with [v1 stability commitments](docs/api-versioning.md)
in place from day one: no v0.x grind.

> Inspired by [openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator);
> openclaw lessons #437, #446, #433, #471, #479, #458, #469 (and many more)
> informed concrete guardrails baked into v1. See
> [docs/superpowers/specs/2026-05-12-hermes-operator-design.md](docs/superpowers/specs/2026-05-12-hermes-operator-design.md) §1.G3.

## Quickstart

```bash
# 1. Install the CRDs and operator via Helm.
helm repo add hermes https://stubbi.github.io/hermes-operator
helm install hermes-operator hermes/hermes-operator \
  -n hermes-operator --create-namespace

# 2. Apply a minimal instance.
kubectl apply -n agents -f - <<'YAML'
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: my-hermes
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: "1.4.2"
  storage:
    persistence:
      enabled: true
      size: 10Gi
YAML

# 3. Watch it converge.
kubectl get hi -n agents -w
# NAME        READY   PHASE   IMAGE                                AGE
# my-hermes   True    Ready   ghcr.io/stubbi/hermes-agent:1.4.2    30s
```

For more involved scenarios, see [`examples/`](examples/).

## Architecture

```mermaid
flowchart LR
  subgraph User
    GitOps[FluxCD / Argo]
    Kubectl[kubectl apply]
  end

  subgraph ControlPlane["Kubernetes control plane"]
    APIServer[(kube-apiserver)]
    HInstance["HermesInstance"]
    HSelfConfig["HermesSelfConfig"]
    HClusterDefaults["HermesClusterDefaults<br/>(singleton)"]
  end

  subgraph Operator["hermes-operator pod"]
    DefaulterWebhook[Defaulter]
    ValidatorWebhook[Validator]
    InstanceCtrl[HermesInstance<br/>controller]
    SelfConfigCtrl[HermesSelfConfig<br/>controller<br/>SSA: hermes.agent/selfconfig]
    ClusterDefaultsCtrl[ClusterDefaults<br/>controller]
  end

  subgraph Workload["agent workload (per HermesInstance)"]
    STS[StatefulSet]
    Svc[Service]
    NetPol[NetworkPolicy default-deny]
    PVC[PVC ~/.hermes]
    Honcho[Honcho Deploy<br/>profile store]
    CronJob[Backup CronJob]
  end

  S3[(S3-compatible<br/>backup target)]
  OCI[(OCI registry<br/>hermes-agent tags)]

  GitOps --> APIServer
  Kubectl --> APIServer
  APIServer <-->|admission| DefaulterWebhook
  APIServer <-->|admission| ValidatorWebhook
  APIServer --> HInstance
  APIServer --> HSelfConfig
  APIServer --> HClusterDefaults
  HInstance --> InstanceCtrl
  HSelfConfig --> SelfConfigCtrl
  HClusterDefaults --> ClusterDefaultsCtrl
  InstanceCtrl --> STS
  InstanceCtrl --> Svc
  InstanceCtrl --> NetPol
  InstanceCtrl --> PVC
  InstanceCtrl --> Honcho
  InstanceCtrl --> CronJob
  SelfConfigCtrl -.SSA patch.-> HInstance
  CronJob --> S3
  InstanceCtrl -.poll.-> OCI
```

The agent runs as a StatefulSet (single replica by default) under a default-
deny NetworkPolicy. The `HermesSelfConfig` controller uses Server-Side Apply
under field manager `hermes.agent/selfconfig`, so FluxCD/Argo can own the
parent `HermesInstance` for other fields without flap. `HermesClusterDefaults`
is a cluster-scoped singleton (name **must** be `cluster`) that fills `nil`
fields only: explicit values on the instance always win.

## Features

| Area | Feature | Notes |
|---|---|---|
| **Declarative** | Single `HermesInstance` CR drives the whole stack | StatefulSet, Service, PVC, NetworkPolicy, ConfigMap, PDB, HPA, ServiceMonitor, Honcho deploy, backup CronJob: all owned and reconciled. |
| **Declarative** | `HermesClusterDefaults` for cluster-wide defaults | Defaulting webhook fills `nil` fields only. |
| **Adaptive** | `HermesSelfConfig` for audited agent-initiated mutations | SSA under field manager `hermes.agent/selfconfig`. Policy-gated by `spec.selfConfigure.protectedKeys`. |
| **Adaptive** | OCI-registry-driven auto-update | Channel-pinned polling, pre-update backup, probe-failure rollback. |
| **Secure** | Default-deny NetworkPolicy + per-gateway allow rules | Derived from `spec.gateways` and `spec.networking.egress`. |
| **Secure** | Read-only root filesystem | Writable `emptyDir`s for `/tmp` and `~/.config` subPaths. |
| **Secure** | Per-CRD validating + defaulting webhooks | Plus warnings on unknown config keys and unresolvable gateway tokens. |
| **Secure** | RBAC aggregation labels | `kubectl auth can-i create hermesinstances --as=jane` works out of the box. |
| **Secure** | Image signing + SBOM | Cosign keyless OIDC, SPDX SBOM on every release. |
| **Observable** | Prometheus metrics + ServiceMonitor | Per-controller, per-instance, per-subsystem. `metrics.secure` consistent. |
| **Observable** | [Grafana dashboard](docs/grafana/) | Ships as JSON. Variables: `namespace`, `instance`. |
| **Observable** | Exhaustive [condition catalogue](docs/conditions.md) | Every condition × every reason code, documented and stable. |
| **Multi-platform** | Telegram / Discord / Slack / WhatsApp / Signal gateways | First-class `spec.gateways.*` sections, secret-rotation-friendly. |
| **Python runtime** | `uv`-installable agent runtime | Init container runs `uv sync` against a lockfile bundled in the agent image. |
| **Python runtime** | FFmpeg + ripgrep available out of the box | Hard dependencies of hermes-agent. |
| **Scalable** | Optional HPA via `spec.availability.hpa` | StatefulSet retained for identity through restarts. |
| **Scalable** | Optional `topologySpreadConstraints` | Sane defaults plus `spec.availability.topologySpreadConstraints` override. |
| **Resilient** | PodDisruptionBudget auto-managed when `replicas > 1` | |
| **Resilient** | Finalizer-driven backup-on-delete | `r.Patch` (JSON patch) for finalizer mutations, never `r.Update`. |
| **Resilient** | Zombie-process reaper | `tini` as PID 1; `shareProcessNamespace: false` by default. |
| **Backup / Restore** | S3-compatible backups | Scheduled, on-delete, pre-update. `tar.zst` snapshots + `meta.json`. |
| **Backup / Restore** | Declarative one-shot restore | `spec.restoreFrom` is immutable once applied. |
| **Migration** | One-shot OpenClaw → Hermes migration | From sibling `OpenClawInstance` or S3 backup. Uses hermes-agent's importer. |
| **Profile store** | Optional Honcho companion | Deployment + Service + PVC + secret, fully managed. |
| **Gateway auth** | Per-platform `secretRef` for tokens | Rotate independently, audited via webhook warnings. |
| **Cloud-native** | Helm chart, OLM bundle, plain kustomize manifests | All three are first-class. CRDs templated under the Helm chart. |
| **Cloud-native** | Multi-arch (`amd64`+`arm64`), Cosign-signed, SBOM-attested | |
| **GitOps** | SSA-based SelfConfig coexists with Argo/Flux | No flap on shared instances. |
| **Stability** | v1.0 ships with [versioning](docs/api-versioning.md) + [deprecation](docs/deprecations.md) policies | Conversion-webhook scaffolding in place for future v2. |

## Worked example: self-configure

The agent can persist a learned skill, env var, config patch, workspace file,
or Honcho profile by creating a `HermesSelfConfig` in its namespace. The
operator validates against the parent instance's `selfConfigure.protectedKeys`
allowlist and applies via SSA:

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: install-finance-skill
  namespace: agents
spec:
  instanceRef: my-hermes
  addSkills:
    - source: "git+https://github.com/foo/finance-skill@v1.2.0"
  patchConfig:
    schedules:
      morning-brief: "0 8 * * *"
  addEnvVars:
    - name: FINANCE_TZ
      value: Europe/Berlin
```

Apply, then watch:

```bash
kubectl get hsc -n agents
# NAME                      PHASE     INSTANCE    AGE
# install-finance-skill     Applied   my-hermes   3s
```

The audit trail lives in `kubectl describe hsc install-finance-skill` and on
the instance via the per-field SSA field manager
`hermes.agent/selfconfig`: `kubectl get hi my-hermes -o jsonpath='{.metadata.managedFields}'`
shows exactly which fields the agent owns vs. Flux owns vs. you own.

See [`examples/`](examples/) for end-to-end recipes.

## Supported Kubernetes versions

| Operator | Kubernetes |
|---|---|
| v1.x | 1.28, 1.29, 1.30, 1.31, 1.32 |

We drop the oldest k8s minor when Kubernetes EOLs it, on the *next* operator
minor release. Patch releases never change the supported matrix.

## Distribution

| Channel | What |
|---|---|
| Helm | `helm install hermes-operator hermes/hermes-operator` |
| OLM / OperatorHub | `kubectl operator install hermes-operator` |
| Plain manifests | `kubectl apply -f https://github.com/stubbi/hermes-operator/releases/latest/download/install.yaml` |
| Container image | `ghcr.io/stubbi/hermes-operator:v1.0.0` (multi-arch, Cosign-signed, SBOM attested) |

## Documentation

- [Design spec](docs/superpowers/specs/2026-05-12-hermes-operator-design.md): the canonical product/architecture doc.
- [API reference](docs/api-reference.md): every field on every CR.
- [Condition catalogue](docs/conditions.md): every status condition, reason code, troubleshooting hint.
- [API versioning policy](docs/api-versioning.md): what is and is not a breaking change.
- [Deprecation policy](docs/deprecations.md): the 3-step flow + active deprecations.
- [Roadmap](ROADMAP.md): shipped, planned, future, non-goals.
- [Examples](examples/): 9 worked YAML recipes.
- [Grafana dashboard](docs/grafana/): operator-overview dashboard JSON.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Pull requests follow
[Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
`docs:`, `ci:`, `chore:`, `refactor:`, `test:`); release-please drives the
release-PR loop from `feat:`/`fix:`.

## Security

See [`SECURITY.md`](SECURITY.md). Report vulnerabilities via the GitHub
security advisory flow; do not file public issues for security bugs.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
