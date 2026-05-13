package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildService_DefaultClusterIPWithGatewayPort(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	svc := BuildService(inst)
	assert.Equal(t, "demo", svc.Name)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(t, corev1.ServiceAffinityNone, svc.Spec.SessionAffinity)
	assert.Equal(t, "demo", svc.Spec.Selector["app.kubernetes.io/instance"])
	require := false
	for _, p := range svc.Spec.Ports {
		if p.Name == "gateway" && p.Port == 8443 {
			require = true
		}
	}
	assert.True(t, require, "default gateway port on 8443")
}

func TestBuildService_Headless(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: corev1.ClusterIPNone},
			},
		},
	}
	svc := BuildService(inst)
	assert.Equal(t, corev1.ClusterIPNone, svc.Spec.ClusterIP)
}

func TestBuildService_LoadBalancerAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Type:                  corev1.ServiceTypeLoadBalancer,
					Annotations:           map[string]string{"foo": "bar"},
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
				},
			},
		},
	}
	svc := BuildService(inst)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
	assert.Equal(t, "bar", svc.Annotations["foo"])
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, svc.Spec.ExternalTrafficPolicy)
}

func TestBuildService_CustomPorts(t *testing.T) {
	t.Parallel()
	tp := int32(8443)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Ports: []hermesv1.NamedServicePort{
						{Name: "gateway", Port: 443, TargetPort: &tp, Protocol: corev1.ProtocolTCP},
					},
				},
			},
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(false)},
			},
		},
	}
	svc := BuildService(inst)
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(443), svc.Spec.Ports[0].Port)
}

func TestBuildService_AddsMetricsPortWhenEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true), Port: 9090},
			},
		},
	}
	svc := BuildService(inst)
	var sawMetrics bool
	for _, p := range svc.Spec.Ports {
		if p.Name == "metrics" && p.Port == 9090 {
			sawMetrics = true
		}
	}
	assert.True(t, sawMetrics, "metrics port emitted when Metrics.Enabled")
}
