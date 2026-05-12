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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	// ConfigMergeModeReplace — Raw replaces ConfigMapRef entirely when both are set.
	// This is the default to avoid surprising merges.
	ConfigMergeModeReplace ConfigMergeMode = "replace"
	// ConfigMergeModeMerge — YAML deep-merge Raw onto ConfigMapRef. Raw wins on conflict.
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
// single-level ConfigMap data map can express them — Plan 3's runtime-init
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
	// Enabled — default false. Plan 3 wires the actual init-container.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// ResourcesSpec sets CPU/memory requests + limits on the agent container.
// Defaults intentionally omitted — the defaulting webhook fills from
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

// SecuritySpec — populated in Task 6.
type SecuritySpec struct{}

// NetworkingSpec — populated in Task 7.
type NetworkingSpec struct{}

// ObservabilitySpec — populated in Task 8.
type ObservabilitySpec struct{}

// AvailabilitySpec — populated in Task 9.
type AvailabilitySpec struct{}

// ProbesSpec — populated in Task 9.
type ProbesSpec struct{}

// SchedulingSpec — populated in Task 9.
type SchedulingSpec struct{}

// InstanceSkill — Plan 3 fills the runtime semantics. The field exists here so
// SSA from HermesSelfConfig (Plan 4) can patch the slice with listMapKey=source.
type InstanceSkill struct {
	// Source is the uv/pip-compatible install source.
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`
}

// SelfConfigureSpec — populated in Task 9.
type SelfConfigureSpec struct{}

// HermesInstanceStatus reflects the observed state of HermesInstance.
type HermesInstanceStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a short human-readable status (Pending|Ready|Degraded).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the instance's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

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

func init() {
	SchemeBuilder.Register(&HermesInstance{}, &HermesInstanceList{})
}
