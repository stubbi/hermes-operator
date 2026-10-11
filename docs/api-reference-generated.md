# API Reference

## Packages
- [hermes.agent/v1](#hermesagentv1)


## hermes.agent/v1

Package v1 contains API Schema definitions for the hermes v1 API group.

### Resource Types
- [HermesClusterDefaults](#hermesclusterdefaults)
- [HermesInstance](#hermesinstance)
- [HermesSelfConfig](#hermesselfconfig)



#### AutoUpdateRollbackSpec



AutoUpdateRollbackSpec configures the rollback path.



_Appears in:_
- [AutoUpdateSpec](#autoupdatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `probeFailureThreshold` _integer_ |  | 3 | Maximum: 100 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### AutoUpdateSourceSpec



AutoUpdateSourceSpec is the OCI registry source for the channel.



_Appears in:_
- [AutoUpdateSpec](#autoupdatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registry` _string_ |  |  | Optional: \{\} <br /> |
| `channel` _string_ |  |  | Optional: \{\} <br /> |


#### AutoUpdateSpec



AutoUpdateSpec controls opt-in OCI-registry polling for newer agent images.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `source` _[AutoUpdateSourceSpec](#autoupdatesourcespec)_ |  |  | Optional: \{\} <br /> |
| `pollInterval` _string_ |  | 1h | Optional: \{\} <br /> |
| `backupBeforeUpdate` _boolean_ |  | true | Optional: \{\} <br /> |
| `rollback` _[AutoUpdateRollbackSpec](#autoupdaterollbackspec)_ |  |  | Optional: \{\} <br /> |


#### AvailabilitySpec



AvailabilitySpec bundles PDB, HPA, and topology-spread.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podDisruptionBudget` _[PDBSpec](#pdbspec)_ |  |  | Optional: \{\} <br /> |
| `horizontalPodAutoscaler` _[HPASpec](#hpaspec)_ |  |  | Optional: \{\} <br /> |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ |  |  | Optional: \{\} <br /> |


#### BackupS3Spec



BackupS3Spec configures the S3-compatible remote target.



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucket` _string_ |  |  |  |
| `endpoint` _string_ |  |  |  |
| `region` _string_ |  |  | Optional: \{\} <br /> |
| `pathPrefix` _string_ |  |  | Optional: \{\} <br /> |
| `credentialsSecretRef` _[LocalObjectReference](#localobjectreference)_ |  |  |  |


#### BackupSpec



BackupSpec controls S3-compatible PVC snapshots for this instance.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[BackupS3Spec](#backups3spec)_ |  |  | Optional: \{\} <br /> |
| `schedule` _string_ |  |  | Optional: \{\} <br /> |
| `onDelete` _boolean_ |  | false | Optional: \{\} <br /> |
| `preUpdate` _boolean_ |  | true | Optional: \{\} <br /> |
| `historyLimit` _integer_ |  | 30 | Maximum: 10000 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `failedHistoryLimit` _integer_ |  | 3 | Maximum: 1000 <br />Minimum: 0 <br />Optional: \{\} <br /> |
| `image` _string_ |  |  | Optional: \{\} <br /> |


#### CABundleSpec



CABundleSpec optionally mounts a CA bundle into the agent container.
Exactly one of ConfigMapName / SecretName SHOULD be set.



_Appears in:_
- [SecurityDefaults](#securitydefaults)
- [SecuritySpec](#securityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName references a ConfigMap in the same namespace. |  | Optional: \{\} <br /> |
| `secretName` _string_ | SecretName references a Secret in the same namespace. |  | Optional: \{\} <br /> |
| `key` _string_ | Key is the data-map key holding the PEM bundle. Default "ca.crt". | ca.crt | Optional: \{\} <br /> |


#### ConfigMergeMode

_Underlying type:_ _string_

ConfigMergeMode controls how Raw and ConfigMapRef are combined.

_Validation:_
- Enum: [replace merge]

_Appears in:_
- [ConfigSpec](#configspec)

| Field | Description |
| --- | --- |
| `replace` | ConfigMergeModeReplace: Raw replaces ConfigMapRef entirely when both are set.<br />This is the default to avoid surprising merges.<br /> |
| `merge` | ConfigMergeModeMerge: YAML deep-merge Raw onto ConfigMapRef. Raw wins on conflict.<br /> |


#### ConfigSpec



ConfigSpec holds the agent's ~/.hermes/config.yaml. Exactly one of Raw or
ConfigMapRef SHOULD be set; the validating webhook rejects both unset and
emits a warning if both are set with MergeMode unset.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `raw` _[RawConfig](#rawconfig)_ | Raw is the inline YAML body of config.yaml. Stored as a RawExtension so<br />users may write structured YAML in the manifest without escaping. |  | Optional: \{\} <br /> |
| `configMapRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core)_ | ConfigMapRef references a ConfigMap in the same namespace whose<br />"config.yaml" key holds the body. |  | Optional: \{\} <br /> |
| `mergeMode` _[ConfigMergeMode](#configmergemode)_ | MergeMode controls combination when both Raw and ConfigMapRef are set. | replace | Enum: [replace merge] <br />Optional: \{\} <br /> |


#### DiscordGatewaySpec



DiscordGatewaySpec binds the agent to a Discord bot application.



_Appears in:_
- [GatewaysSpec](#gatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `botTokenSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |
| `applicationID` _string_ | ApplicationID is the Discord application's snowflake. |  | Optional: \{\} <br /> |
| `guildIDs` _string array_ | GuildIDs scopes slash-command registration to specific guilds. |  | Optional: \{\} <br /> |


#### FFmpegSpec



FFmpegSpec controls the FFmpeg dependency check.



_Appears in:_
- [RuntimeSpec](#runtimespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |


#### GatewaysSpec



GatewaysSpec is the union of all supported messaging-platform bindings.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `telegram` _[TelegramGatewaySpec](#telegramgatewayspec)_ |  |  | Optional: \{\} <br /> |
| `discord` _[DiscordGatewaySpec](#discordgatewayspec)_ |  |  | Optional: \{\} <br /> |
| `slack` _[SlackGatewaySpec](#slackgatewayspec)_ |  |  | Optional: \{\} <br /> |
| `whatsapp` _[WhatsAppGatewaySpec](#whatsappgatewayspec)_ |  |  | Optional: \{\} <br /> |
| `signal` _[SignalGatewaySpec](#signalgatewayspec)_ |  |  | Optional: \{\} <br /> |


#### GrafanaDashboardSpec



GrafanaDashboardSpec configures auto-provisioned Grafana dashboard ConfigMaps.



_Appears in:_
- [MetricsSpec](#metricsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables Grafana dashboard ConfigMap creation. | false | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels to add to the dashboard ConfigMaps (in addition to grafana_dashboard: "1"). |  | Optional: \{\} <br /> |
| `folder` _string_ | Folder is the Grafana folder to place the dashboards in. | Hermes | Optional: \{\} <br /> |


#### HPASpec



HPASpec controls HorizontalPodAutoscaler emission.



_Appears in:_
- [AvailabilitySpec](#availabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `minReplicas` _integer_ | MinReplicas: default 1. | 1 | Minimum: 1 <br />Optional: \{\} <br /> |
| `maxReplicas` _integer_ | MaxReplicas: default 5. | 5 | Minimum: 1 <br />Optional: \{\} <br /> |
| `targetCPUUtilization` _integer_ | TargetCPUUtilization: default 80 (percent). | 80 | Maximum: 100 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `targetMemoryUtilization` _integer_ | TargetMemoryUtilization: optional, when set adds a memory metric. |  | Maximum: 100 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `behavior` _[HorizontalPodAutoscalerBehavior](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#horizontalpodautoscalerbehavior-v2-autoscaling)_ | Behavior is forwarded onto HPA's autoscaling/v2 behavior field.<br />Plan 6 conformance suite asserts the field is exposed; v1 forwards it raw. |  | Optional: \{\} <br /> |


#### HTTPRouteParentRef



HTTPRouteParentRef references a parent (typically a Gateway) the route attaches to.



_Appears in:_
- [HTTPRouteSpec](#httproutespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the parent resource (e.g. the Gateway name). |  | MinLength: 1 <br /> |
| `namespace` _string_ | Namespace of the parent. Defaults to the HermesInstance namespace when empty. |  | Optional: \{\} <br /> |
| `sectionName` _string_ | SectionName is the name of a section within the parent (e.g. a Gateway listener). |  | Optional: \{\} <br /> |


#### HTTPRouteSpec



HTTPRouteSpec controls optional Gateway API HTTPRoute creation. It mirrors the
IngressSpec shape for consistency: a single prefix rule routing to the agent
Service. The route is only created when Enabled is true.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled: when true, the operator creates an HTTPRoute for the agent.<br />Default false. | false | Optional: \{\} <br /> |
| `parentRefs` _[HTTPRouteParentRef](#httprouteparentref) array_ | ParentRefs are the Gateways (or other parents) this route attaches to.<br />At least one is required for the route to take effect. |  | Optional: \{\} <br /> |
| `hostnames` _string array_ | Hostnames are the hostnames matched by this route. |  | Optional: \{\} <br /> |
| `path` _string_ | Path is the path prefix routed to the agent Service. Default "/". | / | Optional: \{\} <br /> |
| `servicePortName` _string_ | ServicePortName: name of the Service port the route should target.<br />Default "gateway". | gateway | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are applied verbatim onto the HTTPRoute. |  | Optional: \{\} <br /> |


#### HermesClusterDefaults



HermesClusterDefaults is the Schema for the hermesclusterdefaults API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `hermes.agent/v1` | | |
| `kind` _string_ | `HermesClusterDefaults` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)_ | spec defines the desired state of HermesClusterDefaults |  | Required: \{\} <br /> |


#### HermesClusterDefaultsSpec



HermesClusterDefaultsSpec is the cluster-wide default set applied by the
defaulting webhook when a HermesInstance leaves a field nil. ClusterDefaults
only fills nil fields; an explicit value on the instance always wins.



_Appears in:_
- [HermesClusterDefaults](#hermesclusterdefaults)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _[ImageSpec](#imagespec)_ | Image defaults the instance's spec.image. |  | Optional: \{\} <br /> |
| `registry` _[RegistryDefaults](#registrydefaults)_ | Registry defaults image-pull plumbing. |  | Optional: \{\} <br /> |
| `storage` _[StorageSpec](#storagespec)_ | Storage defaults the instance's spec.storage. |  | Optional: \{\} <br /> |
| `security` _[SecurityDefaults](#securitydefaults)_ | Security defaults SA annotations + NetworkPolicy on/off + container-level<br />defaults (read-only rootfs etc. are operator-baked, not defaultable). |  | Optional: \{\} <br /> |
| `observability` _[ObservabilityDefaults](#observabilitydefaults)_ | Observability defaults metrics / ServiceMonitor / PrometheusRule. |  | Optional: \{\} <br /> |
| `networking` _[NetworkingDefaults](#networkingdefaults)_ | Networking defaults Service kind + NetworkPolicy enablement. |  | Optional: \{\} <br /> |
| `resources` _[ResourcesSpec](#resourcesspec)_ | Resources defaults requests + limits when the instance leaves them nil. |  | Optional: \{\} <br /> |


#### HermesInstance



HermesInstance is the Schema for the hermesinstances API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `hermes.agent/v1` | | |
| `kind` _string_ | `HermesInstance` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[HermesInstanceSpec](#hermesinstancespec)_ | spec defines the desired state of HermesInstance |  | Required: \{\} <br /> |


#### HermesInstanceSpec



HermesInstanceSpec defines the desired state of HermesInstance.
Field order follows design §4.



_Appears in:_
- [HermesInstance](#hermesinstance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _[ImageSpec](#imagespec)_ | Image selects the hermes-agent container image. |  | Optional: \{\} <br /> |
| `config` _[ConfigSpec](#configspec)_ | Config is the YAML content of ~/.hermes/config.yaml, supplied inline,<br />from a referenced ConfigMap, or merged from both. |  | Optional: \{\} <br /> |
| `workspace` _[WorkspaceSpec](#workspacespec)_ | Workspace seeds initial files and directories into ~/.hermes on first start. |  | Optional: \{\} <br /> |
| `resources` _[ResourcesSpec](#resourcesspec)_ | Resources sets the agent container's CPU/memory requests + limits. |  | Optional: \{\} <br /> |
| `security` _[SecuritySpec](#securityspec)_ | Security configures pod/container security contexts, RBAC, NetworkPolicy,<br />and the optional cluster CA bundle injection. |  | Optional: \{\} <br /> |
| `storage` _[StorageSpec](#storagespec)_ | Storage controls the PVC backing ~/.hermes for this instance. |  | Optional: \{\} <br /> |
| `networking` _[NetworkingSpec](#networkingspec)_ | Networking exposes the agent via Service / Ingress. |  | Optional: \{\} <br /> |
| `observability` _[ObservabilitySpec](#observabilityspec)_ | Observability turns on metrics, ServiceMonitor, PrometheusRule, and logging. |  | Optional: \{\} <br /> |
| `availability` _[AvailabilitySpec](#availabilityspec)_ | Availability sets PDB, HPA, and topology-spread constraints. |  | Optional: \{\} <br /> |
| `probes` _[ProbesSpec](#probesspec)_ | Probes lets users override the built-in liveness/readiness/startup probes. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling targets the agent pod at specific nodes. |  | Optional: \{\} <br /> |
| `shareProcessNamespace` _boolean_ | ShareProcessNamespace enables PID namespace sharing between all containers<br />in the pod. Defaults to false: the upstream hermes-agent image runs under<br />s6-overlay, whose /init must be PID 1 (s6-overlay-suexec aborts otherwise),<br />and s6 already reaps zombies non-blocking on SIGCHLD — so sharing the process<br />namespace (which makes the pause container PID 1) is both incompatible and<br />unnecessary.<br />Security note: enabling this lets every container in the pod see and signal<br />every other container's processes. A compromised sidecar could send signals<br />to the agent and vice versa. Leave false to keep per-container PID isolation. | false | Optional: \{\} <br /> |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | InitContainers is a user-supplied list of init containers appended after<br />any operator-managed init containers (e.g. runtime-init from Plan 3). |  | Optional: \{\} <br /> |
| `sidecars` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Sidecars is a user-supplied list of sidecars appended after operator-managed<br />sidecars (e.g. ollama / web-terminal / tailscale from Plan 3). |  | Optional: \{\} <br /> |
| `extraVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | ExtraVolumes is a user-supplied list of additional pod volumes. |  | Optional: \{\} <br /> |
| `extraVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | ExtraVolumeMounts is a user-supplied list of additional volume mounts<br />applied to the agent container. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envfromsource-v1-core) array_ | EnvFrom is a list of EnvFrom sources (ConfigMap/Secret refs) injected<br />into the agent container. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Env is a list of explicit environment variables for the agent container.<br />SSA list-map key is "name" so HermesSelfConfig can merge entries without<br />replacing the whole list. |  | Optional: \{\} <br /> |
| `skills` _[InstanceSkill](#instanceskill) array_ | Skills is the declarative list of uv-installable skill sources. Plan 3<br />wires the runtime; the field is declared here so SSA from HermesSelfConfig<br />(Plan 4) can target it without a CRD schema change. |  | Optional: \{\} <br /> |
| `selfConfigure` _[SelfConfigureSpec](#selfconfigurespec)_ | SelfConfigure is the allowlist policy for HermesSelfConfig mutations. |  | Optional: \{\} <br /> |
| `suspended` _boolean_ | Suspended scales the StatefulSet to zero replicas without deleting state. |  | Optional: \{\} <br /> |
| `backup` _[BackupSpec](#backupspec)_ | Backup controls scheduled and on-delete PVC snapshot behaviour. |  | Optional: \{\} <br /> |
| `restoreFrom` _string_ | RestoreFrom names a backup snapshot to restore from on next boot. |  | Optional: \{\} <br /> |
| `autoUpdate` _[AutoUpdateSpec](#autoupdatespec)_ | AutoUpdate controls opt-in OCI-registry polling for newer agent images. |  | Optional: \{\} <br /> |
| `migration` _[MigrationSpec](#migrationspec)_ | Migration is a one-shot migration source (set on initial create only). |  | Optional: \{\} <br /> |
| `runtime` _[RuntimeSpec](#runtimespec)_ | Runtime configured the agent's Python toolchain and OS-level dependencies<br />for the old hand-rolled agent image. It is now IGNORED: the published agent<br />image is the upstream NousResearch/hermes-agent s6 runtime, which ships its<br />own Python env, browser, node, and dependencies (see docs/runtime.md), so<br />the operator no longer builds a runtime via init containers. Setting this<br />has no effect.<br />Deprecated: ignored since the upstream-image runtime (v0.1.19); scheduled<br />for removal no earlier than v0.3.0 and 2027-01-01. See docs/deprecations.md. |  | Optional: \{\} <br /> |
| `gateways` _[GatewaysSpec](#gatewaysspec)_ | Gateways configures the platform-side messaging bindings (Telegram, Discord,<br />Slack, WhatsApp, Signal). Each gateway is opt-in and references its own<br />Secret(s) so tokens are rotatable independently. |  | Optional: \{\} <br /> |
| `profileStore` _[ProfileStoreSpec](#profilestorespec)_ | ProfileStore configures the optional Honcho profile-store companion. |  | Optional: \{\} <br /> |
| `tailscale` _[TailscaleSpec](#tailscalespec)_ | Tailscale exposes the gateway over a Tailscale tailnet. |  | Optional: \{\} <br /> |


#### HermesSelfConfig



HermesSelfConfig is the Schema for the hermesselfconfigs API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `hermes.agent/v1` | | |
| `kind` _string_ | `HermesSelfConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[HermesSelfConfigSpec](#hermesselfconfigspec)_ | spec defines the desired state of HermesSelfConfig |  | Required: \{\} <br /> |


#### HermesSelfConfigSpec



HermesSelfConfigSpec is an agent-driven, audited request to mutate the
parent HermesInstance. The operator validates against the parent's
.spec.selfConfigure policy, then applies via Server-Side Apply with
field manager "hermes.agent/selfconfig".



_Appears in:_
- [HermesSelfConfig](#hermesselfconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `instanceRef` _string_ | InstanceRef is the name of the parent HermesInstance in the same namespace. |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `addSkills` _[SelfConfigSkill](#selfconfigskill) array_ | AddSkills appends skills to the parent's .spec.skills. |  | MaxItems: 20 <br />Optional: \{\} <br /> |
| `patchConfig` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | PatchConfig is a JSON merge patch (RFC 7396) merged by the operator<br />into the rendered config ConfigMap once this request is Applied.<br />Later requests win on conflict; a null value deletes a key. |  | Optional: \{\} <br /> |
| `addEnvVars` _[SelfConfigEnvVar](#selfconfigenvvar) array_ | AddEnvVars appends environment variables to the parent's .spec.env. |  | MaxItems: 20 <br />Optional: \{\} <br /> |
| `addWorkspaceFiles` _[SelfConfigWorkspaceFile](#selfconfigworkspacefile) array_ | AddWorkspaceFiles writes files into the workspace ConfigMap. |  | MaxItems: 50 <br />Optional: \{\} <br /> |
| `addProfileSnapshot` _[SelfConfigProfileSnapshot](#selfconfigprofilesnapshot)_ | AddProfileSnapshot writes an opaque Honcho profile snapshot via a one-shot Job. |  | Optional: \{\} <br /> |


#### HonchoImageSpec



HonchoImageSpec selects the Honcho image.



_Appears in:_
- [HonchoSpec](#honchospec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ |  | ghcr.io/plastic-labs/honcho | Optional: \{\} <br /> |
| `tag` _string_ |  | 0.1.0 | Optional: \{\} <br /> |
| `pullPolicy` _string_ |  | IfNotPresent | Enum: [Always IfNotPresent Never] <br />Optional: \{\} <br /> |


#### HonchoPersistenceSpec



HonchoPersistenceSpec controls the Honcho-side PVC.



_Appears in:_
- [HonchoSpec](#honchospec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `size` _string_ |  | 5Gi | Optional: \{\} <br /> |
| `storageClassName` _string_ |  |  | Optional: \{\} <br /> |


#### HonchoSpec



HonchoSpec controls the Honcho companion Deployment.



_Appears in:_
- [ProfileStoreSpec](#profilestorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `image` _[HonchoImageSpec](#honchoimagespec)_ |  |  | Optional: \{\} <br /> |
| `persistence` _[HonchoPersistenceSpec](#honchopersistencespec)_ |  |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  | Optional: \{\} <br /> |
| `apiKeySecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ | APIKeySecretRef points at the Secret holding the Honcho API key. |  | Optional: \{\} <br /> |


#### ImageSpec



ImageSpec selects an OCI image.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ |  | ghcr.io/paperclipinc/hermes-agent | Optional: \{\} <br /> |
| `tag` _string_ | Tag is the container image tag. Either tag or digest must be set; there is<br />no default, because pinning to a mutable tag like :latest can silently pull<br />a broken upstream build. |  | Optional: \{\} <br /> |
| `digest` _string_ | Digest overrides the tag with an image digest (e.g. sha256:abc...). When set<br />it takes precedence over the tag for the resolved image reference. |  | Optional: \{\} <br /> |
| `pullPolicy` _string_ |  | IfNotPresent | Enum: [Always IfNotPresent Never] <br />Optional: \{\} <br /> |


#### IngressSpec



IngressSpec controls optional Ingress creation.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled: when true, the operator creates an Ingress for the agent.<br />Default false. | false | Optional: \{\} <br /> |
| `host` _string_ | Host is the primary hostname. |  | Optional: \{\} <br /> |
| `className` _string_ | ClassName is the IngressClass (`nginx`, `traefik`, ...). |  | Optional: \{\} <br /> |
| `tls` _[IngressTLSSpec](#ingresstlsspec) array_ | TLS is the list of TLS settings. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are applied to the Ingress. The operator merges<br />provider-specific defaults (force-https, etc.) on top of these. |  | Optional: \{\} <br /> |
| `pathType` _[PathType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pathtype-v1-networking)_ | PathType: default Prefix. | Prefix | Enum: [Exact Prefix ImplementationSpecific] <br />Optional: \{\} <br /> |
| `path` _string_ | Path: default "/". | / | Optional: \{\} <br /> |
| `servicePortName` _string_ | ServicePortName: name of the Service port the Ingress should route to.<br />Default "gateway". | gateway | Optional: \{\} <br /> |


#### IngressTLSSpec



IngressTLSSpec is a single TLS section on the Ingress.



_Appears in:_
- [IngressSpec](#ingressspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ |  |  | MinLength: 1 <br /> |
| `hosts` _string array_ |  |  |  |


#### InstanceSkill



InstanceSkill: Plan 3 fills the runtime semantics. The field exists here so
SSA from HermesSelfConfig (Plan 4) can patch the slice with listMapKey=source.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ | Source is the uv/pip-compatible install source. |  | MinLength: 1 <br /> |
| `version` _string_ | Version optionally pins the install version. Mirrors SelfConfigSkill.Version<br />so HermesSelfConfig can carry the field through SSA without truncation. |  | Optional: \{\} <br /> |


#### LocalObjectReference



LocalObjectReference is a same-namespace reference by name.



_Appears in:_
- [BackupS3Spec](#backups3spec)
- [MigrationBackupS3](#migrationbackups3)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |


#### LogFormat

_Underlying type:_ _string_

LogFormat is the agent's log output format.

_Validation:_
- Enum: [text json]

_Appears in:_
- [LoggingSpec](#loggingspec)

| Field | Description |
| --- | --- |
| `text` |  |
| `json` |  |


#### LoggingSpec



LoggingSpec controls the agent's logger configuration via env vars.



_Appears in:_
- [ObservabilityDefaults](#observabilitydefaults)
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `format` _[LogFormat](#logformat)_ |  | text | Enum: [text json] <br />Optional: \{\} <br /> |
| `level` _string_ | Level: Plan 3 wires HERMES_LOG_LEVEL on the agent container. | info | Enum: [trace debug info warn error] <br />Optional: \{\} <br /> |


#### MetricsSpec



MetricsSpec controls the agent's Prometheus metrics endpoint.



_Appears in:_
- [ObservabilityDefaults](#observabilitydefaults)
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `port` _integer_ | Port for the /metrics endpoint. | 9090 | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `secure` _boolean_ | Secure: when true, /metrics requires bearer-token auth and uses HTTPS.<br />The ServiceMonitor scheme/scrape settings must agree (lesson #435/#440). | false | Optional: \{\} <br /> |
| `grafanaDashboard` _[GrafanaDashboardSpec](#grafanadashboardspec)_ | GrafanaDashboard configures auto-provisioned Grafana dashboard ConfigMaps<br />(operator overview + per-instance). When enabled, the operator emits<br />ConfigMaps labeled grafana_dashboard="1" so the Grafana sidecar provisioner<br />picks them up automatically. |  | Optional: \{\} <br /> |


#### MigrationBackupRef



MigrationBackupRef points at an OpenClaw backup snapshot in S3.



_Appears in:_
- [MigrationFromOpenClawSource](#migrationfromopenclawsource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[MigrationBackupS3](#migrationbackups3)_ |  |  |  |


#### MigrationBackupS3



MigrationBackupS3 mirrors BackupS3Spec but adds an explicit Key.



_Appears in:_
- [MigrationBackupRef](#migrationbackupref)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucket` _string_ |  |  |  |
| `endpoint` _string_ |  |  |  |
| `region` _string_ |  |  | Optional: \{\} <br /> |
| `key` _string_ |  |  |  |
| `credentialsSecretRef` _[LocalObjectReference](#localobjectreference)_ |  |  |  |


#### MigrationFromOpenClawSource



MigrationFromOpenClawSource is exactly-one-of (validated by webhook).



_Appears in:_
- [MigrationFromOpenClawSpec](#migrationfromopenclawspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `openclawInstanceRef` _[NamespacedObjectReference](#namespacedobjectreference)_ |  |  | Optional: \{\} <br /> |
| `backupRef` _[MigrationBackupRef](#migrationbackupref)_ |  |  | Optional: \{\} <br /> |


#### MigrationFromOpenClawSpec



MigrationFromOpenClawSpec describes an OpenClaw source.



_Appears in:_
- [MigrationSpec](#migrationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _[MigrationFromOpenClawSource](#migrationfromopenclawsource)_ |  |  |  |
| `mode` _string_ |  | copy | Enum: [copy move] <br />Optional: \{\} <br /> |
| `image` _string_ |  |  | Optional: \{\} <br /> |


#### MigrationSpec



MigrationSpec is a one-shot migration source (immutable once status.migration.completed is true).



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fromOpenClaw` _[MigrationFromOpenClawSpec](#migrationfromopenclawspec)_ |  |  | Optional: \{\} <br /> |


#### NamedServicePort



NamedServicePort is a single Service port. The TargetPort is optional and
defaults to Port when nil.



_Appears in:_
- [ServiceSpec](#servicespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MaxLength: 63 <br />MinLength: 1 <br /> |
| `port` _integer_ |  |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `targetPort` _integer_ |  |  | Optional: \{\} <br /> |
| `protocol` _[Protocol](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#protocol-v1-core)_ |  | TCP | Enum: [TCP UDP SCTP] <br />Optional: \{\} <br /> |
| `nodePort` _integer_ | NodePort is honored only when the Service is NodePort or LoadBalancer. |  | Optional: \{\} <br /> |


#### NamespacedObjectReference



NamespacedObjectReference is a name+namespace pointer.



_Appears in:_
- [MigrationFromOpenClawSource](#migrationfromopenclawsource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `namespace` _string_ |  |  |  |


#### NetworkPolicyDefaults



NetworkPolicyDefaults defaults whether per-instance NetworkPolicies are created.



_Appears in:_
- [NetworkingDefaults](#networkingdefaults)
- [SecurityDefaults](#securitydefaults)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  | Optional: \{\} <br /> |
| `allowDNS` _boolean_ |  |  | Optional: \{\} <br /> |


#### NetworkPolicySpec



NetworkPolicySpec controls per-instance NetworkPolicy creation.



_Appears in:_
- [SecuritySpec](#securityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled: when true (the default), the operator creates a deny-all<br />NetworkPolicy plus selective allow rules (DNS + 443 egress + Service ingress<br />from the same namespace). | true | Optional: \{\} <br /> |
| `allowDNS` _boolean_ | AllowDNS: emit the standard DNS egress rule (UDP+TCP 53 to any peer).<br />Default true. Disable only when CoreDNS is reachable via a different<br />transport (e.g. node-local DNS via hostNetwork). | true | Optional: \{\} <br /> |
| `allowedIngressNamespaces` _string array_ | AllowedIngressNamespaces is the set of additional namespaces (beyond the<br />instance's own) whose pods may connect to the agent's exposed ports. |  | Optional: \{\} <br /> |
| `allowedIngressCIDRs` _string array_ | AllowedIngressCIDRs is the set of CIDRs that may connect to the agent's<br />exposed ports. |  | Optional: \{\} <br /> |
| `allowedEgressCIDRs` _string array_ | AllowedEgressCIDRs is the set of CIDRs the agent may connect to in addition<br />to the operator-built defaults (DNS + 443). |  | Optional: \{\} <br /> |
| `additionalEgress` _[NetworkPolicyEgressRule](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#networkpolicyegressrule-v1-networking) array_ | AdditionalEgress is a list of user-supplied egress rules appended verbatim<br />to the generated NetworkPolicy. |  | Optional: \{\} <br /> |


#### NetworkingDefaults



NetworkingDefaults mirrors the defaultable subset of NetworkingSpec.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _[ServiceDefaults](#servicedefaults)_ |  |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicyDefaults](#networkpolicydefaults)_ |  |  | Optional: \{\} <br /> |


#### NetworkingSpec



NetworkingSpec exposes the agent via Service + (optionally) Ingress.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _[ServiceSpec](#servicespec)_ | Service controls the Service kind and ports. |  | Optional: \{\} <br /> |
| `ingress` _[IngressSpec](#ingressspec)_ | Ingress controls optional Ingress creation. |  | Optional: \{\} <br /> |
| `httpRoute` _[HTTPRouteSpec](#httproutespec)_ | HTTPRoute controls optional Gateway API HTTPRoute creation. The operator<br />emits an unstructured gateway.networking.k8s.io/v1 HTTPRoute; the Gateway<br />API CRDs must be installed in the cluster for this to take effect. |  | Optional: \{\} <br /> |


#### ObservabilityDefaults



ObservabilityDefaults mirrors the defaultable subset of ObservabilitySpec.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metrics` _[MetricsSpec](#metricsspec)_ |  |  | Optional: \{\} <br /> |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ |  |  | Optional: \{\} <br /> |
| `prometheusRule` _[PrometheusRuleSpec](#prometheusrulespec)_ |  |  | Optional: \{\} <br /> |
| `logging` _[LoggingSpec](#loggingspec)_ |  |  | Optional: \{\} <br /> |


#### ObservabilitySpec



ObservabilitySpec controls metrics, scraping, alerting, logging.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metrics` _[MetricsSpec](#metricsspec)_ |  |  | Optional: \{\} <br /> |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ |  |  | Optional: \{\} <br /> |
| `prometheusRule` _[PrometheusRuleSpec](#prometheusrulespec)_ |  |  | Optional: \{\} <br /> |
| `logging` _[LoggingSpec](#loggingspec)_ |  |  | Optional: \{\} <br /> |


#### PDBSpec



PDBSpec controls PodDisruptionBudget emission.



_Appears in:_
- [AvailabilitySpec](#availabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MinAvailable: optional, mutually exclusive with MaxUnavailable. |  | Optional: \{\} <br /> |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MaxUnavailable: optional, mutually exclusive with MinAvailable.<br />Default 1 when neither is set and PDB is enabled. |  | Optional: \{\} <br /> |


#### PersistenceSpec







_Appears in:_
- [StorageSpec](#storagespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `size` _string_ |  | 1Gi | Optional: \{\} <br /> |
| `storageClassName` _string_ |  |  | Optional: \{\} <br /> |


#### ProbesSpec



ProbesSpec overrides the operator's built-in probes. Each field is a complete
probe: set every value you want non-default because we apply it verbatim.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `liveness` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  | Optional: \{\} <br /> |
| `readiness` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  | Optional: \{\} <br /> |
| `startup` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#probe-v1-core)_ |  |  | Optional: \{\} <br /> |


#### ProfileStoreSpec



ProfileStoreSpec is the union of supported profile-store backends. Only
`honcho` is supported in v1.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `honcho` _[HonchoSpec](#honchospec)_ |  |  | Optional: \{\} <br /> |


#### PrometheusRule



PrometheusRule is a minimal copy of monitoringv1.Rule so we don't depend on
the Prometheus-Operator Go types at compile time. The runtime emits
unstructured objects.



_Appears in:_
- [PrometheusRuleSpec](#prometheusrulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `alert` _string_ |  |  | MinLength: 1 <br /> |
| `expr` _string_ |  |  | MinLength: 1 <br /> |
| `for` _string_ |  |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |


#### PrometheusRuleSpec



PrometheusRuleSpec controls emission of a default PrometheusRule with
hermes-agent alerts (HighRestartRate, MetricsDown, etc.).



_Appears in:_
- [ObservabilityDefaults](#observabilitydefaults)
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `additionalRules` _[PrometheusRule](#prometheusrule) array_ | AdditionalRules is a list of user-supplied rules merged onto the operator<br />default ruleset. |  | Optional: \{\} <br /> |


#### RBACSpec



RBACSpec controls per-instance ServiceAccount + Role + RoleBinding creation.



_Appears in:_
- [SecuritySpec](#securityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `createServiceAccount` _boolean_ | CreateServiceAccount: when true (the default), the operator creates and<br />owns a ServiceAccount named after the instance. | true | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName: when CreateServiceAccount is false, the agent uses<br />this externally-managed ServiceAccount. Must exist in the same namespace. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are applied to the operator-created ServiceAccount. Use this<br />for IRSA (`eks.amazonaws.com/role-arn`), GKE Workload Identity<br />(`iam.gke.io/gcp-service-account`), Azure Workload Identity, etc. |  | Optional: \{\} <br /> |


#### RawConfig



RawConfig wraps runtime.RawExtension so deepcopy is generated cleanly.



_Appears in:_
- [ConfigSpec](#configspec)



#### RegistryDefaults



RegistryDefaults groups image-pull secret hints.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pullSecretName` _string_ | PullSecretName, if non-empty, is added to every instance's<br />pod.spec.imagePullSecrets when the instance doesn't override. |  | Optional: \{\} <br /> |


#### ResourcesSpec



ResourcesSpec sets CPU/memory requests + limits on the agent container.
Defaults intentionally omitted: the defaulting webhook fills from
HermesClusterDefaults if available, otherwise the field is left empty
(meaning the agent inherits whatever Pod-level defaults the namespace's
LimitRange applies).



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `applyOperatorDefaults` _boolean_ | ApplyOperatorDefaults controls the operator's built-in fallback for the<br />agent container. When requests or limits are left unset, the operator<br />fills that side in so an agent executing model-driven code cannot run<br />unbounded and starve its node. Unset means true.<br />Set false to keep a side genuinely unbounded, for example a long-lived<br />instance whose working set is known to exceed the default limits and<br />which would otherwise be OOM-killed after an operator upgrade.<br />Whichever side you set explicitly is used verbatim either way; this flag<br />only governs the side you leave unset. |  | Optional: \{\} <br /> |


#### RipgrepSpec



RipgrepSpec controls the ripgrep dependency check.



_Appears in:_
- [RuntimeSpec](#runtimespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |


#### RuntimeSpec



RuntimeSpec controlled Python/uv runtime concerns for the old hand-rolled agent
image's init-container build.

Deprecated: ignored since the upstream-image runtime (v0.1.19); the upstream
agent image is self-contained. Scheduled for removal no earlier than v0.3.0 and
2027-01-01. See docs/deprecations.md.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `python` _string_ | Python is informational only: the agent image's Python version is fixed<br />at build time. Setting this does NOT pull a different interpreter; it<br />exists so downstream tooling can assert the runtime it expects. | 3.11 | Optional: \{\} <br /> |
| `uv` _[UVSpec](#uvspec)_ | UV controls the initial `uv sync` against the lockfile bundled in the<br />agent image. Enabled by default. |  | Optional: \{\} <br /> |
| `ffmpeg` _[FFmpegSpec](#ffmpegspec)_ | FFmpeg toggles the FFmpeg dependency check. The agent image always ships<br />FFmpeg; disabling here only skips the readiness assertion. |  | Optional: \{\} <br /> |
| `ripgrep` _[RipgrepSpec](#ripgrepspec)_ | Ripgrep toggles the ripgrep dependency check. See FFmpeg. |  | Optional: \{\} <br /> |
| `extraAptPackages` _string array_ | ExtraAptPackages adds additional Debian packages installed by a<br />root-privileged init container BEFORE the main agent container starts.<br />Use sparingly: the init container runs as root and breaks the otherwise<br />hardened security posture for one container only. |  | Optional: \{\} <br /> |
| `extraPipPackages` _string array_ | ExtraPipPackages adds additional Python packages installed via<br />`uv pip install` into a persistent venv on the data PVC. |  | Optional: \{\} <br /> |


#### SchedulingSpec



SchedulingSpec targets the agent pod at specific nodes.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeSelector` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#toleration-v1-core) array_ |  |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#affinity-v1-core)_ |  |  | Optional: \{\} <br /> |
| `priorityClassName` _string_ |  |  | Optional: \{\} <br /> |
| `runtimeClassName` _string_ | RuntimeClassName selects the RuntimeClass the instance pod runs under,<br />so an agent executing model-driven code can be placed on a sandboxed<br />runtime (gVisor "runsc", Kata) instead of the cluster default.<br />The named RuntimeClass must already exist in the cluster; an unknown name<br />leaves the pod unschedulable. Empty means the cluster default runtime. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$` <br />Optional: \{\} <br /> |


#### SecurityDefaults



SecurityDefaults mirrors the defaultable subset of SecuritySpec.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceAccount` _[ServiceAccountDefaults](#serviceaccountdefaults)_ |  |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicyDefaults](#networkpolicydefaults)_ |  |  | Optional: \{\} <br /> |
| `caBundle` _[CABundleSpec](#cabundlespec)_ |  |  | Optional: \{\} <br /> |


#### SecuritySpec



SecuritySpec bundles pod/container security, per-instance RBAC, NetworkPolicy,
and the optional CA-bundle mount.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#podsecuritycontext-v1-core)_ | PodSecurityContext overrides the operator's default hardened pod context.<br />Operator default is enforced when nil: runAsNonRoot=true, runAsUser=1000,<br />fsGroup=1000, seccompProfile=RuntimeDefault. |  | Optional: \{\} <br /> |
| `containerSecurityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#securitycontext-v1-core)_ | ContainerSecurityContext overrides the operator's default hardened container<br />context. Operator default: readOnlyRootFilesystem=true, allowPrivilegeEscalation=false,<br />drop ALL capabilities. |  | Optional: \{\} <br /> |
| `rbac` _[RBACSpec](#rbacspec)_ | RBAC controls per-instance ServiceAccount + Role + RoleBinding creation. |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicySpec](#networkpolicyspec)_ | NetworkPolicy controls per-instance NetworkPolicy creation (default-deny baseline). |  | Optional: \{\} <br /> |
| `caBundle` _[CABundleSpec](#cabundlespec)_ | CABundle optionally mounts a ConfigMap- or Secret-sourced CA bundle into<br />/etc/ssl/certs/hermes-ca-bundle.crt and sets SSL_CERT_FILE in the agent env. |  | Optional: \{\} <br /> |


#### SelfConfigAction

_Underlying type:_ _string_

SelfConfigAction names a category of mutation. Used by
HermesInstance.spec.selfConfigure.allowedActions to gate what the agent
may request via HermesSelfConfig.

_Validation:_
- Enum: [skills config envVars workspaceFiles profiles]

_Appears in:_
- [SelfConfigureSpec](#selfconfigurespec)

| Field | Description |
| --- | --- |
| `skills` |  |
| `config` |  |
| `envVars` |  |
| `workspaceFiles` |  |
| `profiles` |  |




#### SelfConfigEnvVar



SelfConfigEnvVar is an environment variable entry.



_Appears in:_
- [HermesSelfConfigSpec](#hermesselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the environment variable. Must be a C_IDENTIFIER. |  | MinLength: 1 <br />Pattern: `^[A-Za-z_][A-Za-z0-9_]*$` <br /> |
| `value` _string_ | Value is the literal value. Mutually exclusive with ValueFrom. |  | Optional: \{\} <br /> |
| `valueFrom` _[SelfConfigEnvVarSource](#selfconfigenvvarsource)_ | ValueFrom selects a value from a Secret or ConfigMap key. |  | Optional: \{\} <br /> |


#### SelfConfigEnvVarSource



SelfConfigEnvVarSource selects a Secret or ConfigMap key. Exactly one ref must be set.



_Appears in:_
- [SelfConfigEnvVar](#selfconfigenvvar)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretKeyRef` _[SelfConfigKeySelector](#selfconfigkeyselector)_ |  |  | Optional: \{\} <br /> |
| `configMapKeyRef` _[SelfConfigKeySelector](#selfconfigkeyselector)_ |  |  | Optional: \{\} <br /> |


#### SelfConfigKeySelector



SelfConfigKeySelector selects a key from a Secret or ConfigMap.



_Appears in:_
- [SelfConfigEnvVarSource](#selfconfigenvvarsource)
- [SelfConfigWorkspaceFile](#selfconfigworkspacefile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `key` _string_ |  |  | MinLength: 1 <br /> |




#### SelfConfigProfileSnapshot



SelfConfigProfileSnapshot writes one Honcho profile snapshot via a Job.



_Appears in:_
- [HermesSelfConfigSpec](#hermesselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `profileID` _string_ |  |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `data` _string_ | Data is the opaque snapshot payload. |  | MinLength: 1 <br /> |


#### SelfConfigSkill



SelfConfigSkill names one skill to install.



_Appears in:_
- [HermesSelfConfigSpec](#hermesselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ | Source is a uv-compatible package specifier. Required. |  | MaxLength: 512 <br />MinLength: 1 <br /> |
| `version` _string_ | Version optionally pins a version. |  | Optional: \{\} <br /> |


#### SelfConfigWorkspaceFile



SelfConfigWorkspaceFile is a single file to materialise into the workspace.



_Appears in:_
- [HermesSelfConfigSpec](#hermesselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path is the relative path under ~/.hermes/workspace/. |  | MaxLength: 512 <br />MinLength: 1 <br />Pattern: `^[A-Za-z0-9._/-]+$` <br /> |
| `content` _string_ | Content is the literal file body. |  | Optional: \{\} <br /> |
| `contentFrom` _[SelfConfigKeySelector](#selfconfigkeyselector)_ | ContentFrom reads the file body from a Secret key. |  | Optional: \{\} <br /> |


#### SelfConfigureSpec



SelfConfigureSpec is the allowlist policy for HermesSelfConfig mutations.
Plan 4 wires the controller; the field exists here so Plan 4 doesn't need a
CRD change. The validator rejects Enabled=true with ProtectedKeys empty.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled: explicit *bool so the defaulter can distinguish "user said false"<br />from "user did not set it" (Plan 4 relies on this). |  | Optional: \{\} <br /> |
| `allowedActions` _[SelfConfigAction](#selfconfigaction) array_ | AllowedActions is the set of permitted action categories Plan 4 will<br />enforce: skills, config, envVars, workspaceFiles, profiles. |  | Enum: [skills config envVars workspaceFiles profiles] <br />Optional: \{\} <br /> |
| `protectedKeys` _string array_ | ProtectedKeys is the list of glob expressions over JSON paths that may<br />not be mutated by HermesSelfConfig. Required (non-empty) when Enabled=true. |  | Optional: \{\} <br /> |


#### ServiceAccountDefaults



ServiceAccountDefaults defaults the per-instance SA annotations (IRSA / WI).



_Appears in:_
- [SecurityDefaults](#securitydefaults)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `annotations` _object (keys:string, values:string)_ |  |  | Optional: \{\} <br /> |


#### ServiceDefaults



ServiceDefaults defaults the Service kind cluster-wide.



_Appears in:_
- [NetworkingDefaults](#networkingdefaults)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#servicetype-v1-core)_ |  |  | Enum: [ClusterIP NodePort LoadBalancer] <br />Optional: \{\} <br /> |


#### ServiceMonitorSpec



ServiceMonitorSpec controls Prometheus-Operator ServiceMonitor emission.
When Enabled is true, the operator emits an unstructured ServiceMonitor; it
does not require the Prometheus-Operator CRDs to be installed at compile time.



_Appears in:_
- [ObservabilityDefaults](#observabilitydefaults)
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are extra labels applied onto the ServiceMonitor for Prometheus<br />label-selector matching (e.g. `release: kube-prometheus-stack`). |  | Optional: \{\} <br /> |
| `interval` _string_ | Interval: default "30s". | 30s | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Optional: \{\} <br /> |
| `scrapeTimeout` _string_ | ScrapeTimeout: default "10s". | 10s | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Optional: \{\} <br /> |


#### ServiceSpec



ServiceSpec controls the agent's Service.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#servicetype-v1-core)_ | Type is the Service kind. Default ClusterIP (headed): Plan 1 emitted a<br />headless Service; v1 keeps ClusterIP as the default and lets users opt<br />into Headless via Type=ClusterIP with ClusterIP="None" through the spec. | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br />Optional: \{\} <br /> |
| `clusterIP` _string_ | ClusterIP: set to "None" for a headless Service. Default empty (api-server allocates). |  | Optional: \{\} <br /> |
| `ports` _[NamedServicePort](#namedserviceport) array_ | Ports is the list of Service ports. If empty, the operator emits a default<br />"gateway" port on 8443 (matches the StatefulSet's container port). |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are applied verbatim onto the Service (LoadBalancer hints, etc.). |  | Optional: \{\} <br /> |
| `loadBalancerClass` _string_ | LoadBalancerClass is propagated when Type=LoadBalancer. |  | Optional: \{\} <br /> |
| `externalTrafficPolicy` _[ServiceExternalTrafficPolicyType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceexternaltrafficpolicytype-v1-core)_ | ExternalTrafficPolicy is propagated when Type=LoadBalancer or NodePort. |  | Enum: [Cluster Local] <br />Optional: \{\} <br /> |


#### SignalGatewaySpec



SignalGatewaySpec binds the agent to signal-cli-rest-api running as a sidecar
or external service.



_Appears in:_
- [GatewaysSpec](#gatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `phoneNumberSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |
| `authTokenSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |


#### SlackGatewaySpec



SlackGatewaySpec binds the agent to a Slack workspace via the bolt SDK.



_Appears in:_
- [GatewaysSpec](#gatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `botTokenSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |
| `appTokenSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |
| `signingSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |


#### StorageSpec



StorageSpec controls the PVC backing the agent's data directory.



_Appears in:_
- [HermesClusterDefaultsSpec](#hermesclusterdefaultsspec)
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `persistence` _[PersistenceSpec](#persistencespec)_ |  |  |  |


#### TailscaleAuthKey



TailscaleAuthKey points at the Secret key holding the Tailscale auth key.



_Appears in:_
- [TailscaleSpec](#tailscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |


#### TailscaleImageSpec



TailscaleImageSpec pins the tailscale sidecar image.



_Appears in:_
- [TailscaleSpec](#tailscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ |  | tailscale/tailscale | Optional: \{\} <br /> |
| `tag` _string_ |  | v1.86.2 | Optional: \{\} <br /> |
| `pullPolicy` _string_ |  | IfNotPresent | Enum: [Always IfNotPresent Never] <br />Optional: \{\} <br /> |


#### TailscaleSpec



TailscaleSpec configures exposing the hermes gateway over a Tailscale tailnet.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns on the operator-managed Tailscale sidecar. | false | Optional: \{\} <br /> |
| `mode` _string_ | Mode selects how the gateway is exposed over the tailnet. Only "serve"<br />is implemented today (private tailnet exposure with a Tailscale TLS cert). | serve | Enum: [serve] <br />Optional: \{\} <br /> |
| `authKey` _[TailscaleAuthKey](#tailscaleauthkey)_ | AuthKey references the Secret holding a reusable, ephemeral Tailscale auth<br />key, exposed to the sidecar as TS_AUTHKEY. Required when Enabled is true. |  | Optional: \{\} <br /> |
| `hostname` _string_ | Hostname overrides the tailnet/MagicDNS hostname. Defaults to metadata.name. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `image` _[TailscaleImageSpec](#tailscaleimagespec)_ | Image overrides the tailscale sidecar image. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | Resources sets the sidecar resource requirements. |  | Optional: \{\} <br /> |


#### TelegramGatewaySpec



TelegramGatewaySpec binds the agent to a Telegram Bot API token.



_Appears in:_
- [GatewaysSpec](#gatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `botTokenSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ | BotTokenSecretRef points at the Secret holding the Bot API token.<br />Required when Enabled. |  | Optional: \{\} <br /> |
| `allowedUserIDs` _integer array_ | AllowedUserIDs is an optional allow-list of Telegram user IDs. |  | Optional: \{\} <br /> |
| `webhookURL` _string_ | WebhookURL is the public HTTPS URL to register with Telegram. When empty<br />the agent runs in long-poll mode. |  | Optional: \{\} <br /> |


#### UVCacheVolumeSpec



UVCacheVolumeSpec mirrors a stripped-down VolumeSource union. Exactly one of
EmptyDir or PersistentVolumeClaim may be set; the defaulter fills EmptyDir
when both are nil.



_Appears in:_
- [UVSpec](#uvspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#emptydirvolumesource-v1-core)_ |  |  | Optional: \{\} <br /> |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#persistentvolumeclaimvolumesource-v1-core)_ |  |  | Optional: \{\} <br /> |


#### UVSpec



UVSpec controls the `uv sync` init container.



_Appears in:_
- [RuntimeSpec](#runtimespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | true | Optional: \{\} <br /> |
| `extraIndexURL` _string_ | ExtraIndexURL is appended to uv's index list. Useful for private PyPI<br />mirrors. Empty by default. |  | Optional: \{\} <br /> |
| `cacheVolume` _[UVCacheVolumeSpec](#uvcachevolumespec)_ | CacheVolume controls the volume mounted at /home/hermes/.cache/uv.<br />Defaults to an emptyDir with a 1Gi sizeLimit: fast and ephemeral. |  | Optional: \{\} <br /> |


#### WhatsAppGatewaySpec



WhatsAppGatewaySpec binds the agent to a WhatsApp provider (Twilio,
Meta Cloud API, etc.).



_Appears in:_
- [GatewaysSpec](#gatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  | false | Optional: \{\} <br /> |
| `providerSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ |  |  | Optional: \{\} <br /> |


#### WorkspaceBootstrap



WorkspaceBootstrap toggles the first-start bootstrap script.



_Appears in:_
- [WorkspaceSpec](#workspacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled: default false. Plan 3 wires the actual init-container. | false | Optional: \{\} <br /> |


#### WorkspaceFile



WorkspaceFile is a single seeded file. Nested paths are allowed; the workspace
ConfigMap encodes them with "__" separators (decoded by runtime-init).



_Appears in:_
- [WorkspaceSpec](#workspacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path is the relative path under ~/.hermes (e.g. "notes/finance/2026.md"). |  | MaxLength: 4096 <br />MinLength: 1 <br />Pattern: `^[^/].*[^/]$\|^[^/]$` <br /> |
| `content` _string_ | Content is the UTF-8 body. Binary content must be base64-encoded by the<br />caller and decoded by the bootstrap step (out of scope of v1 schema). |  | MaxLength: 1048576 <br /> |


#### WorkspaceSpec



WorkspaceSpec seeds initial files and directories into ~/.hermes on first
start. Path values support arbitrary nested directories ("a/b/c.md" is fine);
the workspace ConfigMap encodes nested paths using "__" as the separator so a
single-level ConfigMap data map can express them: Plan 3's runtime-init
container decodes the keys back to filesystem paths before invoking the agent.

Lesson from openclaw #482: do not constrain Path to a single segment; that
caused users to flatten their notes into hash-separated filenames.



_Appears in:_
- [HermesInstanceSpec](#hermesinstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialFiles` _[WorkspaceFile](#workspacefile) array_ | InitialFiles is the list of files to seed.<br />SSA list-map key is "path" so HermesSelfConfig (Plan 4) can patch entries<br />in place without replacing the whole slice. |  | Optional: \{\} <br /> |
| `initialDirs` _string array_ | InitialDirs is the list of directories to mkdir -p on first start. |  | Optional: \{\} <br /> |
| `configMapRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core)_ | ConfigMapRef references a user-owned ConfigMap whose entries are merged<br />onto InitialFiles (operator-managed entries win on conflict). |  | Optional: \{\} <br /> |
| `bootstrap` _[WorkspaceBootstrap](#workspacebootstrap)_ | Bootstrap controls the optional one-shot bootstrap script that hermes-agent<br />runs on first start (e.g. `hermes onboard`). Default disabled. |  | Optional: \{\} <br /> |


