package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildHPA_DefaultsScaleTargetStatefulSet(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{Enabled: Ptr(true)},
			},
		},
	}
	hpa := BuildHPA(inst)
	assert.Equal(t, "demo", hpa.Name)
	assert.Equal(t, "agents", hpa.Namespace)
	assert.Equal(t, "StatefulSet", hpa.Spec.ScaleTargetRef.Kind)
	assert.Equal(t, "apps/v1", hpa.Spec.ScaleTargetRef.APIVersion)
	assert.Equal(t, "demo", hpa.Spec.ScaleTargetRef.Name)
	assert.NotNil(t, hpa.Spec.MinReplicas)
	assert.Equal(t, int32(1), *hpa.Spec.MinReplicas)
	assert.Equal(t, int32(5), hpa.Spec.MaxReplicas)
	assert.NotEmpty(t, hpa.Spec.Metrics)
}

func TestBuildHPA_CustomCPUTarget(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled:              Ptr(true),
					MinReplicas:          Ptr(int32(2)),
					MaxReplicas:          Ptr(int32(10)),
					TargetCPUUtilization: Ptr(int32(70)),
				},
			},
		},
	}
	hpa := BuildHPA(inst)
	assert.Equal(t, int32(2), *hpa.Spec.MinReplicas)
	assert.Equal(t, int32(10), hpa.Spec.MaxReplicas)
	require := false
	for _, m := range hpa.Spec.Metrics {
		if m.Type == autoscalingv2.ResourceMetricSourceType && m.Resource.Name == corev1.ResourceCPU {
			require = true
			assert.Equal(t, int32(70), *m.Resource.Target.AverageUtilization)
		}
	}
	assert.True(t, require)
}

func TestBuildHPA_MemoryMetric(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled:                 Ptr(true),
					TargetMemoryUtilization: Ptr(int32(85)),
				},
			},
		},
	}
	hpa := BuildHPA(inst)
	var sawMemory bool
	for _, m := range hpa.Spec.Metrics {
		if m.Type == autoscalingv2.ResourceMetricSourceType && m.Resource.Name == corev1.ResourceMemory {
			sawMemory = true
			assert.Equal(t, int32(85), *m.Resource.Target.AverageUtilization)
		}
	}
	assert.True(t, sawMemory)
}

func TestIsHPAEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.False(t, IsHPAEnabled(inst))
	inst.Spec.Availability.HorizontalPodAutoscaler.Enabled = Ptr(false)
	assert.False(t, IsHPAEnabled(inst))
	inst.Spec.Availability.HorizontalPodAutoscaler.Enabled = Ptr(true)
	assert.True(t, IsHPAEnabled(inst))
}

func TestHPAName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", HPAName(inst))
}
