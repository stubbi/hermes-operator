# hermes-operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/stubbi/hermes-operator)](https://goreportcard.com/report/github.com/stubbi/hermes-operator)
[![CI](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml)
[![Conformance](https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/conformance.yaml)
[![Release](https://github.com/stubbi/hermes-operator/actions/workflows/release.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/release.yaml)

Kubernetes operator for [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent) — a Python-based self-improving multi-platform AI agent.

> **Status: alpha.** Plans 1–4 of 7 shipped (minimal happy path + full spec + runtime/gateways/profileStore + HermesSelfConfig SSA).

## Features

| Feature | Status | Plan |
|---|---|---|
| Reconcile HermesInstance (PVC, ConfigMap, Service, StatefulSet) | ✅ v1.0 | Plan 1 |
| Full HermesInstance spec (resources, security, scheduling, ...) | ✅ v1.0 | Plan 2 |
| Defaulting webhook (HermesClusterDefaults singleton) | ✅ v1.0 | Plan 2 |
| Validating webhook (required / immutable / one-of) | ✅ v1.0 | Plan 2 |
| NetworkPolicy (deny-all baseline + selective allow) | ✅ v1.0 | Plan 2 |
| Per-instance RBAC (SA + Role + RoleBinding) | ✅ v1.0 | Plan 2 |
| PodDisruptionBudget | ✅ v1.0 | Plan 2 |
| HorizontalPodAutoscaler | ✅ v1.0 | Plan 2 |
| Ingress (provider-aware annotations) | ✅ v1.0 | Plan 2 |
| Prometheus ServiceMonitor + PrometheusRule | ✅ v1.0 | Plan 2 |
| `spec.suspended` scale-to-zero | ✅ v1.0 | Plan 2 |
| cert-manager-driven webhook TLS | ✅ v1.0 | Plan 2 |
| Python runtime + uv lockfile (init containers for `uv sync`, extra apt/pip) | ✅ v1.0 | Plan 3 |
| Multi-platform gateways (Telegram, Discord, Slack, WhatsApp, Signal) | ✅ v1.0 | Plan 3 |
| Honcho profile store (sibling Deployment+Service+PVC+NP, env-injected) | ✅ v1.0 | Plan 3 |
| Self-configure | Agent-driven mutations via `HermesSelfConfig`. Server-Side Apply with field manager `hermes.agent/selfconfig` lets FluxCD/Argo co-own the instance. Allowlisted action categories: `skills`, `config`, `envVars`, `workspaceFiles`, `profiles`. Protected paths matched by glob. | ✅ v1.0 | Plan 4 |
| S3-compatible backups (scheduled, on-delete, pre-update) | ✅ v1.0 | Plan 5 |
| Declarative one-shot restore (`spec.restoreFrom`) | ✅ v1.0 | Plan 5 |
| OCI-registry auto-update with rollback | ✅ v1.0 | Plan 5 |
| OpenClaw → Hermes one-shot migration | ✅ v1.0 | Plan 5 |
| OLM bundle + GoReleaser + conformance suite | ⏳ pending | Plan 6 |

## Quick start

```bash
helm install hermes-operator charts/hermes-operator -n hermes-system --create-namespace

kubectl apply -f - <<EOF
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: demo
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
EOF

kubectl get hi
```

## Day-2 Operations

- [Backup & Restore](docs/backup-restore.md)
- [Auto-Update](docs/autoupdate.md)
- [OpenClaw → Hermes Migration](docs/migration.md)
- [Snapshot Format Reference](docs/backup-format.md)

The operator implements three trigger paths for backup (scheduled, on-delete, pre-update), declarative one-shot restore with terminal immutability, OCI-registry auto-update with semver channels and probe-failure rollback, and a one-shot OpenClaw → Hermes migration init container with both in-cluster-ref and S3-backup source modes.

## Design

See [`docs/superpowers/specs/2026-05-12-hermes-operator-design.md`](docs/superpowers/specs/2026-05-12-hermes-operator-design.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions are licensed Apache-2.0.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.
