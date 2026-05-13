package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildPDB_DefaultMaxUnavailable(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	pdb := BuildPDB(inst)
	assert.Equal(t, "demo", pdb.Name)
	assert.Equal(t, "agents", pdb.Namespace)
	assert.NotNil(t, pdb.Spec.MaxUnavailable)
	assert.Equal(t, intstr.FromInt(1), *pdb.Spec.MaxUnavailable)
	assert.Equal(t, "demo", pdb.Spec.Selector.MatchLabels["app.kubernetes.io/instance"])
}

func TestBuildPDB_HonorsMinAvailable(t *testing.T) {
	t.Parallel()
	pdbMin := intstr.FromString("50%")
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &pdbMin},
			},
		},
	}
	pdb := BuildPDB(inst)
	assert.NotNil(t, pdb.Spec.MinAvailable)
	assert.Equal(t, "50%", pdb.Spec.MinAvailable.StrVal)
	assert.Nil(t, pdb.Spec.MaxUnavailable, "MinAvailable and MaxUnavailable are mutually exclusive")
}

func TestPDBName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", PDBName(inst))
}
