# hermes-operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/stubbi/hermes-operator)](https://goreportcard.com/report/github.com/stubbi/hermes-operator)
[![CI](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml)

Kubernetes operator for [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent) — a Python-based self-improving multi-platform AI agent.

> **Status: alpha.** Plan 1 of 7 shipped (minimal happy path).

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

## Design

See [`docs/superpowers/specs/2026-05-12-hermes-operator-design.md`](docs/superpowers/specs/2026-05-12-hermes-operator-design.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions are licensed Apache-2.0.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.
