package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildGatewayTokenSecret_NameAndLabels(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	s := BuildGatewayTokenSecret(inst)
	assert.Equal(t, "demo-gateway-tokens", s.Name)
	assert.Equal(t, "agents", s.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, s.Type)
	assert.Equal(t, "hermes-agent", s.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "true", s.Annotations["hermes.agent/placeholder"])
}

func TestGatewayTokenSecretName_Determ(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo-gateway-tokens", GatewayTokenSecretName(inst))
}
