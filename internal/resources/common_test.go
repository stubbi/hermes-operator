package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestPtr(t *testing.T) {
	s := Ptr("x")
	assert.NotNil(t, s)
	assert.Equal(t, "x", *s)
}

func TestLabelsForInstance(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	got := LabelsForInstance(inst)
	assert.Equal(t, "hermes-agent", got["app.kubernetes.io/name"])
	assert.Equal(t, "demo", got["app.kubernetes.io/instance"])
	assert.Equal(t, "hermes-operator", got["app.kubernetes.io/managed-by"])
	assert.Equal(t, "hermes.agent", got["app.kubernetes.io/part-of"])
}

func TestMergePreservingForeignAnnotations(t *testing.T) {
	existing := map[string]string{
		"hermes.agent/foo":    "old",
		"third-party/keep-me": "preserve",
	}
	desired := map[string]string{
		"hermes.agent/foo": "new",
		"hermes.agent/bar": "added",
	}
	got := MergePreservingForeign(existing, desired, "hermes.agent/")
	assert.Equal(t, "new", got["hermes.agent/foo"], "operator key overwritten")
	assert.Equal(t, "added", got["hermes.agent/bar"], "new operator key added")
	assert.Equal(t, "preserve", got["third-party/keep-me"], "foreign key preserved")
}

func TestPortConstants(t *testing.T) {
	t.Parallel()
	// Constants must be stable: Plan 3-6 reference these by name.
	assert.Equal(t, int32(8443), GatewayPort)
	assert.Equal(t, int32(9090), DefaultMetricsPort)
	assert.Equal(t, "gateway", GatewayPortName)
	assert.Equal(t, "metrics", MetricsPortName)
}

func TestSelectorLabels(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	got := SelectorLabels(inst)
	// Selector labels are the immutable subset of LabelsForInstance.
	assert.Equal(t, "hermes-agent", got["app.kubernetes.io/name"])
	assert.Equal(t, "demo", got["app.kubernetes.io/instance"])
	// Selector labels MUST NOT include "managed-by" because that field is
	// allowed to evolve across operator versions.
	_, exists := got["app.kubernetes.io/managed-by"]
	assert.False(t, exists)
}

func TestServiceAccountName_Override(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				RBAC: hermesv1.RBACSpec{ServiceAccountName: "byo-sa"},
			},
		},
	}
	assert.Equal(t, "byo-sa", ServiceAccountNameFor(inst))

	inst.Spec.Security.RBAC.ServiceAccountName = ""
	assert.Equal(t, "demo", ServiceAccountNameFor(inst))
}

func TestBoolValue(t *testing.T) {
	t.Parallel()
	assert.True(t, BoolValue(Ptr(true)))
	assert.False(t, BoolValue(Ptr(false)))
	assert.False(t, BoolValue(nil))
	assert.True(t, BoolValueOrDefault(nil, true))
	assert.False(t, BoolValueOrDefault(Ptr(false), true))
}
