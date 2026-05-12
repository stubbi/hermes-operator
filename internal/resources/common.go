package resources

import (
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// Ptr returns a pointer to v. Use only for short-lived literals.
func Ptr[T any](v T) *T { return &v }

// LabelsForInstance returns the standard recommended labels for resources
// owned by a HermesInstance. Plans 2+ may add more.
func LabelsForInstance(inst *hermesv1.HermesInstance) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "hermes-agent",
		"app.kubernetes.io/instance":   inst.Name,
		"app.kubernetes.io/managed-by": "hermes-operator",
		"app.kubernetes.io/part-of":    "hermes.agent",
	}
}

// MergePreservingForeign merges desired into existing, overwriting keys that
// start with the operator prefix and preserving all other keys.
// Lesson from openclaw-operator #446/#447.
func MergePreservingForeign(existing, desired map[string]string, operatorPrefix string) map[string]string {
	out := make(map[string]string, len(existing)+len(desired))
	for k, v := range existing {
		if strings.HasPrefix(k, operatorPrefix) {
			continue
		}
		out[k] = v
	}
	for k, v := range desired {
		out[k] = v
	}
	return out
}

// Named ports used across builders. Stable across versions.
const (
	GatewayPort        int32 = 8443
	DefaultMetricsPort int32 = 9090
	GatewayPortName          = "gateway"
	MetricsPortName          = "metrics"
)

// SelectorLabels returns the immutable subset of LabelsForInstance suitable for
// Selector fields on Service/Deployment/StatefulSet. Selectors are immutable
// in k8s; the operator-managed-by label may evolve across versions, so we
// exclude it from selectors.
func SelectorLabels(inst *hermesv1.HermesInstance) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "hermes-agent",
		"app.kubernetes.io/instance": inst.Name,
	}
}

// ServiceAccountNameFor returns the ServiceAccount the agent pod should use:
// the spec.security.rbac.serviceAccountName override when set, else the
// operator-created SA which has the same name as the instance.
func ServiceAccountNameFor(inst *hermesv1.HermesInstance) string {
	if inst.Spec.Security.RBAC.ServiceAccountName != "" {
		return inst.Spec.Security.RBAC.ServiceAccountName
	}
	return inst.Name
}

// BoolValue dereferences a *bool, returning false on nil.
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// BoolValueOrDefault dereferences a *bool, returning def on nil.
func BoolValueOrDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
