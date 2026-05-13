# Supported Kubernetes Versions

## Version Support Matrix

| Kubernetes | Support Status | envtest CI | Notes |
|------------|---------------|------------|-------|
| 1.32       | Supported      | Yes        | Latest stable |
| 1.31       | Supported      | Yes        |       |
| 1.30       | Supported      | Yes        |       |
| 1.29       | Supported      | Yes        |       |
| 1.28       | Supported      | Yes        | Minimum supported |
| < 1.28     | Not supported  | No         | Use hermes-operator < v1.0.0 |

## EOL Policy

hermes-operator supports the **five most recent minor releases** of Kubernetes at any given time, tracking the upstream [Kubernetes release cadence](https://kubernetes.io/releases/patch-releases/).

When a new Kubernetes minor version reaches GA:
- It is added to the CI matrix immediately.
- The oldest supported minor is moved to "best-effort" for one release cycle, then dropped.

## v1 Commitments

For the **v1.0.0** release of hermes-operator:

- Kubernetes 1.28–1.32 are validated via envtest in CI on every PR and main push.
- API compatibility: `HermesInstance`, `HermesClusterDefaults`, and `HermesSelfConfig` v1 APIs will not have breaking changes within the v1.x line.
- Deprecation notices will be given at least one minor release before removal.
- Security patches will be backported to the most recent minor release if the current minor is not yet 30 days old.

## Checking Your Version

```bash
kubectl get deployment -n hermes-operator-system hermes-operator-controller-manager \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```
