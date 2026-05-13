package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// ServiceMonitorGVK is the GroupVersionKind we emit.
func ServiceMonitorGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
}

// ServiceMonitorName returns the deterministic name.
func ServiceMonitorName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildServiceMonitor returns an unstructured ServiceMonitor. Scheme on the
// endpoint follows spec.observability.metrics.secure (lesson #435/#440).
func BuildServiceMonitor(inst *hermesv1.HermesInstance) *unstructured.Unstructured {
	labels := map[string]string{}
	for k, v := range LabelsForInstance(inst) {
		labels[k] = v
	}
	for k, v := range inst.Spec.Observability.ServiceMonitor.Labels {
		labels[k] = v
	}

	interval := inst.Spec.Observability.ServiceMonitor.Interval
	if interval == "" {
		interval = "30s"
	}
	scrapeTimeout := inst.Spec.Observability.ServiceMonitor.ScrapeTimeout
	if scrapeTimeout == "" {
		scrapeTimeout = "10s"
	}
	scheme := "http"
	if BoolValue(inst.Spec.Observability.Metrics.Secure) {
		scheme = "https"
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ServiceMonitorGVK().GroupVersion().String(),
			"kind":       ServiceMonitorGVK().Kind,
			"metadata": map[string]interface{}{
				"name":      ServiceMonitorName(inst),
				"namespace": inst.Namespace,
				"labels":    toIface(labels),
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": toIface(SelectorLabels(inst)),
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":          MetricsPortName,
						"interval":      interval,
						"scrapeTimeout": scrapeTimeout,
						"path":          "/metrics",
						"scheme":        scheme,
					},
				},
			},
		},
	}
}

func toIface(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
