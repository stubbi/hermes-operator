/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HermesInstanceSpec defines the desired state of HermesInstance.
// Field order follows design §4.
type HermesInstanceSpec struct {
	// Image selects the hermes-agent container image.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Config is the YAML content of ~/.hermes/config.yaml, supplied inline,
	// from a referenced ConfigMap, or merged from both.
	// +optional
	Config ConfigSpec `json:"config,omitempty"`

	// Workspace seeds initial files and directories into ~/.hermes on first start.
	// +optional
	Workspace WorkspaceSpec `json:"workspace,omitempty"`

	// Resources sets the agent container's CPU/memory requests + limits.
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`

	// Security configures pod/container security contexts, RBAC, NetworkPolicy,
	// and the optional cluster CA bundle injection.
	// +optional
	Security SecuritySpec `json:"security,omitempty"`

	// Storage controls the PVC backing ~/.hermes for this instance.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Networking exposes the agent via Service / Ingress.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	// Observability turns on metrics, ServiceMonitor, PrometheusRule, and logging.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Availability sets PDB, HPA, and topology-spread constraints.
	// +optional
	Availability AvailabilitySpec `json:"availability,omitempty"`

	// Probes lets users override the built-in liveness/readiness/startup probes.
	// +optional
	Probes ProbesSpec `json:"probes,omitempty"`

	// Scheduling targets the agent pod at specific nodes.
	// +optional
	Scheduling SchedulingSpec `json:"scheduling,omitempty"`

	// InitContainers is a user-supplied list of init containers appended after
	// any operator-managed init containers (e.g. runtime-init from Plan 3).
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Sidecars is a user-supplied list of sidecars appended after operator-managed
	// sidecars (e.g. ollama / web-terminal / tailscale from Plan 3).
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// ExtraVolumes is a user-supplied list of additional pod volumes.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts is a user-supplied list of additional volume mounts
	// applied to the agent container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// EnvFrom is a list of EnvFrom sources (ConfigMap/Secret refs) injected
	// into the agent container.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Env is a list of explicit environment variables for the agent container.
	// SSA list-map key is "name" so HermesSelfConfig can merge entries without
	// replacing the whole list.
	// +listType=map
	// +listMapKey=name
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Skills is the declarative list of uv-installable skill sources. Plan 3
	// wires the runtime; the field is declared here so SSA from HermesSelfConfig
	// (Plan 4) can target it without a CRD schema change.
	// +listType=map
	// +listMapKey=source
	// +optional
	Skills []InstanceSkill `json:"skills,omitempty"`

	// SelfConfigure is the allowlist policy for HermesSelfConfig mutations.
	// +optional
	SelfConfigure SelfConfigureSpec `json:"selfConfigure,omitempty"`

	// Suspended scales the StatefulSet to zero replicas without deleting state.
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// Backup controls scheduled and on-delete PVC snapshot behaviour.
	// +optional
	Backup BackupSpec `json:"backup,omitempty"`

	// RestoreFrom names a backup snapshot to restore from on next boot.
	// +optional
	RestoreFrom string `json:"restoreFrom,omitempty"`

	// AutoUpdate controls opt-in OCI-registry polling for newer agent images.
	// +optional
	AutoUpdate AutoUpdateSpec `json:"autoUpdate,omitempty"`

	// Migration is a one-shot migration source (set on initial create only).
	// +optional
	Migration MigrationSpec `json:"migration,omitempty"`

	// Runtime controls the agent's Python toolchain and OS-level dependencies.
	// All fields default to the values that match the operator's published
	// ghcr.io/stubbi/hermes-agent image.
	// +optional
	Runtime RuntimeSpec `json:"runtime,omitempty"`

	// Gateways configures the platform-side messaging bindings (Telegram, Discord,
	// Slack, WhatsApp, Signal). Each gateway is opt-in and references its own
	// Secret(s) so tokens are rotatable independently.
	// +optional
	Gateways GatewaysSpec `json:"gateways,omitempty"`

	// ProfileStore configures the optional Honcho profile-store companion.
	// +optional
	ProfileStore ProfileStoreSpec `json:"profileStore,omitempty"`
}

// ImageSpec selects an OCI image.
type ImageSpec struct {
	// +kubebuilder:default="ghcr.io/stubbi/hermes-agent"
	// +optional
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:default="latest"
	// +optional
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// StorageSpec controls the PVC backing the agent's data directory.
type StorageSpec struct {
	Persistence PersistenceSpec `json:"persistence,omitempty"`
}

type PersistenceSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +kubebuilder:default="1Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// ConfigMergeMode controls how Raw and ConfigMapRef are combined.
// +kubebuilder:validation:Enum=replace;merge
type ConfigMergeMode string

const (
	// ConfigMergeModeReplace: Raw replaces ConfigMapRef entirely when both are set.
	// This is the default to avoid surprising merges.
	ConfigMergeModeReplace ConfigMergeMode = "replace"
	// ConfigMergeModeMerge: YAML deep-merge Raw onto ConfigMapRef. Raw wins on conflict.
	ConfigMergeModeMerge ConfigMergeMode = "merge"
)

// ConfigSpec holds the agent's ~/.hermes/config.yaml. Exactly one of Raw or
// ConfigMapRef SHOULD be set; the validating webhook rejects both unset and
// emits a warning if both are set with MergeMode unset.
type ConfigSpec struct {
	// Raw is the inline YAML body of config.yaml. Stored as a RawExtension so
	// users may write structured YAML in the manifest without escaping.
	// +optional
	Raw *RawConfig `json:"raw,omitempty"`

	// ConfigMapRef references a ConfigMap in the same namespace whose
	// "config.yaml" key holds the body.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// MergeMode controls combination when both Raw and ConfigMapRef are set.
	// +kubebuilder:default=replace
	// +optional
	MergeMode ConfigMergeMode `json:"mergeMode,omitempty"`
}

// +kubebuilder:object:generate=true

// RawConfig wraps runtime.RawExtension so deepcopy is generated cleanly.
type RawConfig struct {
	runtime.RawExtension `json:",inline"`
}

// WorkspaceSpec seeds initial files and directories into ~/.hermes on first
// start. Path values support arbitrary nested directories ("a/b/c.md" is fine);
// the workspace ConfigMap encodes nested paths using "__" as the separator so a
// single-level ConfigMap data map can express them: Plan 3's runtime-init
// container decodes the keys back to filesystem paths before invoking the agent.
//
// Lesson from openclaw #482: do not constrain Path to a single segment; that
// caused users to flatten their notes into hash-separated filenames.
type WorkspaceSpec struct {
	// InitialFiles is the list of files to seed.
	// SSA list-map key is "path" so HermesSelfConfig (Plan 4) can patch entries
	// in place without replacing the whole slice.
	// +listType=map
	// +listMapKey=path
	// +optional
	InitialFiles []WorkspaceFile `json:"initialFiles,omitempty"`

	// InitialDirs is the list of directories to mkdir -p on first start.
	// +listType=set
	// +optional
	InitialDirs []string `json:"initialDirs,omitempty"`

	// ConfigMapRef references a user-owned ConfigMap whose entries are merged
	// onto InitialFiles (operator-managed entries win on conflict).
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// Bootstrap controls the optional one-shot bootstrap script that hermes-agent
	// runs on first start (e.g. `hermes onboard`). Default disabled.
	// +optional
	Bootstrap WorkspaceBootstrap `json:"bootstrap,omitempty"`
}

// WorkspaceFile is a single seeded file. Nested paths are allowed; the workspace
// ConfigMap encodes them with "__" separators (decoded by runtime-init).
type WorkspaceFile struct {
	// Path is the relative path under ~/.hermes (e.g. "notes/finance/2026.md").
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	// +kubebuilder:validation:Pattern=`^[^/].*[^/]$|^[^/]$`
	Path string `json:"path"`

	// Content is the UTF-8 body. Binary content must be base64-encoded by the
	// caller and decoded by the bootstrap step (out of scope of v1 schema).
	// +kubebuilder:validation:MaxLength=1048576
	Content string `json:"content"`
}

// WorkspaceBootstrap toggles the first-start bootstrap script.
type WorkspaceBootstrap struct {
	// Enabled: default false. Plan 3 wires the actual init-container.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// ResourcesSpec sets CPU/memory requests + limits on the agent container.
// Defaults intentionally omitted: the defaulting webhook fills from
// HermesClusterDefaults if available, otherwise the field is left empty
// (meaning the agent inherits whatever Pod-level defaults the namespace's
// LimitRange applies).
type ResourcesSpec struct {
	// Requests is the resource-requests map.
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`

	// Limits is the resource-limits map.
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// ToContainerResourceRequirements converts to a corev1.ResourceRequirements,
// useful inside resource builders.
func (r *ResourcesSpec) ToContainerResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}
}

// SecuritySpec bundles pod/container security, per-instance RBAC, NetworkPolicy,
// and the optional CA-bundle mount.
type SecuritySpec struct {
	// PodSecurityContext overrides the operator's default hardened pod context.
	// Operator default is enforced when nil: runAsNonRoot=true, runAsUser=1000,
	// fsGroup=1000, seccompProfile=RuntimeDefault.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ContainerSecurityContext overrides the operator's default hardened container
	// context. Operator default: readOnlyRootFilesystem=true, allowPrivilegeEscalation=false,
	// drop ALL capabilities.
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// RBAC controls per-instance ServiceAccount + Role + RoleBinding creation.
	// +optional
	RBAC RBACSpec `json:"rbac,omitempty"`

	// NetworkPolicy controls per-instance NetworkPolicy creation (default-deny baseline).
	// +optional
	NetworkPolicy NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// CABundle optionally mounts a ConfigMap- or Secret-sourced CA bundle into
	// /etc/ssl/certs/hermes-ca-bundle.crt and sets SSL_CERT_FILE in the agent env.
	// +optional
	CABundle CABundleSpec `json:"caBundle,omitempty"`
}

// RBACSpec controls per-instance ServiceAccount + Role + RoleBinding creation.
type RBACSpec struct {
	// CreateServiceAccount: when true (the default), the operator creates and
	// owns a ServiceAccount named after the instance.
	// +kubebuilder:default=true
	// +optional
	CreateServiceAccount *bool `json:"createServiceAccount,omitempty"`

	// ServiceAccountName: when CreateServiceAccount is false, the agent uses
	// this externally-managed ServiceAccount. Must exist in the same namespace.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Annotations are applied to the operator-created ServiceAccount. Use this
	// for IRSA (`eks.amazonaws.com/role-arn`), GKE Workload Identity
	// (`iam.gke.io/gcp-service-account`), Azure Workload Identity, etc.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NetworkPolicySpec controls per-instance NetworkPolicy creation.
type NetworkPolicySpec struct {
	// Enabled: when true (the default), the operator creates a deny-all
	// NetworkPolicy plus selective allow rules (DNS + 443 egress + Service ingress
	// from the same namespace).
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AllowDNS: emit the standard DNS egress rule (UDP+TCP 53 to any peer).
	// Default true. Disable only when CoreDNS is reachable via a different
	// transport (e.g. node-local DNS via hostNetwork).
	// +kubebuilder:default=true
	// +optional
	AllowDNS *bool `json:"allowDNS,omitempty"`

	// AllowedIngressNamespaces is the set of additional namespaces (beyond the
	// instance's own) whose pods may connect to the agent's exposed ports.
	// +listType=set
	// +optional
	AllowedIngressNamespaces []string `json:"allowedIngressNamespaces,omitempty"`

	// AllowedIngressCIDRs is the set of CIDRs that may connect to the agent's
	// exposed ports.
	// +listType=set
	// +optional
	AllowedIngressCIDRs []string `json:"allowedIngressCIDRs,omitempty"`

	// AllowedEgressCIDRs is the set of CIDRs the agent may connect to in addition
	// to the operator-built defaults (DNS + 443).
	// +listType=set
	// +optional
	AllowedEgressCIDRs []string `json:"allowedEgressCIDRs,omitempty"`

	// AdditionalEgress is a list of user-supplied egress rules appended verbatim
	// to the generated NetworkPolicy.
	// +optional
	AdditionalEgress []networkingv1.NetworkPolicyEgressRule `json:"additionalEgress,omitempty"`
}

// CABundleSpec optionally mounts a CA bundle into the agent container.
// Exactly one of ConfigMapName / SecretName SHOULD be set.
type CABundleSpec struct {
	// ConfigMapName references a ConfigMap in the same namespace.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// SecretName references a Secret in the same namespace.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Key is the data-map key holding the PEM bundle. Default "ca.crt".
	// +kubebuilder:default="ca.crt"
	// +optional
	Key string `json:"key,omitempty"`
}

// NetworkingSpec exposes the agent via Service + (optionally) Ingress.
type NetworkingSpec struct {
	// Service controls the Service kind and ports.
	// +optional
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress controls optional Ingress creation.
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`
}

// ServiceSpec controls the agent's Service.
type ServiceSpec struct {
	// Type is the Service kind. Default ClusterIP (headed): Plan 1 emitted a
	// headless Service; v1 keeps ClusterIP as the default and lets users opt
	// into Headless via Type=ClusterIP with ClusterIP="None" through the spec.
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// ClusterIP: set to "None" for a headless Service. Default empty (api-server allocates).
	// +optional
	ClusterIP string `json:"clusterIP,omitempty"`

	// Ports is the list of Service ports. If empty, the operator emits a default
	// "gateway" port on 8443 (matches the StatefulSet's container port).
	// +listType=map
	// +listMapKey=name
	// +optional
	Ports []NamedServicePort `json:"ports,omitempty"`

	// Annotations are applied verbatim onto the Service (LoadBalancer hints, etc.).
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// LoadBalancerClass is propagated when Type=LoadBalancer.
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// ExternalTrafficPolicy is propagated when Type=LoadBalancer or NodePort.
	// +kubebuilder:validation:Enum=Cluster;Local
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
}

// NamedServicePort is a single Service port. The TargetPort is optional and
// defaults to Port when nil.
type NamedServicePort struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`

	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	// +kubebuilder:default=TCP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`

	// NodePort is honored only when the Service is NodePort or LoadBalancer.
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`
}

// IngressSpec controls optional Ingress creation.
type IngressSpec struct {
	// Enabled: when true, the operator creates an Ingress for the agent.
	// Default false.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Host is the primary hostname.
	// +optional
	Host string `json:"host,omitempty"`

	// ClassName is the IngressClass (`nginx`, `traefik`, ...).
	// +optional
	ClassName *string `json:"className,omitempty"`

	// TLS is the list of TLS settings.
	// +optional
	TLS []IngressTLSSpec `json:"tls,omitempty"`

	// Annotations are applied to the Ingress. The operator merges
	// provider-specific defaults (force-https, etc.) on top of these.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// PathType: default Prefix.
	// +kubebuilder:default=Prefix
	// +kubebuilder:validation:Enum=Exact;Prefix;ImplementationSpecific
	// +optional
	PathType networkingv1.PathType `json:"pathType,omitempty"`

	// Path: default "/".
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// ServicePortName: name of the Service port the Ingress should route to.
	// Default "gateway".
	// +kubebuilder:default="gateway"
	// +optional
	ServicePortName string `json:"servicePortName,omitempty"`
}

// IngressTLSSpec is a single TLS section on the Ingress.
type IngressTLSSpec struct {
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// +listType=set
	Hosts []string `json:"hosts,omitempty"`
}

// ObservabilitySpec controls metrics, scraping, alerting, logging.
type ObservabilitySpec struct {
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`
	// +optional
	ServiceMonitor ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
	// +optional
	PrometheusRule PrometheusRuleSpec `json:"prometheusRule,omitempty"`
	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`
}

// MetricsSpec controls the agent's Prometheus metrics endpoint.
type MetricsSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port for the /metrics endpoint.
	// +kubebuilder:default=9090
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// Secure: when true, /metrics requires bearer-token auth and uses HTTPS.
	// The ServiceMonitor scheme/scrape settings must agree (lesson #435/#440).
	// +kubebuilder:default=false
	// +optional
	Secure *bool `json:"secure,omitempty"`
}

// ServiceMonitorSpec controls Prometheus-Operator ServiceMonitor emission.
// When Enabled is true, the operator emits an unstructured ServiceMonitor; it
// does not require the Prometheus-Operator CRDs to be installed at compile time.
type ServiceMonitorSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Labels are extra labels applied onto the ServiceMonitor for Prometheus
	// label-selector matching (e.g. `release: kube-prometheus-stack`).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Interval: default "30s".
	// +kubebuilder:default="30s"
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	// +optional
	Interval string `json:"interval,omitempty"`

	// ScrapeTimeout: default "10s".
	// +kubebuilder:default="10s"
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	// +optional
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`
}

// PrometheusRuleSpec controls emission of a default PrometheusRule with
// hermes-agent alerts (HighRestartRate, MetricsDown, etc.).
type PrometheusRuleSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AdditionalRules is a list of user-supplied rules merged onto the operator
	// default ruleset.
	// +optional
	AdditionalRules []PrometheusRule `json:"additionalRules,omitempty"`
}

// PrometheusRule is a minimal copy of monitoringv1.Rule so we don't depend on
// the Prometheus-Operator Go types at compile time. The runtime emits
// unstructured objects.
type PrometheusRule struct {
	// +kubebuilder:validation:MinLength=1
	Alert string `json:"alert"`
	// +kubebuilder:validation:MinLength=1
	Expr string `json:"expr"`
	// +optional
	For string `json:"for,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// LogFormat is the agent's log output format.
// +kubebuilder:validation:Enum=text;json
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// LoggingSpec controls the agent's logger configuration via env vars.
type LoggingSpec struct {
	// +kubebuilder:default=text
	// +optional
	Format LogFormat `json:"format,omitempty"`

	// Level: Plan 3 wires HERMES_LOG_LEVEL on the agent container.
	// +kubebuilder:default=info
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error
	// +optional
	Level string `json:"level,omitempty"`
}

// AvailabilitySpec bundles PDB, HPA, and topology-spread.
type AvailabilitySpec struct {
	// +optional
	PodDisruptionBudget PDBSpec `json:"podDisruptionBudget,omitempty"`

	// +optional
	HorizontalPodAutoscaler HPASpec `json:"horizontalPodAutoscaler,omitempty"`

	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// PDBSpec controls PodDisruptionBudget emission.
type PDBSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinAvailable: optional, mutually exclusive with MaxUnavailable.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// MaxUnavailable: optional, mutually exclusive with MinAvailable.
	// Default 1 when neither is set and PDB is enabled.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// HPASpec controls HorizontalPodAutoscaler emission.
type HPASpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinReplicas: default 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas: default 5.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilization: default 80 (percent).
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`

	// TargetMemoryUtilization: optional, when set adds a memory metric.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemoryUtilization *int32 `json:"targetMemoryUtilization,omitempty"`

	// Behavior is forwarded onto HPA's autoscaling/v2 behavior field.
	// Plan 6 conformance suite asserts the field is exposed; v1 forwards it raw.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// ProbesSpec overrides the operator's built-in probes. Each field is a complete
// probe: set every value you want non-default because we apply it verbatim.
type ProbesSpec struct {
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
}

// SchedulingSpec targets the agent pod at specific nodes.
type SchedulingSpec struct {
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// InstanceSkill: Plan 3 fills the runtime semantics. The field exists here so
// SSA from HermesSelfConfig (Plan 4) can patch the slice with listMapKey=source.
type InstanceSkill struct {
	// Source is the uv/pip-compatible install source.
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`

	// Version optionally pins the install version. Mirrors SelfConfigSkill.Version
	// so HermesSelfConfig can carry the field through SSA without truncation.
	// +optional
	Version string `json:"version,omitempty"`
}

// SelfConfigureSpec is the allowlist policy for HermesSelfConfig mutations.
// Plan 4 wires the controller; the field exists here so Plan 4 doesn't need a
// CRD change. The validator rejects Enabled=true with ProtectedKeys empty.
type SelfConfigureSpec struct {
	// Enabled: explicit *bool so the defaulter can distinguish "user said false"
	// from "user did not set it" (Plan 4 relies on this).
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AllowedActions is the set of permitted action categories Plan 4 will
	// enforce: skills, config, envVars, workspaceFiles, profiles.
	// +listType=set
	// +optional
	AllowedActions []SelfConfigAction `json:"allowedActions,omitempty"`

	// ProtectedKeys is the list of glob expressions over JSON paths that may
	// not be mutated by HermesSelfConfig. Required (non-empty) when Enabled=true.
	// +listType=set
	// +optional
	ProtectedKeys []string `json:"protectedKeys,omitempty"`
}

// BackupSpec controls S3-compatible PVC snapshots for this instance.
type BackupSpec struct {
	// +optional
	S3 *BackupS3Spec `json:"s3,omitempty"`

	// +optional
	Schedule string `json:"schedule,omitempty"`

	// +kubebuilder:default=false
	// +optional
	OnDelete bool `json:"onDelete,omitempty"`

	// +kubebuilder:default=true
	// +optional
	PreUpdate *bool `json:"preUpdate,omitempty"`

	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10000
	// +optional
	HistoryLimit *int32 `json:"historyLimit,omitempty"`

	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +optional
	FailedHistoryLimit *int32 `json:"failedHistoryLimit,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`
}

// BackupS3Spec configures the S3-compatible remote target.
type BackupS3Spec struct {
	Bucket   string `json:"bucket"`
	Endpoint string `json:"endpoint"`
	// +optional
	Region string `json:"region,omitempty"`
	// +optional
	PathPrefix           string               `json:"pathPrefix,omitempty"`
	CredentialsSecretRef LocalObjectReference `json:"credentialsSecretRef"`
}

// LocalObjectReference is a same-namespace reference by name.
type LocalObjectReference struct {
	Name string `json:"name"`
}

// AutoUpdateSpec controls opt-in OCI-registry polling for newer agent images.
type AutoUpdateSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	Source AutoUpdateSourceSpec `json:"source,omitempty"`

	// +kubebuilder:default="1h"
	// +optional
	PollInterval string `json:"pollInterval,omitempty"`

	// +kubebuilder:default=true
	// +optional
	BackupBeforeUpdate *bool `json:"backupBeforeUpdate,omitempty"`

	// +optional
	Rollback AutoUpdateRollbackSpec `json:"rollback,omitempty"`
}

// AutoUpdateSourceSpec is the OCI registry source for the channel.
type AutoUpdateSourceSpec struct {
	// +optional
	Registry string `json:"registry,omitempty"`

	// +optional
	Channel string `json:"channel,omitempty"`
}

// AutoUpdateRollbackSpec configures the rollback path.
type AutoUpdateRollbackSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	ProbeFailureThreshold int32 `json:"probeFailureThreshold,omitempty"`
}

// MigrationSpec is a one-shot migration source (immutable once status.migration.completed is true).
type MigrationSpec struct {
	// +optional
	FromOpenClaw *MigrationFromOpenClawSpec `json:"fromOpenClaw,omitempty"`
}

// MigrationFromOpenClawSpec describes an OpenClaw source.
type MigrationFromOpenClawSpec struct {
	Source MigrationFromOpenClawSource `json:"source"`

	// +kubebuilder:default=copy
	// +kubebuilder:validation:Enum=copy;move
	// +optional
	Mode string `json:"mode,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`
}

// MigrationFromOpenClawSource is exactly-one-of (validated by webhook).
type MigrationFromOpenClawSource struct {
	// +optional
	OpenClawInstanceRef *NamespacedObjectReference `json:"openclawInstanceRef,omitempty"`

	// +optional
	BackupRef *MigrationBackupRef `json:"backupRef,omitempty"`
}

// NamespacedObjectReference is a name+namespace pointer.
type NamespacedObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// MigrationBackupRef points at an OpenClaw backup snapshot in S3.
type MigrationBackupRef struct {
	S3 MigrationBackupS3 `json:"s3"`
}

// MigrationBackupS3 mirrors BackupS3Spec but adds an explicit Key.
type MigrationBackupS3 struct {
	Bucket   string `json:"bucket"`
	Endpoint string `json:"endpoint"`
	// +optional
	Region               string               `json:"region,omitempty"`
	Key                  string               `json:"key"`
	CredentialsSecretRef LocalObjectReference `json:"credentialsSecretRef"`
}

// HermesInstanceStatus reflects the observed state of HermesInstance.
type HermesInstanceStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a short human-readable status (Pending | Ready | Degraded | Suspended).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions reflect subsystem readiness. Plan 2 emits:
	//   StorageReady, ConfigReady, SecretsReady, NetworkPolicyReady, RBACReady,
	//   ServiceReady, PDBReady, HPAReady, IngressReady, ServiceMonitorReady,
	//   PrometheusRuleReady, WebhookReady (manager-level), Ready (overall).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Replicas is the latest observed StatefulSet replica count.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the latest observed ready-replica count.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// RestoredFrom records the snapshot ID used in the last restore.
	// +optional
	RestoredFrom string `json:"restoredFrom,omitempty"`

	// Backup reflects the most recent backup state.
	// +optional
	Backup BackupStatus `json:"backup,omitempty"`

	// AutoUpdate reflects the current auto-update state.
	// +optional
	AutoUpdate AutoUpdateStatus `json:"autoUpdate,omitempty"`

	// Migration reflects the one-shot migration state.
	// +optional
	Migration MigrationStatus `json:"migration,omitempty"`
}

// BackupStatus reflects the most recent backup outcomes.
type BackupStatus struct {
	// +optional
	LastSuccessTime *metav1.Time `json:"lastSuccessTime,omitempty"`
	// +optional
	LastSuccessSnapshotID string `json:"lastSuccessSnapshotID,omitempty"`
	// +optional
	LastFailureTime *metav1.Time `json:"lastFailureTime,omitempty"`
	// +optional
	LastFailureReason string `json:"lastFailureReason,omitempty"`
	// +optional
	FinalBackupJobName string `json:"finalBackupJobName,omitempty"`
}

// AutoUpdateStatus reflects the current auto-update rollout state.
type AutoUpdateStatus struct {
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
	// +optional
	CurrentTag string `json:"currentTag,omitempty"`
	// +optional
	TargetTag string `json:"targetTag,omitempty"`
	// +optional
	LastSuccessTag string `json:"lastSuccessTag,omitempty"`
	// +optional
	LastFailedTag string `json:"lastFailedTag,omitempty"`
	// +optional
	PreUpdateSnapshot string `json:"preUpdateSnapshot,omitempty"`
	// +optional
	ProbeFailures int32 `json:"probeFailures,omitempty"`
	// +optional
	RolloutDeadline *metav1.Time `json:"rolloutDeadline,omitempty"`
}

// MigrationStatus reflects the one-shot migration outcome.
type MigrationStatus struct {
	// +optional
	Completed bool `json:"completed,omitempty"`
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// +optional
	SourceVersion string `json:"sourceVersion,omitempty"`
}

// Condition type constants. Centralised so Plan 4-6 and docs/conditions.md stay aligned.
const (
	ConditionTypeReady               = "Ready"
	ConditionTypeStorageReady        = "StorageReady"
	ConditionTypeConfigReady         = "ConfigReady"
	ConditionTypeSecretsReady        = "SecretsReady"
	ConditionTypeNetworkPolicyReady  = "NetworkPolicyReady"
	ConditionTypeRBACReady           = "RBACReady"
	ConditionTypeServiceReady        = "ServiceReady"
	ConditionTypePDBReady            = "PDBReady"
	ConditionTypeHPAReady            = "HPAReady"
	ConditionTypeIngressReady        = "IngressReady"
	ConditionTypeServiceMonitorReady = "ServiceMonitorReady"
	ConditionTypePrometheusRuleReady = "PrometheusRuleReady"

	ConditionBackupReady          = "BackupReady"
	ConditionRestoreApplied       = "RestoreApplied"
	ConditionAutoUpdated          = "AutoUpdated"
	ConditionAutoUpdateRolledBack = "AutoUpdateRolledBack"
	ConditionMigrationCompleted   = "MigrationCompleted"

	FinalizerBackupOnDelete    = "hermes.agent/backup-on-delete"
	AnnotationAutoUpdateTarget = "hermes.agent/autoupdate-target"
	AnnotationSkipFinalBackup  = "hermes.agent/skip-final-backup"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hi;hermes,categories=hermes;agents
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HermesInstance is the Schema for the hermesinstances API
type HermesInstance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HermesInstance
	// +required
	Spec HermesInstanceSpec `json:"spec"`

	// status defines the observed state of HermesInstance
	// +optional
	Status HermesInstanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HermesInstanceList contains a list of HermesInstance
type HermesInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HermesInstance `json:"items"`
}

// RuntimeSpec controls Python/uv runtime concerns for the agent container.
type RuntimeSpec struct {
	// Python is informational only: the agent image's Python version is fixed
	// at build time. Setting this does NOT pull a different interpreter; it
	// exists so downstream tooling can assert the runtime it expects.
	// +kubebuilder:default="3.11"
	// +optional
	Python string `json:"python,omitempty"`

	// UV controls the initial `uv sync` against the lockfile bundled in the
	// agent image. Enabled by default.
	// +optional
	UV UVSpec `json:"uv,omitempty"`

	// FFmpeg toggles the FFmpeg dependency check. The agent image always ships
	// FFmpeg; disabling here only skips the readiness assertion.
	// +optional
	FFmpeg FFmpegSpec `json:"ffmpeg,omitempty"`

	// Ripgrep toggles the ripgrep dependency check. See FFmpeg.
	// +optional
	Ripgrep RipgrepSpec `json:"ripgrep,omitempty"`

	// ExtraAptPackages adds additional Debian packages installed by a
	// root-privileged init container BEFORE the main agent container starts.
	// Use sparingly: the init container runs as root and breaks the otherwise
	// hardened security posture for one container only.
	// +listType=atomic
	// +optional
	ExtraAptPackages []string `json:"extraAptPackages,omitempty"`

	// ExtraPipPackages adds additional Python packages installed via
	// `uv pip install` into a persistent venv on the data PVC.
	// +listType=atomic
	// +optional
	ExtraPipPackages []string `json:"extraPipPackages,omitempty"`
}

// UVSpec controls the `uv sync` init container.
type UVSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ExtraIndexURL is appended to uv's index list. Useful for private PyPI
	// mirrors. Empty by default.
	// +optional
	ExtraIndexURL string `json:"extraIndexURL,omitempty"`

	// CacheVolume controls the volume mounted at /home/hermes/.cache/uv.
	// Defaults to an emptyDir with a 1Gi sizeLimit: fast and ephemeral.
	// +optional
	CacheVolume UVCacheVolumeSpec `json:"cacheVolume,omitempty"`
}

// UVCacheVolumeSpec mirrors a stripped-down VolumeSource union. Exactly one of
// EmptyDir or PersistentVolumeClaim may be set; the defaulter fills EmptyDir
// when both are nil.
type UVCacheVolumeSpec struct {
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

// FFmpegSpec controls the FFmpeg dependency check.
type FFmpegSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// RipgrepSpec controls the ripgrep dependency check.
type RipgrepSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// GatewaysSpec is the union of all supported messaging-platform bindings.
type GatewaysSpec struct {
	// +optional
	Telegram TelegramGatewaySpec `json:"telegram,omitempty"`
	// +optional
	Discord DiscordGatewaySpec `json:"discord,omitempty"`
	// +optional
	Slack SlackGatewaySpec `json:"slack,omitempty"`
	// +optional
	WhatsApp WhatsAppGatewaySpec `json:"whatsapp,omitempty"`
	// +optional
	Signal SignalGatewaySpec `json:"signal,omitempty"`
}

// TelegramGatewaySpec binds the agent to a Telegram Bot API token.
type TelegramGatewaySpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// BotTokenSecretRef points at the Secret holding the Bot API token.
	// Required when Enabled.
	// +optional
	BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

	// AllowedUserIDs is an optional allow-list of Telegram user IDs.
	// +listType=atomic
	// +optional
	AllowedUserIDs []int64 `json:"allowedUserIDs,omitempty"`

	// WebhookURL is the public HTTPS URL to register with Telegram. When empty
	// the agent runs in long-poll mode.
	// +optional
	WebhookURL string `json:"webhookURL,omitempty"`
}

// DiscordGatewaySpec binds the agent to a Discord bot application.
type DiscordGatewaySpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

	// ApplicationID is the Discord application's snowflake.
	// +optional
	ApplicationID string `json:"applicationID,omitempty"`

	// GuildIDs scopes slash-command registration to specific guilds.
	// +listType=atomic
	// +optional
	GuildIDs []string `json:"guildIDs,omitempty"`
}

// SlackGatewaySpec binds the agent to a Slack workspace via the bolt SDK.
type SlackGatewaySpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	BotTokenSecretRef *corev1.SecretKeySelector `json:"botTokenSecretRef,omitempty"`

	// +optional
	AppTokenSecretRef *corev1.SecretKeySelector `json:"appTokenSecretRef,omitempty"`

	// +optional
	SigningSecretRef *corev1.SecretKeySelector `json:"signingSecretRef,omitempty"`
}

// WhatsAppGatewaySpec binds the agent to a WhatsApp provider (Twilio,
// Meta Cloud API, etc.).
type WhatsAppGatewaySpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	ProviderSecretRef *corev1.SecretKeySelector `json:"providerSecretRef,omitempty"`
}

// SignalGatewaySpec binds the agent to signal-cli-rest-api running as a sidecar
// or external service.
type SignalGatewaySpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	PhoneNumberSecretRef *corev1.SecretKeySelector `json:"phoneNumberSecretRef,omitempty"`

	// +optional
	AuthTokenSecretRef *corev1.SecretKeySelector `json:"authTokenSecretRef,omitempty"`
}

// ProfileStoreSpec is the union of supported profile-store backends. Only
// `honcho` is supported in v1.
type ProfileStoreSpec struct {
	// +optional
	Honcho HonchoSpec `json:"honcho,omitempty"`
}

// HonchoSpec controls the Honcho companion Deployment.
type HonchoSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	Image HonchoImageSpec `json:"image,omitempty"`

	// +optional
	Persistence HonchoPersistenceSpec `json:"persistence,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// APIKeySecretRef points at the Secret holding the Honcho API key.
	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// HonchoImageSpec selects the Honcho image.
type HonchoImageSpec struct {
	// +kubebuilder:default="ghcr.io/plastic-labs/honcho"
	// +optional
	Repository string `json:"repository,omitempty"`

	// +kubebuilder:default="0.1.0"
	// +optional
	Tag string `json:"tag,omitempty"`

	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +optional
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// HonchoPersistenceSpec controls the Honcho-side PVC.
type HonchoPersistenceSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +kubebuilder:default="5Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HermesInstance{}, &HermesInstanceList{})
}
