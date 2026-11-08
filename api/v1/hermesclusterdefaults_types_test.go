package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHermesClusterDefaults_Shape(t *testing.T) {
	t.Parallel()
	hcd := &HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: HermesClusterDefaultsSpec{
			Image: ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.4.2"},
			Registry: RegistryDefaults{
				PullSecretName: "ghcr-pull",
			},
			Storage: StorageSpec{
				Persistence: PersistenceSpec{Size: "10Gi", StorageClassName: Ptr("gp3")},
			},
			Security: SecurityDefaults{
				ServiceAccount: ServiceAccountDefaults{
					Annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:..."},
				},
			},
			Networking: NetworkingDefaults{
				NetworkPolicy: NetworkPolicyDefaults{Enabled: Ptr(true)},
			},
			Observability: ObservabilityDefaults{
				ServiceMonitor: ServiceMonitorSpec{Enabled: Ptr(true)},
			},
		},
	}
	assert.Equal(t, "cluster", hcd.Name)
	assert.Equal(t, "ghcr.io/stubbi/hermes-agent", hcd.Spec.Image.Repository)
	assert.Equal(t, "ghcr-pull", hcd.Spec.Registry.PullSecretName)
	assert.NotNil(t, hcd.Spec.Storage.Persistence.StorageClassName)

	// Sanity: a non-cluster name should still parse: the *webhook* rejects it,
	// not the type system.
	other := &HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "not-cluster"}}
	_ = corev1.ObjectReference{Name: other.Name}
}
