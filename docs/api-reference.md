# API Reference

> **Source of truth:** Go types in `api/v1/`. This document is the human-readable summary; the CRD YAMLs under `config/crd/bases/` and the chart's `templates/crds/` are the machine truth.

## Table of Contents

- [HermesInstance](#hermesinstance)
  - [spec.image](#specimage)
  - [spec.config](#specconfig)
  - [spec.workspace](#specworkspace)
  - [spec.resources](#specresources)
  - [spec.security](#specsecurity)
  - [spec.storage](#specstorage)
  - [spec.networking](#specnetworking)
  - [spec.observability](#specobservability)
  - [spec.availability](#specavailability)
  - [spec.probes](#specprobes)
  - [spec.scheduling](#specscheduling)
  - [spec.selfConfigure](#specSelfconfigure)
  - [spec.initContainers, spec.sidecars, spec.extraVolumes, spec.extraVolumeMounts](#specextras)
  - [spec.env, spec.envFrom, spec.skills](#specenv)
  - [spec.suspended](#specsuspended)
  - [status](#hermesinstance-status)
- [HermesClusterDefaults](#hermesclusterdefaults)
  - [spec.image](#hcd-specimage)
  - [spec.registry](#hcd-specregistry)
  - [spec.storage](#hcd-specstorage)
  - [spec.security](#hcd-specsecurity)
  - [spec.observability](#hcd-specobservability)
  - [spec.networking](#hcd-specnetworking)
  - [spec.resources](#hcd-specresources)
  - [status](#hermesclusterdefaults-status)
- [HermesSelfConfig](#hermesselfconfig)

---

## HermesInstance

**API group / version:** `hermes.agent/v1`
**Kind:** `HermesInstance`
**Scope:** Namespaced
**Short names:** `hi`, `hermes`
**Categories:** `hermes`, `agents`

HermesInstance describes a single hermes-agent deployment backed by a StatefulSet and a PVC. The controller reconciles all subsystems (ConfigMap, Service, StatefulSet, NetworkPolicy, RBAC, PDB, HPA, Ingress, ServiceMonitor, PrometheusRule) and reports readiness via `.status.conditions`.

### spec.image

Selects the OCI image for the hermes-agent container.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.image.repository` | `string` | `ghcr.io/stubbi/hermes-agent` | Container image repository. |
| `spec.image.tag` | `string` | `latest` | Image tag. |
| `spec.image.pullPolicy` | `string` (enum) | `IfNotPresent` | Image pull policy. Allowed values: `Always`, `IfNotPresent`, `Never`. |

### spec.config

Supplies the body of `~/.hermes/config.yaml`. Exactly one of `raw` or `configMapRef` should be set; the validating webhook rejects both being unset and emits a warning when both are set with `mergeMode` unset.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.config.raw` | `RawConfig` (inline YAML, `runtime.RawExtension`) | `nil` | Inline YAML body of `config.yaml`. Users may write structured YAML directly in the manifest without escaping. |
| `spec.config.configMapRef` | `LocalObjectReference` | `nil` | References a ConfigMap in the same namespace whose `config.yaml` key holds the body. |
| `spec.config.mergeMode` | `string` (enum) | `replace` | Controls combination when both `raw` and `configMapRef` are set. `replace` — `raw` replaces the ConfigMap content entirely. `merge` — deep YAML merge; `raw` wins on key conflicts. |

### spec.workspace

Seeds initial files and directories into `~/.hermes` on first start. Nested paths are supported (e.g. `notes/finance/2026.md`); the workspace ConfigMap encodes nested paths using `__` as separator, which the runtime-init container (Plan 3) decodes back to filesystem paths.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.workspace.initialFiles` | `[]WorkspaceFile` (listType=map, listMapKey=path) | `[]` | Files to seed. Each entry has a `path` (relative to `~/.hermes`, 1–4096 chars, no leading/trailing slash) and a `content` (UTF-8 body, max 1 MiB). |
| `spec.workspace.initialDirs` | `[]string` (listType=set) | `[]` | Directories to `mkdir -p` on first start. |
| `spec.workspace.configMapRef` | `LocalObjectReference` | `nil` | User-owned ConfigMap whose entries are merged onto `initialFiles`. Operator-managed entries win on key conflicts. |
| `spec.workspace.bootstrap.enabled` | `*bool` | `false` | When true, hermes-agent runs a one-shot bootstrap script (`hermes onboard`) on first start. Plan 3 wires the actual init-container. |

**WorkspaceFile fields:**

| Field | Type | Constraints | Description |
|---|---|---|---|
| `path` | `string` | Required; 1–4096 chars; pattern `^[^/].*[^/]$\|^[^/]$` | Relative path under `~/.hermes`. |
| `content` | `string` | Max 1 MiB (1 048 576 chars) | UTF-8 body of the file. |

### spec.resources

Sets CPU/memory requests and limits on the agent container. Defaults are intentionally absent at the schema level — the defaulting webhook fills them from `HermesClusterDefaults` when available; otherwise the pod inherits namespace-level `LimitRange` defaults.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.resources.requests` | `corev1.ResourceList` | `nil` | Resource requests map (e.g. `cpu: 100m`, `memory: 128Mi`). |
| `spec.resources.limits` | `corev1.ResourceList` | `nil` | Resource limits map (e.g. `cpu: 500m`, `memory: 512Mi`). |

### spec.security

Bundles pod/container security contexts, per-instance RBAC, NetworkPolicy, and optional CA-bundle injection.

#### spec.security.podSecurityContext

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.podSecurityContext` | `*corev1.PodSecurityContext` | `nil` (operator default applied) | Overrides the operator's default hardened pod security context. Operator default when nil: `runAsNonRoot=true`, `runAsUser=1000`, `fsGroup=1000`, `seccompProfile=RuntimeDefault`. |

#### spec.security.containerSecurityContext

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.containerSecurityContext` | `*corev1.SecurityContext` | `nil` (operator default applied) | Overrides the operator's default hardened container context. Operator default when nil: `readOnlyRootFilesystem=true`, `allowPrivilegeEscalation=false`, drop ALL capabilities. |

#### spec.security.rbac

Controls per-instance ServiceAccount, Role, and RoleBinding creation.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.rbac.createServiceAccount` | `*bool` | `true` | When true, the operator creates and owns a ServiceAccount named after the instance. Set to false to supply your own. |
| `spec.security.rbac.serviceAccountName` | `string` | `""` | Used when `createServiceAccount=false`. Name of an externally-managed ServiceAccount in the same namespace. |
| `spec.security.rbac.annotations` | `map[string]string` | `nil` | Annotations applied to the operator-created ServiceAccount. Use for IRSA (`eks.amazonaws.com/role-arn`), GKE Workload Identity (`iam.gke.io/gcp-service-account`), Azure Workload Identity, etc. |

#### spec.security.networkPolicy

Controls per-instance NetworkPolicy creation (default-deny baseline + selective allow).

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.networkPolicy.enabled` | `*bool` | `true` | When true, the operator creates a deny-all NetworkPolicy plus selective allow rules: DNS egress, port-443 egress, and Service ingress from the same namespace. |
| `spec.security.networkPolicy.allowDNS` | `*bool` | `true` | Emits a standard DNS egress rule (UDP+TCP port 53 to any peer). Disable only when CoreDNS is reachable via a different transport (e.g. node-local DNS via hostNetwork). |
| `spec.security.networkPolicy.allowedIngressNamespaces` | `[]string` (listType=set) | `[]` | Additional namespaces whose pods may connect to the agent's exposed ports (beyond the instance's own namespace). |
| `spec.security.networkPolicy.allowedIngressCIDRs` | `[]string` (listType=set) | `[]` | CIDRs that may connect to the agent's exposed ports. |
| `spec.security.networkPolicy.allowedEgressCIDRs` | `[]string` (listType=set) | `[]` | CIDRs the agent may connect to in addition to operator-built defaults (DNS + port 443). |
| `spec.security.networkPolicy.additionalEgress` | `[]networkingv1.NetworkPolicyEgressRule` | `[]` | User-supplied egress rules appended verbatim to the generated NetworkPolicy. |

#### spec.security.caBundle

Optionally mounts a CA bundle into the agent container at `/etc/ssl/certs/hermes-ca-bundle.crt` and sets `SSL_CERT_FILE` in the agent environment. Exactly one of `configMapName` or `secretName` should be set.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.caBundle.configMapName` | `string` | `""` | References a ConfigMap in the same namespace holding the PEM bundle. |
| `spec.security.caBundle.secretName` | `string` | `""` | References a Secret in the same namespace holding the PEM bundle. |
| `spec.security.caBundle.key` | `string` | `ca.crt` | Data-map key holding the PEM bundle within the ConfigMap or Secret. |

### spec.storage

Controls the PVC backing `~/.hermes` for this instance.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.storage.persistence.enabled` | `*bool` | `true` | When true, the operator creates and manages a PVC for the agent's data directory. |
| `spec.storage.persistence.size` | `string` | `1Gi` | Requested PVC size (Kubernetes quantity string). |
| `spec.storage.persistence.storageClassName` | `*string` | `nil` (cluster default) | StorageClass name. When nil, the cluster's default StorageClass is used. |

Note: PVCs are immutable once created — the operator only creates, never updates them.

### spec.networking

Exposes the agent via a Service and optionally an Ingress.

#### spec.networking.service

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.networking.service.type` | `string` (enum) | `ClusterIP` | Service type. Allowed: `ClusterIP`, `NodePort`, `LoadBalancer`. |
| `spec.networking.service.clusterIP` | `string` | `""` (api-server allocates) | Set to `None` for a headless Service. |
| `spec.networking.service.ports` | `[]NamedServicePort` (listType=map, listMapKey=name) | `[]` (operator emits default `gateway` port on 8443) | List of Service ports. When empty, the operator emits a `gateway` port on 8443 matching the StatefulSet's container port. |
| `spec.networking.service.annotations` | `map[string]string` | `nil` | Annotations applied verbatim to the Service (LoadBalancer hints, etc.). |
| `spec.networking.service.loadBalancerClass` | `*string` | `nil` | Propagated when `type=LoadBalancer`. |
| `spec.networking.service.externalTrafficPolicy` | `string` (enum) | `""` | Propagated when `type=LoadBalancer` or `NodePort`. Allowed: `Cluster`, `Local`. |

**NamedServicePort fields:**

| Field | Type | Default | Description |
|---|---|---|---|
| `name` | `string` | Required | Port name (1–63 chars). |
| `port` | `int32` | Required | Service port number (1–65535). |
| `targetPort` | `*int32` | `nil` (defaults to `port`) | Container port to forward to. |
| `protocol` | `string` (enum) | `TCP` | Transport protocol. Allowed: `TCP`, `UDP`, `SCTP`. |
| `nodePort` | `int32` | `0` (api-server allocates) | Node port number. Honored only when Service type is `NodePort` or `LoadBalancer`. |

#### spec.networking.ingress

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.networking.ingress.enabled` | `*bool` | `false` | When true, the operator creates an Ingress for the agent. |
| `spec.networking.ingress.host` | `string` | `""` | Primary hostname for the Ingress rule. |
| `spec.networking.ingress.className` | `*string` | `nil` | IngressClass name (e.g. `nginx`, `traefik`). |
| `spec.networking.ingress.tls` | `[]IngressTLSSpec` | `[]` | TLS settings. Each entry has a required `secretName` and an optional `hosts` list (listType=set). |
| `spec.networking.ingress.annotations` | `map[string]string` | `nil` | Annotations applied to the Ingress. The operator merges provider-specific defaults (force-HTTPS, etc.) on top. |
| `spec.networking.ingress.pathType` | `string` (enum) | `Prefix` | Path type. Allowed: `Exact`, `Prefix`, `ImplementationSpecific`. |
| `spec.networking.ingress.path` | `string` | `/` | URL path for the Ingress rule. |
| `spec.networking.ingress.servicePortName` | `string` | `gateway` | Name of the Service port the Ingress routes to. |

### spec.observability

Controls metrics exposure, Prometheus Operator integration, and logging configuration.

#### spec.observability.metrics

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.observability.metrics.enabled` | `*bool` | `true` | When true, the agent container exposes a `/metrics` endpoint. |
| `spec.observability.metrics.port` | `int32` | `9090` | Port for the `/metrics` endpoint (1–65535). |
| `spec.observability.metrics.secure` | `*bool` | `false` | When true, `/metrics` requires bearer-token auth and uses HTTPS. The ServiceMonitor scheme/scrape settings must agree (see lesson #435/#440). |

#### spec.observability.serviceMonitor

Controls Prometheus Operator `ServiceMonitor` emission. When enabled, the operator emits an unstructured `ServiceMonitor`; the Prometheus Operator CRDs do not need to be present at compile time.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.observability.serviceMonitor.enabled` | `*bool` | `false` | When true, the operator creates a ServiceMonitor in the same namespace. |
| `spec.observability.serviceMonitor.labels` | `map[string]string` | `nil` | Extra labels on the ServiceMonitor for Prometheus label-selector matching (e.g. `release: kube-prometheus-stack`). |
| `spec.observability.serviceMonitor.interval` | `string` | `30s` | Scrape interval. Must match the Prometheus duration regex (e.g. `30s`, `1m`). |
| `spec.observability.serviceMonitor.scrapeTimeout` | `string` | `10s` | Scrape timeout. Must be less than or equal to `interval`. Must match the Prometheus duration regex. |

#### spec.observability.prometheusRule

Controls emission of a default `PrometheusRule` with hermes-agent alerts (HighRestartRate, MetricsDown, etc.).

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.observability.prometheusRule.enabled` | `*bool` | `false` | When true, the operator creates a PrometheusRule with default hermes-agent alerting rules. |
| `spec.observability.prometheusRule.additionalRules` | `[]PrometheusRule` | `[]` | User-supplied alert rules merged onto the operator default ruleset. Each entry has required `alert` (name) and `expr` (PromQL) fields, plus optional `for`, `labels`, and `annotations`. |

#### spec.observability.logging

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.observability.logging.format` | `string` (enum) | `text` | Agent log output format. Allowed: `text`, `json`. |
| `spec.observability.logging.level` | `string` (enum) | `info` | Agent log level. Allowed: `trace`, `debug`, `info`, `warn`, `error`. Plan 3 wires `HERMES_LOG_LEVEL` on the agent container. |

### spec.availability

Bundles PodDisruptionBudget, HorizontalPodAutoscaler, and topology-spread constraints.

#### spec.availability.podDisruptionBudget

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.availability.podDisruptionBudget.enabled` | `*bool` | `false` | When true, the operator creates a PodDisruptionBudget. |
| `spec.availability.podDisruptionBudget.minAvailable` | `*IntOrString` | `nil` | Minimum available pods. Mutually exclusive with `maxUnavailable`. |
| `spec.availability.podDisruptionBudget.maxUnavailable` | `*IntOrString` | `nil` (defaults to `1` when neither is set and PDB is enabled) | Maximum unavailable pods. Mutually exclusive with `minAvailable`. |

#### spec.availability.horizontalPodAutoscaler

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.availability.horizontalPodAutoscaler.enabled` | `*bool` | `false` | When true, the operator creates a HorizontalPodAutoscaler. |
| `spec.availability.horizontalPodAutoscaler.minReplicas` | `*int32` | `1` | Minimum replica count (minimum value: 1). |
| `spec.availability.horizontalPodAutoscaler.maxReplicas` | `*int32` | `5` | Maximum replica count (minimum value: 1). |
| `spec.availability.horizontalPodAutoscaler.targetCPUUtilization` | `*int32` | `80` | Target CPU utilization percentage (1–100). The HPA metric target type is set explicitly to `Utilization`. |
| `spec.availability.horizontalPodAutoscaler.targetMemoryUtilization` | `*int32` | `nil` (disabled) | Target memory utilization percentage (1–100). When set, adds a memory-based HPA metric alongside the CPU metric. |
| `spec.availability.horizontalPodAutoscaler.behavior` | `*autoscalingv2.HorizontalPodAutoscalerBehavior` | `nil` | Forwarded verbatim to the HPA `spec.behavior` field for fine-grained scale-up/scale-down control. |

#### spec.availability.topologySpreadConstraints

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.availability.topologySpreadConstraints` | `[]corev1.TopologySpreadConstraint` | `[]` | List of topology-spread constraints applied to the agent pod. Forwarded verbatim to the StatefulSet pod template. |

### spec.probes

Overrides the operator's built-in liveness, readiness, and startup probes. Each field is a complete `corev1.Probe` applied verbatim — set every value you want non-default.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.probes.liveness` | `*corev1.Probe` | `nil` (operator default applied) | Replaces the operator's default liveness probe. |
| `spec.probes.readiness` | `*corev1.Probe` | `nil` (operator default applied) | Replaces the operator's default readiness probe. |
| `spec.probes.startup` | `*corev1.Probe` | `nil` (operator default applied) | Replaces the operator's default startup probe. |

### spec.scheduling

Targets the agent pod at specific nodes.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.scheduling.nodeSelector` | `map[string]string` | `nil` | Node labels that the pod must match (forwarded to `pod.spec.nodeSelector`). |
| `spec.scheduling.tolerations` | `[]corev1.Toleration` | `[]` | Tolerations applied to the pod (forwarded to `pod.spec.tolerations`). |
| `spec.scheduling.affinity` | `*corev1.Affinity` | `nil` | Node/pod affinity and anti-affinity rules (forwarded to `pod.spec.affinity`). |
| `spec.scheduling.priorityClassName` | `string` | `""` | PriorityClass name applied to the pod (forwarded to `pod.spec.priorityClassName`). |

### spec.selfConfigure

The allowlist policy for `HermesSelfConfig` mutations (Plan 4). Declares here so Plan 4 can target it via SSA without a CRD schema change. The validating webhook rejects `enabled=true` with `protectedKeys` empty.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.selfConfigure.enabled` | `*bool` | `nil` (Plan 4 interprets nil as false) | Whether agent self-mutation via HermesSelfConfig is permitted. Explicit `*bool` so the defaulter can distinguish "user said false" from "user did not set it". |
| `spec.selfConfigure.allowedActions` | `[]string` (listType=set) | `[]` | Permitted action categories Plan 4 enforces. Known values: `skills`, `config`, `envVars`, `workspaceFiles`, `profiles`. |
| `spec.selfConfigure.protectedKeys` | `[]string` (listType=set) | `[]` | Glob expressions over JSON paths that HermesSelfConfig may not mutate. Required to be non-empty when `enabled=true`. |

### spec extras

#### spec.initContainers

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.initContainers` | `[]corev1.Container` | `[]` | User-supplied init containers appended after any operator-managed init containers (e.g. the runtime-init container from Plan 3). |

#### spec.sidecars

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.sidecars` | `[]corev1.Container` | `[]` | User-supplied sidecar containers appended after operator-managed sidecars (e.g. ollama, web-terminal, tailscale from Plan 3). |

#### spec.extraVolumes

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.extraVolumes` | `[]corev1.Volume` | `[]` | Additional pod volumes appended to the operator-managed volume list. |

#### spec.extraVolumeMounts

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.extraVolumeMounts` | `[]corev1.VolumeMount` | `[]` | Additional volume mounts applied to the agent container, appended to the operator-managed list. |

### spec.env

#### spec.envFrom

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.envFrom` | `[]corev1.EnvFromSource` | `[]` | EnvFrom sources (ConfigMap or Secret refs) injected into the agent container. |

#### spec.env

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.env` | `[]corev1.EnvVar` (listType=map, listMapKey=name) | `[]` | Explicit environment variables for the agent container. The list-map key is `name` so HermesSelfConfig (Plan 4) can merge individual entries without replacing the whole list. |

#### spec.skills

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.skills` | `[]InstanceSkill` (listType=map, listMapKey=source) | `[]` | Declarative list of uv-installable skill sources. Each entry has a required `source` field (uv/pip-compatible install source, min length 1). Plan 3 wires the runtime; declared here so Plan 4's SSA can target it without a CRD schema change. |

### spec.suspended

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.suspended` | `bool` | `false` | When true, scales the StatefulSet to zero replicas. State (PVC, ConfigMap, etc.) is preserved. The instance `status.phase` transitions to `Suspended`. |

### `spec.runtime`

Controls the Python/uv runtime concerns of the agent container.

| Field | Type | Default | Description |
|---|---|---|---|
| `python` | string | `"3.11"` | Informational. The agent image's Python version is fixed at build time. |
| `uv.enabled` | *bool | `true` | When true, an init container runs `uv sync --frozen` against the lockfile bundled in the agent image. |
| `uv.extraIndexURL` | string | `""` | Appended to uv's index list. Useful for private PyPI mirrors. |
| `uv.cacheVolume.emptyDir` | EmptyDirVolumeSource | `{sizeLimit: 1Gi}` | Volume backing `/home/hermes/.cache/uv`. |
| `uv.cacheVolume.persistentVolumeClaim` | PersistentVolumeClaimVolumeSource | nil | If set, overrides the emptyDir. |
| `ffmpeg.enabled` | *bool | `true` | Toggles the FFmpeg dependency check (image always ships FFmpeg). |
| `ripgrep.enabled` | *bool | `true` | Toggles the ripgrep dependency check. |
| `extraAptPackages` | []string | `[]` | Adds APT packages via a root-privileged init container. **Security implication**: the init container runs as UID 0 for the duration of the apt install only. |
| `extraPipPackages` | []string | `[]` | Adds Python packages via `uv pip install` into a persistent venv on the data PVC (`/home/hermes/.hermes/.venv-extras`). |

### `spec.gateways`

Multi-platform messaging gateway bindings. Every platform is opt-in; tokens are referenced via Secret selectors so they can be rotated independently.

#### `spec.gateways.telegram`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | Bot API token. Surfaced as `TELEGRAM_BOT_TOKEN`. |
| `allowedUserIDs` | []int64 | `[]` | Allow-list of Telegram user IDs. Surfaced as `TELEGRAM_ALLOWED_USER_IDS` (comma-separated). |
| `webhookURL` | string | `""` | Public HTTPS URL to register with Telegram. Empty = long-poll. |

#### `spec.gateways.discord`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | Bot token. Surfaced as `DISCORD_BOT_TOKEN`. |
| `applicationID` | string | `""` | Application snowflake. Surfaced as `DISCORD_APPLICATION_ID`. |
| `guildIDs` | []string | `[]` | Scopes slash-command registration. Surfaced as `DISCORD_GUILD_IDS` (comma-separated). |

#### `spec.gateways.slack`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `botTokenSecretRef`. |
| `botTokenSecretRef` | SecretKeySelector | nil | `xoxb-` bot token. Surfaced as `SLACK_BOT_TOKEN`. |
| `appTokenSecretRef` | SecretKeySelector | nil | `xapp-` app-level token for Socket Mode. Surfaced as `SLACK_APP_TOKEN`. |
| `signingSecretRef` | SecretKeySelector | nil | Slack signing secret. Surfaced as `SLACK_SIGNING_SECRET`. |

#### `spec.gateways.whatsapp`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `providerSecretRef`. |
| `providerSecretRef` | SecretKeySelector | nil | Provider credentials. The whole Secret is mounted as env with prefix `WHATSAPP_`. |

#### `spec.gateways.signal`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires both `phoneNumberSecretRef` and `authTokenSecretRef`. |
| `phoneNumberSecretRef` | SecretKeySelector | nil | Registered phone number. Surfaced as `SIGNAL_PHONE_NUMBER`. |
| `authTokenSecretRef` | SecretKeySelector | nil | Auth token for signal-cli-rest-api. Surfaced as `SIGNAL_AUTH_TOKEN`. |

### `spec.profileStore`

Optional companion service for the Honcho dialectic profile store.

#### `spec.profileStore.honcho`

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | *bool | `false` | Toggle. Enabling requires `apiKeySecretRef`. |
| `image.repository` | string | `"ghcr.io/plastic-labs/honcho"` | Honcho image. |
| `image.tag` | string | `"0.1.0"` | Honcho image tag. |
| `image.pullPolicy` | string | `IfNotPresent` | One of `Always`, `IfNotPresent`, `Never`. |
| `persistence.enabled` | *bool | `true` | Whether to create a PVC for Honcho. |
| `persistence.size` | string | `"5Gi"` | PVC size. |
| `persistence.storageClassName` | *string | nil | PVC storage class. |
| `resources` | ResourceRequirements | `{}` | Honcho container resource requests/limits. |
| `apiKeySecretRef` | SecretKeySelector | nil | API key the agent uses to authenticate. Surfaced as `HONCHO_API_KEY` on both the agent and the Honcho container. |

The agent receives `HONCHO_BASE_URL=http://<inst>-honcho:8000` automatically.

The Honcho-side PVC layout that Plan 4's `addProfileSnapshot` Job writes to is `/data/snapshots/<profileID>/<RFC3339-timestamp>.json` (relative to the Honcho container, which mounts the PVC at `/data`).

### HermesInstance status

| Field | Type | Description |
|---|---|---|
| `status.observedGeneration` | `int64` | Most recent `metadata.generation` that the controller has fully reconciled. |
| `status.phase` | `string` | Short human-readable status. Values: `Pending`, `Ready`, `Degraded`, `Suspended`. |
| `status.replicas` | `int32` | Latest observed StatefulSet replica count. |
| `status.readyReplicas` | `int32` | Latest observed ready-replica count. |
| `status.conditions` | `[]metav1.Condition` (listType=map, listMapKey=type) | Subsystem readiness conditions. See `docs/conditions.md` for the full catalogue. Condition types: `StorageReady`, `ConfigReady`, `SecretsReady`, `NetworkPolicyReady`, `RBACReady`, `ServiceReady`, `PDBReady`, `HPAReady`, `IngressReady`, `ServiceMonitorReady`, `PrometheusRuleReady`, `Ready`. |

---

## HermesClusterDefaults

**API group / version:** `hermes.agent/v1`
**Kind:** `HermesClusterDefaults`
**Scope:** Cluster-scoped
**Short name:** `hcd`
**Categories:** `hermes`, `agents`
**Singleton constraint:** `metadata.name` must be `cluster`. The validating webhook rejects any other name.

HermesClusterDefaults provides cluster-wide defaults applied by the defaulting webhook when a HermesInstance leaves a field nil. An explicit value on the instance always wins. Only one instance of this resource should exist, named `cluster`.

### hcd spec.image

Same schema as `HermesInstance.spec.image`. Used as the cluster-wide default image when `spec.image` is omitted on an instance.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.image.repository` | `string` | `ghcr.io/stubbi/hermes-agent` | Default container image repository for all instances. |
| `spec.image.tag` | `string` | `latest` | Default image tag. |
| `spec.image.pullPolicy` | `string` (enum) | `IfNotPresent` | Default pull policy. Allowed: `Always`, `IfNotPresent`, `Never`. |

### hcd spec.registry

Image-pull secret hints applied when the instance does not override.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.registry.pullSecretName` | `string` | `""` | When non-empty, added to every instance's `pod.spec.imagePullSecrets` (unless the instance overrides). |

### hcd spec.storage

Same schema as `HermesInstance.spec.storage`. Used as the cluster-wide default when the instance leaves `spec.storage` nil.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.storage.persistence.enabled` | `*bool` | `true` | Default persistence enablement for all instances. |
| `spec.storage.persistence.size` | `string` | `1Gi` | Default PVC size for all instances. |
| `spec.storage.persistence.storageClassName` | `*string` | `nil` | Default StorageClass. When nil, the cluster default is used. |

### hcd spec.security

Defaults the defaultable subset of `SecuritySpec`. Note: hardened security contexts (`readOnlyRootFilesystem`, dropped capabilities, etc.) are operator-baked and cannot be defaulted from here.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.security.serviceAccount.annotations` | `map[string]string` | `nil` | Default annotations applied to every operator-created ServiceAccount (IRSA, GKE WI, Azure WI, etc.). |
| `spec.security.networkPolicy.enabled` | `*bool` | `nil` (operator default `true` applies) | Default value for whether per-instance NetworkPolicies are created. |
| `spec.security.networkPolicy.allowDNS` | `*bool` | `nil` (operator default `true` applies) | Default value for whether the DNS egress rule is emitted. |
| `spec.security.caBundle.configMapName` | `string` | `""` | Default ConfigMap name for the CA bundle. |
| `spec.security.caBundle.secretName` | `string` | `""` | Default Secret name for the CA bundle. |
| `spec.security.caBundle.key` | `string` | `ca.crt` | Default data-map key for the PEM bundle. |

### hcd spec.observability

Defaults the defaultable subset of `ObservabilitySpec`. Uses the same nested types as `HermesInstance.spec.observability`.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.observability.metrics.enabled` | `*bool` | `true` | Default metrics enablement. |
| `spec.observability.metrics.port` | `int32` | `9090` | Default metrics port. |
| `spec.observability.metrics.secure` | `*bool` | `false` | Default metrics security. |
| `spec.observability.serviceMonitor.enabled` | `*bool` | `false` | Default ServiceMonitor enablement. |
| `spec.observability.serviceMonitor.labels` | `map[string]string` | `nil` | Default extra labels for ServiceMonitor. |
| `spec.observability.serviceMonitor.interval` | `string` | `30s` | Default scrape interval. |
| `spec.observability.serviceMonitor.scrapeTimeout` | `string` | `10s` | Default scrape timeout. |
| `spec.observability.prometheusRule.enabled` | `*bool` | `false` | Default PrometheusRule enablement. |
| `spec.observability.prometheusRule.additionalRules` | `[]PrometheusRule` | `[]` | Default additional alert rules. |
| `spec.observability.logging.format` | `string` (enum) | `text` | Default log format. Allowed: `text`, `json`. |
| `spec.observability.logging.level` | `string` (enum) | `info` | Default log level. Allowed: `trace`, `debug`, `info`, `warn`, `error`. |

### hcd spec.networking

Defaults the defaultable subset of `NetworkingSpec`.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.networking.service.type` | `string` (enum) | `""` (operator default `ClusterIP` applies) | Default Service type for all instances. Allowed: `ClusterIP`, `NodePort`, `LoadBalancer`. |
| `spec.networking.networkPolicy.enabled` | `*bool` | `nil` | Default NetworkPolicy enablement (duplicated from `spec.security.networkPolicy` for convenience; instance-level `spec.security.networkPolicy` takes precedence). |
| `spec.networking.networkPolicy.allowDNS` | `*bool` | `nil` | Default DNS egress rule enablement. |

### hcd spec.resources

Same schema as `HermesInstance.spec.resources`. Provides cluster-wide default resource requests and limits when the instance leaves `spec.resources` nil.

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.resources.requests` | `corev1.ResourceList` | `nil` | Default resource requests (e.g. `cpu: 100m`, `memory: 128Mi`). |
| `spec.resources.limits` | `corev1.ResourceList` | `nil` | Default resource limits (e.g. `cpu: 500m`, `memory: 512Mi`). |

### HermesClusterDefaults status

| Field | Type | Description |
|---|---|---|
| `status.observedGeneration` | `int64` | Most recent `metadata.generation` that the controller has fully reconciled. |
| `status.conditions` | `[]metav1.Condition` (listType=map, listMapKey=type) | Singleton readiness conditions. See `docs/conditions.md` for the full catalogue. Condition type: `Ready`. |

---

## HermesSelfConfig

Agent-driven, audited mutation API. Validated against the parent
`HermesInstance.spec.selfConfigure` policy and applied via Server-Side Apply
with field manager `hermes.agent/selfconfig`.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `instanceRef` | string | yes | Name of the parent `HermesInstance` in the same namespace. |
| `addSkills[]` | list | no | uv-compatible source specifiers appended to `HermesInstance.spec.skills`. SSA list-map key: `source`. |
| `addSkills[].source` | string | yes (per item) | e.g. `git+https://github.com/foo/skill@v1.2`. |
| `addSkills[].version` | string | no | Audit-only human label. |
| `patchConfig` | `apiextensions/v1.JSON` | no | JSON merge patch (RFC 7396) written to the workspace ConfigMap key `selfconfig.yaml`. Layered onto `~/.hermes/config.yaml` at agent startup. |
| `addEnvVars[]` | list | no | Environment variables appended to `HermesInstance.spec.env`. SSA list-map key: `name`. |
| `addEnvVars[].name` | string | yes | C-identifier (`^[A-Za-z_][A-Za-z0-9_]*$`). |
| `addEnvVars[].value` | string | no | Literal value. Mutually exclusive with `valueFrom`. |
| `addEnvVars[].valueFrom.secretKeyRef` | object | no | Resolve from `Secret` key. |
| `addEnvVars[].valueFrom.configMapKeyRef` | object | no | Resolve from `ConfigMap` key. |
| `addWorkspaceFiles[]` | list | no | Files materialised under `~/.hermes/workspace/`. Nested paths supported via `/` → `__` ConfigMap-key encoding. |
| `addWorkspaceFiles[].path` | string | yes | Relative path. |
| `addWorkspaceFiles[].content` | string | no | Literal file body. |
| `addProfileSnapshot` | object | no | Write a Honcho profile snapshot. Requires `HermesInstance.spec.profileStore.honcho.enabled=true`. |
| `addProfileSnapshot.profileID` | string | yes | Honcho profile identifier. |
| `addProfileSnapshot.data` | string | yes | Opaque payload written verbatim to `/data/snapshots/<profileID>/<RFC3339>.json`. |

### Status

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | int64 | Spec generation the controller last processed. |
| `phase` | enum | One of `Pending`, `Applied`, `Denied`. |
| `appliedAt` | timestamp | Time of the most recent successful SSA write. |
| `denyReason` | string | Populated when `phase=Denied`. |
| `appliedFields[]` | list of strings | Dotted paths SSA touched, e.g. `spec.env[name=FINANCE_TZ]`. |
| `conditions[]` | `[]metav1.Condition` | Standard k8s conditions. Types: `Applied`, `Denied`, `Pending`. |

### Policy model

Allowed mutations are governed by the parent's `selfConfigure` block:

```yaml
spec:
  selfConfigure:
    enabled: true
    allowedActions: [skills, config, envVars, workspaceFiles, profiles]
    protectedKeys:
      - "provider.apiKey"
      - "*.secret*"
      - "gateways.telegram.token"
```

- `enabled: false` (or unset) → every SelfConfig is `Denied` with reason `selfconfig disabled on parent`.
- `allowedActions` is the closed set of permitted mutation categories.
- `protectedKeys` are glob patterns matched against the dotted JSON path of `patchConfig` (gobwas/glob, `.` is the segment separator).

### Field-manager contract

The reconciler writes via `client.Apply` with field owner `hermes.agent/selfconfig`. It owns only the paths the SelfConfig touches — never `spec.image`, `spec.storage`, `spec.gateways`, etc. Other field managers (FluxCD, Argo CD, kubectl users) co-own their own paths and are not disturbed. By default conflicts are surfaced as `Denied`. To force ownership, set `metadata.annotations["hermes.agent/force-ownership"]: "true"` on the SelfConfig.
