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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HermesClusterDefaultsSpec is the cluster-wide default set applied by the
// defaulting webhook when a HermesInstance leaves a field nil. ClusterDefaults
// only fills nil fields; an explicit value on the instance always wins.
type HermesClusterDefaultsSpec struct {
	// Image defaults the instance's spec.image.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Registry defaults image-pull plumbing.
	// +optional
	Registry RegistryDefaults `json:"registry,omitempty"`

	// Storage defaults the instance's spec.storage.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Security defaults SA annotations + NetworkPolicy on/off + container-level
	// defaults (read-only rootfs etc. are operator-baked, not defaultable).
	// +optional
	Security SecurityDefaults `json:"security,omitempty"`

	// Observability defaults metrics / ServiceMonitor / PrometheusRule.
	// +optional
	Observability ObservabilityDefaults `json:"observability,omitempty"`

	// Networking defaults Service kind + NetworkPolicy enablement.
	// +optional
	Networking NetworkingDefaults `json:"networking,omitempty"`

	// Resources defaults requests + limits when the instance leaves them nil.
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`
}

// HermesClusterDefaultsStatus reflects observed singleton state.
type HermesClusterDefaultsStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions track Ready ("singleton-name OK and reachable").
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=hcd,categories=hermes;agents
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HermesClusterDefaults is the Schema for the hermesclusterdefaults API
type HermesClusterDefaults struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HermesClusterDefaults
	// +required
	Spec HermesClusterDefaultsSpec `json:"spec"`

	// status defines the observed state of HermesClusterDefaults
	// +optional
	Status HermesClusterDefaultsStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HermesClusterDefaultsList contains a list of HermesClusterDefaults
type HermesClusterDefaultsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HermesClusterDefaults `json:"items"`
}

// RegistryDefaults groups image-pull secret hints.
type RegistryDefaults struct {
	// PullSecretName, if non-empty, is added to every instance's
	// pod.spec.imagePullSecrets when the instance doesn't override.
	// +optional
	PullSecretName string `json:"pullSecretName,omitempty"`
}

// SecurityDefaults mirrors the defaultable subset of SecuritySpec.
type SecurityDefaults struct {
	// +optional
	ServiceAccount ServiceAccountDefaults `json:"serviceAccount,omitempty"`

	// +optional
	NetworkPolicy NetworkPolicyDefaults `json:"networkPolicy,omitempty"`

	// +optional
	CABundle CABundleSpec `json:"caBundle,omitempty"`
}

// ServiceAccountDefaults defaults the per-instance SA annotations (IRSA / WI).
type ServiceAccountDefaults struct {
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NetworkPolicyDefaults defaults whether per-instance NetworkPolicies are created.
type NetworkPolicyDefaults struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	AllowDNS *bool `json:"allowDNS,omitempty"`
}

// NetworkingDefaults mirrors the defaultable subset of NetworkingSpec.
type NetworkingDefaults struct {
	// +optional
	Service ServiceDefaults `json:"service,omitempty"`

	// +optional
	NetworkPolicy NetworkPolicyDefaults `json:"networkPolicy,omitempty"`
}

// ServiceDefaults defaults the Service kind cluster-wide.
type ServiceDefaults struct {
	// +optional
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	Type corev1.ServiceType `json:"type,omitempty"`
}

// ObservabilityDefaults mirrors the defaultable subset of ObservabilitySpec.
type ObservabilityDefaults struct {
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`

	// +optional
	ServiceMonitor ServiceMonitorSpec `json:"serviceMonitor,omitempty"`

	// +optional
	PrometheusRule PrometheusRuleSpec `json:"prometheusRule,omitempty"`

	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HermesClusterDefaults{}, &HermesClusterDefaultsList{})
}
