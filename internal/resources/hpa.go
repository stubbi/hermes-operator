package resources

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HPAName returns the deterministic HPA name.
func HPAName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// IsHPAEnabled returns true when spec.availability.horizontalPodAutoscaler.enabled is true.
func IsHPAEnabled(inst *hermesv1.HermesInstance) bool {
	return BoolValue(inst.Spec.Availability.HorizontalPodAutoscaler.Enabled)
}

// BuildHPA constructs the desired HorizontalPodAutoscaler. Scale target is the
// StatefulSet built by BuildStatefulSet (same name).
func BuildHPA(inst *hermesv1.HermesInstance) *autoscalingv2.HorizontalPodAutoscaler {
	hs := inst.Spec.Availability.HorizontalPodAutoscaler

	minReplicas := int32(1)
	if hs.MinReplicas != nil {
		minReplicas = *hs.MinReplicas
	}
	maxReplicas := int32(5)
	if hs.MaxReplicas != nil {
		maxReplicas = *hs.MaxReplicas
	}
	cpuTarget := int32(80)
	if hs.TargetCPUUtilization != nil {
		cpuTarget = *hs.TargetCPUUtilization
	}

	metrics := []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: Ptr(cpuTarget),
				},
			},
		},
	}
	if hs.TargetMemoryUtilization != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: hs.TargetMemoryUtilization,
				},
			},
		})
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HPAName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       StatefulSetName(inst),
			},
			MinReplicas: Ptr(minReplicas),
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior:    hs.Behavior,
		},
	}
}
