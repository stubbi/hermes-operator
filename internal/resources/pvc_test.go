package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPVC_DefaultsAndLabels(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{
					Enabled: Ptr(true),
					Size:    "5Gi",
				},
			},
		},
	}

	pvc := BuildPVC(inst)
	assert.Equal(t, "demo-data", pvc.Name)
	assert.Equal(t, "agents", pvc.Namespace)
	assert.Equal(t, "hermes-agent", pvc.Labels["app.kubernetes.io/name"])
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pvc.Spec.AccessModes)
	assert.Equal(t, resource.MustParse("5Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
	assert.Nil(t, pvc.Spec.StorageClassName, "no storage class when unset")
}

func TestBuildPVC_StorageClass(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{
					Enabled:          Ptr(true),
					Size:             "1Gi",
					StorageClassName: Ptr("gp3"),
				},
			},
		},
	}
	pvc := BuildPVC(inst)
	assert.NotNil(t, pvc.Spec.StorageClassName)
	assert.Equal(t, "gp3", *pvc.Spec.StorageClassName)
}
