package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
