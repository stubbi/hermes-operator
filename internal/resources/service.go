package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// ServiceName returns the deterministic Service name.
func ServiceName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildService constructs the desired Service. Honors spec.networking.service
// (Type, ClusterIP, Ports, Annotations, LoadBalancerClass, ExternalTrafficPolicy);
// appends a "metrics" port automatically when spec.observability.metrics.enabled.
func BuildService(inst *hermesv1.HermesInstance) *corev1.Service {
	ss := inst.Spec.Networking.Service

	svcType := ss.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}

	ports := buildServicePorts(inst)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: ss.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                  svcType,
			ClusterIP:             ss.ClusterIP,
			SessionAffinity:       corev1.ServiceAffinityNone,
			Selector:              SelectorLabels(inst),
			Ports:                 ports,
			LoadBalancerClass:     ss.LoadBalancerClass,
			ExternalTrafficPolicy: ss.ExternalTrafficPolicy,
		},
	}
}

func buildServicePorts(inst *hermesv1.HermesInstance) []corev1.ServicePort {
	ports := []corev1.ServicePort{}
	custom := inst.Spec.Networking.Service.Ports
	if len(custom) > 0 {
		for _, p := range custom {
			protocol := p.Protocol
			if protocol == "" {
				protocol = corev1.ProtocolTCP
			}
			target := p.Port
			if p.TargetPort != nil {
				target = *p.TargetPort
			}
			sp := corev1.ServicePort{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: intstr.FromInt32(target),
				Protocol:   protocol,
				NodePort:   p.NodePort,
			}
			ports = append(ports, sp)
		}
	} else {
		ports = append(ports, corev1.ServicePort{
			Name:       GatewayPortName,
			Port:       GatewayPort,
			TargetPort: intstr.FromString(GatewayPortName),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	if BoolValueOrDefault(inst.Spec.Observability.Metrics.Enabled, true) {
		port := inst.Spec.Observability.Metrics.Port
		if port == 0 {
			port = DefaultMetricsPort
		}
		seen := false
		for _, p := range ports {
			if p.Name == MetricsPortName {
				seen = true
				break
			}
		}
		if !seen {
			ports = append(ports, corev1.ServicePort{
				Name:       MetricsPortName,
				Port:       port,
				TargetPort: intstr.FromString(MetricsPortName),
				Protocol:   corev1.ProtocolTCP,
			})
		}
	}
	return ports
}
