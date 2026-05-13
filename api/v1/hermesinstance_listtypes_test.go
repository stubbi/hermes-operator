package v1

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCRDListTypesForSSA asserts that the generated CRD OpenAPI schema carries
// the x-kubernetes-list-map-keys markers required for Server-Side Apply to
// merge the Env and Skills slices by key instead of replacing them atomically.
// If these regress, GitOps coexistence with HermesSelfConfig breaks silently.
func TestCRDListTypesForSSA(t *testing.T) {
	body, err := os.ReadFile("../../config/crd/bases/hermes.agent_hermesinstances.yaml")
	require.NoError(t, err, "CRD YAML missing: run `make manifests`")
	s := string(body)

	require.True(t, strings.Contains(s, "x-kubernetes-list-map-keys:\n                - name"),
		"HermesInstance.spec.env missing list-map-key=name in CRD YAML")
	require.True(t, strings.Contains(s, "x-kubernetes-list-map-keys:\n                - source"),
		"HermesInstance.spec.skills missing list-map-key=source in CRD YAML")
}
