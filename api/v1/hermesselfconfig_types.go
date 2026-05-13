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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SelfConfigAction names a category of mutation. Used by
// HermesInstance.spec.selfConfigure.allowedActions to gate what the agent
// may request via HermesSelfConfig.
// +kubebuilder:validation:Enum=skills;config;envVars;workspaceFiles;profiles
type SelfConfigAction string

const (
	ActionSkills         SelfConfigAction = "skills"
	ActionConfig         SelfConfigAction = "config"
	ActionEnvVars        SelfConfigAction = "envVars"
	ActionWorkspaceFiles SelfConfigAction = "workspaceFiles"
	ActionProfiles       SelfConfigAction = "profiles"
)

// HermesSelfConfigSpec is an agent-driven, audited request to mutate the
// parent HermesInstance. The operator validates against the parent's
// .spec.selfConfigure policy, then applies via Server-Side Apply with
// field manager "hermes.agent/selfconfig".
type HermesSelfConfigSpec struct {
	// InstanceRef is the name of the parent HermesInstance in the same namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	InstanceRef string `json:"instanceRef"`

	// AddSkills appends skills to the parent's .spec.skills.
	// +listType=map
	// +listMapKey=source
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddSkills []SelfConfigSkill `json:"addSkills,omitempty"`

	// PatchConfig is a JSON merge patch (RFC 7396) applied to the agent's
	// runtime config at ~/.hermes/config.yaml.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PatchConfig *apiextensionsv1.JSON `json:"patchConfig,omitempty"`

	// AddEnvVars appends environment variables to the parent's .spec.env.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddEnvVars []SelfConfigEnvVar `json:"addEnvVars,omitempty"`

	// AddWorkspaceFiles writes files into the workspace ConfigMap.
	// +listType=map
	// +listMapKey=path
	// +kubebuilder:validation:MaxItems=50
	// +optional
	AddWorkspaceFiles []SelfConfigWorkspaceFile `json:"addWorkspaceFiles,omitempty"`

	// AddProfileSnapshot writes an opaque Honcho profile snapshot via a one-shot Job.
	// +optional
	AddProfileSnapshot *SelfConfigProfileSnapshot `json:"addProfileSnapshot,omitempty"`
}

// SelfConfigSkill names one skill to install.
type SelfConfigSkill struct {
	// Source is a uv-compatible package specifier. Required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Source string `json:"source"`

	// Version optionally pins a version.
	// +optional
	Version string `json:"version,omitempty"`
}

// SelfConfigEnvVar is an environment variable entry.
type SelfConfigEnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`

	// Value is the literal value. Mutually exclusive with ValueFrom.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom selects a value from a Secret or ConfigMap key.
	// +optional
	ValueFrom *SelfConfigEnvVarSource `json:"valueFrom,omitempty"`
}

// SelfConfigEnvVarSource selects a Secret or ConfigMap key. Exactly one ref must be set.
type SelfConfigEnvVarSource struct {
	// +optional
	SecretKeyRef *SelfConfigKeySelector `json:"secretKeyRef,omitempty"`
	// +optional
	ConfigMapKeyRef *SelfConfigKeySelector `json:"configMapKeyRef,omitempty"`
}

// SelfConfigKeySelector selects a key from a Secret or ConfigMap.
type SelfConfigKeySelector struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// SelfConfigWorkspaceFile is a single file to materialise into the workspace.
type SelfConfigWorkspaceFile struct {
	// Path is the relative path under ~/.hermes/workspace/.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._/-]+$`
	Path string `json:"path"`

	// Content is the literal file body.
	// +optional
	Content string `json:"content,omitempty"`

	// ContentFrom reads the file body from a Secret key.
	// +optional
	ContentFrom *SelfConfigKeySelector `json:"contentFrom,omitempty"`
}

// SelfConfigProfileSnapshot writes one Honcho profile snapshot via a Job.
type SelfConfigProfileSnapshot struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProfileID string `json:"profileID"`

	// Data is the opaque snapshot payload.
	// +kubebuilder:validation:MinLength=1
	Data string `json:"data"`
}

// SelfConfigPhase is a short human-readable status.
// +kubebuilder:validation:Enum=Pending;Applied;Denied
type SelfConfigPhase string

const (
	SelfConfigPhasePending SelfConfigPhase = "Pending"
	SelfConfigPhaseApplied SelfConfigPhase = "Applied"
	SelfConfigPhaseDenied  SelfConfigPhase = "Denied"
)

// SelfConfigConditionType enumerates the conditions a HermesSelfConfig may carry.
type SelfConfigConditionType string

const (
	SelfConfigConditionApplied SelfConfigConditionType = "Applied"
	SelfConfigConditionDenied  SelfConfigConditionType = "Denied"
	SelfConfigConditionPending SelfConfigConditionType = "Pending"
)

// HermesSelfConfigStatus reflects the observed state of a HermesSelfConfig.
type HermesSelfConfigStatus struct {
	// ObservedGeneration is the spec generation the controller last processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase summarises the current state. One of Pending, Applied, Denied.
	// +optional
	Phase SelfConfigPhase `json:"phase,omitempty"`

	// AppliedAt is the timestamp of the most recent successful SSA write.
	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`

	// DenyReason is populated when Phase=Denied.
	// +optional
	DenyReason string `json:"denyReason,omitempty"`

	// AppliedFields lists the dotted paths SSA touched on the parent.
	// +listType=set
	// +optional
	AppliedFields []string `json:"appliedFields,omitempty"`

	// Conditions surface fine-grained state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hsc,categories=hermes;agents
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instanceRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="DenyReason",type=string,JSONPath=`.status.denyReason`,priority=1

// HermesSelfConfig is the Schema for the hermesselfconfigs API
type HermesSelfConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HermesSelfConfig
	// +required
	Spec HermesSelfConfigSpec `json:"spec"`

	// status defines the observed state of HermesSelfConfig
	// +optional
	Status HermesSelfConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HermesSelfConfigList contains a list of HermesSelfConfig
type HermesSelfConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HermesSelfConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HermesSelfConfig{}, &HermesSelfConfigList{})
}
