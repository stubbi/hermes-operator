# HermesSelfConfig Guide

`HermesSelfConfig` lets a hermes-agent mutate its own `HermesInstance` at runtime: adding skills, patching config, injecting environment variables, seeding workspace files, and writing Honcho profile snapshots: all through Server-Side Apply with a dedicated field manager so GitOps tooling is never disturbed.

## Prerequisites

- A `HermesInstance` with `spec.selfConfigure.enabled: true` and at least one entry in `spec.selfConfigure.allowedActions`.
- The operator (Plan 4+) running in the cluster.

## Enabling self-configuration on an instance

```yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: my-agent
  namespace: default
spec:
  selfConfigure:
    enabled: true
    allowedActions:
      - skills
      - config
      - envVars
      - workspaceFiles
      - profiles
    protectedKeys:
      - "provider.apiKey"
      - "*.secret*"
      - "gateways.telegram.token"
```

`protectedKeys` are gobwas/glob patterns matched against the dotted JSON path of any `patchConfig` key. A patch that touches a protected path is rejected with `phase=Denied` and reason `ProtectedPath`.

## Creating a HermesSelfConfig

### Add a skill

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: my-agent-add-finance-skill
  namespace: default
spec:
  instanceRef: my-agent
  addSkills:
    - source: git+https://github.com/example/finance-skill@v1.2.0
      version: "v1.2.0"
```

The controller appends the skill to `HermesInstance.spec.skills` using SSA with field manager `hermes.agent/selfconfig`. The list-map key is `source`, so a second apply with the same source is idempotent.

### Patch the agent config

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: my-agent-config-patch
  namespace: default
spec:
  instanceRef: my-agent
  patchConfig:
    timezone: "Europe/Berlin"
    max_tokens: 4096
```

`patchConfig` is a JSON merge patch (RFC 7396) written to the workspace ConfigMap under key `selfconfig.yaml`. The agent startup script layers it onto `~/.hermes/config.yaml`. Protected keys matched by `spec.selfConfigure.protectedKeys` on the parent instance are blocked before the write.

### Inject environment variables

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: my-agent-env
  namespace: default
spec:
  instanceRef: my-agent
  addEnvVars:
    - name: FINANCE_TZ
      value: "America/New_York"
    - name: API_KEY
      valueFrom:
        secretKeyRef:
          name: finance-api-secret
          key: api-key
```

Environment variable names must match `^[A-Za-z_][A-Za-z0-9_]*$`. `value` and `valueFrom` are mutually exclusive.

### Seed workspace files

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: my-agent-workspace
  namespace: default
spec:
  instanceRef: my-agent
  addWorkspaceFiles:
    - path: notes/finance/2026-q1.md
      content: |
        # Q1 2026 Finance Notes
        Initial seed from SelfConfig.
```

Nested paths (containing `/`) are encoded as `__`-separated ConfigMap keys internally and decoded back to filesystem paths by the runtime-init container.

### Write a Honcho profile snapshot

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: my-agent-profile
  namespace: default
spec:
  instanceRef: my-agent
  addProfileSnapshot:
    profileID: "user-42"
    data: '{"summary": "Prefers concise answers", "topics": ["finance", "tax"]}'
```

Requires `HermesInstance.spec.profileStore.honcho.enabled=true`. The controller dispatches a batch Job that writes the payload to `/data/snapshots/<profileID>/<RFC3339-timestamp>.json` on the Honcho PVC.

## Checking the status

```bash
kubectl describe hermesselfconfig my-agent-add-finance-skill
```

Example output:

```
Name:         my-agent-add-finance-skill
Namespace:    default
API Version:  hermes.agent/v1
Kind:         HermesSelfConfig
Spec:
  Instance Ref:  my-agent
  Add Skills:
    Source:   git+https://github.com/example/finance-skill@v1.2.0
    Version:  v1.2.0
Status:
  Applied At:           2026-05-12T14:23:01Z
  Applied Fields:
    spec.skills[source=git+https://github.com/example/finance-skill@v1.2.0]
  Observed Generation:  1
  Phase:                Applied
  Conditions:
    Last Transition Time:  2026-05-12T14:23:01Z
    Message:               SSA writes applied successfully
    Observed Generation:   1
    Reason:                SSASuccess
    Status:                True
    Type:                  Applied
```

If the request is denied:

```
Status:
  Deny Reason:          selfconfig disabled on parent
  Observed Generation:  1
  Phase:                Denied
  Conditions:
    Last Transition Time:  2026-05-12T14:20:15Z
    Message:               selfconfig disabled on parent
    Observed Generation:   1
    Reason:                PolicyViolation
    Status:                True
    Type:                  Denied
```

## Watching field ownership

Use `yq` to inspect which field manager owns which paths on the parent instance:

```bash
kubectl get hermesinstance my-agent -o json \
  | yq '.metadata.managedFields[] | select(.manager == "hermes.agent/selfconfig")'
```

This shows only the fields the SelfConfig controller has written. FluxCD, Argo CD, or your own `kubectl apply` still own their respective paths: no conflicts unless two managers claim the same path.

## Forcing ownership

If a conflict is detected (another manager already owns a path you want to write), the controller marks the SelfConfig `Denied` with reason `SSAConflict`. To override:

```yaml
metadata:
  annotations:
    hermes.agent/force-ownership: "true"
```

Use with care: force-ownership removes the conflicting manager's claim on that path.

## Policy reference

| Parent setting | Effect |
|---|---|
| `selfConfigure.enabled: false` (or unset) | All SelfConfigs are `Denied` with reason `selfconfig disabled on parent`. |
| `allowedActions` omits `skills` | `addSkills` is blocked with reason `PolicyViolation`. |
| `allowedActions` omits `config` | `patchConfig` is blocked. |
| `allowedActions` omits `envVars` | `addEnvVars` is blocked. |
| `allowedActions` omits `workspaceFiles` | `addWorkspaceFiles` is blocked. |
| `allowedActions` omits `profiles` | `addProfileSnapshot` is blocked. |
| `protectedKeys` matches a `patchConfig` path | That specific key is blocked with reason `ProtectedPath`. |

## GitOps coexistence

`HermesSelfConfig` is designed for zero-conflict coexistence with FluxCD and Argo CD:

- The field manager `hermes.agent/selfconfig` is distinct from `flux-system` and `argocd-application-controller`.
- Skills appended via SSA use the list-map key `source`: Flux's own `spec.skills` entries use the same list but different source values, so they coexist as separate list items.
- The controller never touches `spec.image`, `spec.storage`, `spec.gateways`, or any other field not listed in the SelfConfig spec.

For the envtest proof of this coexistence, see `internal/controller/hermesselfconfig_ssa_test.go`. The full conformance suite (real kind cluster, multiple concurrent writers, Kubernetes 1.28-1.32) lands in Plan 6: see `test/conformance/gitops_coexistence_test.go`.
