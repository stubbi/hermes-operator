# Full-featured `HermesInstance`

A deliberately maximal example: every top-level sub-spec is exercised at
least once. **Do not copy this into production as-is**: it is for
discovery. Start from [`minimal/`](../minimal/) and add only what you
need.

## Prerequisites

This example references several Secrets that you must create first:

```bash
kubectl create namespace agents

# Gateway tokens (placeholder values: replace with real ones).
kubectl create secret generic hermes-telegram \
  -n agents --from-literal=token=REPLACE_WITH_TELEGRAM_BOT_TOKEN
kubectl create secret generic hermes-discord \
  -n agents --from-literal=token=REPLACE_WITH_DISCORD_BOT_TOKEN

# S3 backup credentials.
kubectl create secret generic hermes-s3-creds \
  -n agents \
  --from-literal=accessKey=REPLACE \
  --from-literal=secretKey=REPLACE

# Honcho secret.
kubectl create secret generic hermes-honcho \
  -n agents --from-literal=apiKey=REPLACE

# Tailscale auth key (must be REUSABLE + EPHEMERAL; see the main README).
kubectl create secret generic hermes-tailscale \
  -n agents --from-literal=authKey=REPLACE_WITH_TS_AUTH_KEY

# Image pull secret (if your registry needs auth).
kubectl create secret docker-registry ghcr-pull \
  -n agents \
  --docker-server=ghcr.io \
  --docker-username=YOUR_GITHUB_USERNAME \
  --docker-password=YOUR_GITHUB_PAT
```

## Apply

```bash
kubectl apply -n agents -f hermesinstance.yaml
```

## What this exercises

| Sub-spec | What |
|---|---|
| `image` | Pinned tag + image pull secret. |
| `config` | Raw inline + merge mode. |
| `workspace` | Two seeded files. |
| `resources` | Explicit requests + limits. |
| `security` | Pod + container security context, SA annotation (IRSA). |
| `storage` | 50Gi GP3 PVC. |
| `networking` | Ingress + NetworkPolicy egress allow-list. |
| `observability` | Metrics + ServiceMonitor. |
| `availability` | PDB + HPA + topology spread. |
| `probes` | Custom liveness/readiness. |
| `backup` | Scheduled + on-delete + pre-update, with history limit. |
| `runtime` | Pinned Python + extra apt + extra pip. |
| `gateways` | Telegram + Discord. |
| `profileStore` | Honcho with persistence. |
| `tailscale` | Serve mode: gateway exposed on the tailnet with a Tailscale TLS cert. |
| `autoUpdate` | Channel-pinned with rollback. |
| `selfConfigure` | Enabled with a strict `protectedKeys`. |
| `scheduling` | Node selector + toleration. |
| `initContainers` | One custom init. |
| `sidecars` | One custom sidecar. |
| `extraVolumes` / `extraVolumeMounts` | Extra hostPath for tracing. |
| `envFrom` / `env` | A configMapRef + a literal env var. |
| `suspended` | Set to `false` (default): flip to `true` to scale to zero. |

### Planned (not yet in the v1 CRD)

A first-class `spec.webTerminal` field is on the roadmap, tracked in
[#42](https://github.com/stubbi/hermes-operator/issues/42). It is not yet
exposed on the v1 CRD, so setting it has no effect (the apiserver prunes
unknown fields). Until it lands, run a web-terminal container through the
generic `spec.sidecars` escape hatch.

The corresponding conditions on `kubectl describe hi full-featured` are:
`Ready`, `StorageReady`, `ConfigReady`, `SecretsReady`, `NetworkPolicyReady`,
`RBACReady`, `GatewayReady`, `ProfileStoreReady`, `TailscaleReady`,
`BackupReady`, `AutoUpdated`, `WebhookReady`. (`RestoreApplied`, `MigrationCompleted`, and
`AutoUpdateRolledBack` are absent because nothing triggers them.)
