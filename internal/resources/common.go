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
