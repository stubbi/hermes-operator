package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require := hermesv1.AddToScheme(scheme)
	assert.NoError(t, require)
	assert.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func TestDefaulter_FillsNilFromClusterDefaults(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	hcd := &hermesv1.HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: hermesv1.HermesClusterDefaultsSpec{
			Image: hermesv1.ImageSpec{
				Repository: "ghcr.io/stubbi/hermes-agent",
				Tag:        "1.4.2",
			},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Size: "10Gi"},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hcd).Build()
	d := &HermesInstanceDefaulter{Client: c}

	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	assert.NoError(t, d.Default(context.Background(), inst))

	assert.Equal(t, "ghcr.io/stubbi/hermes-agent", inst.Spec.Image.Repository)
	assert.Equal(t, "1.4.2", inst.Spec.Image.Tag)
	assert.Equal(t, "10Gi", inst.Spec.Storage.Persistence.Size)
}

func TestDefaulter_ExplicitInstanceValuesAlwaysWin(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	hcd := &hermesv1.HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: hermesv1.HermesClusterDefaultsSpec{
			Image: hermesv1.ImageSpec{Repository: "default-repo", Tag: "default-tag"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hcd).Build()
	d := &HermesInstanceDefaulter{Client: c}

	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "explicit-repo", Tag: "explicit-tag"},
		},
	}
	assert.NoError(t, d.Default(context.Background(), inst))
	assert.Equal(t, "explicit-repo", inst.Spec.Image.Repository)
	assert.Equal(t, "explicit-tag", inst.Spec.Image.Tag)
}

func TestDefaulter_NoClusterDefaultsIsNotAnError(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &HermesInstanceDefaulter{Client: c}
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	err := d.Default(context.Background(), inst)
	assert.NoError(t, err, "missing HermesClusterDefaults is allowed")
	_ = apierrors.IsNotFound(nil)
}
