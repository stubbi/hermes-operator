package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// PrometheusRuleGVK is the GroupVersionKind emitted.
func PrometheusRuleGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"}
}

// PrometheusRuleName returns the deterministic name.
func PrometheusRuleName(inst *hermesv1.HermesInstance) string { return inst.Name }

func defaultPrometheusRules(inst *hermesv1.HermesInstance) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"alert": "HermesHighRestartRate",
			"expr":  `sum by (pod) (rate(kube_pod_container_status_restarts_total{pod=~"` + inst.Name + `-.*"}[10m])) > 0.1`,
			"for":   "10m",
			"labels": map[string]interface{}{
				"severity": "warning",
				"instance": inst.Name,
			},
			"annotations": map[string]interface{}{
				"summary":     "Hermes pod restarting frequently",
				"description": "Pod {{$labels.pod}} has been restarting > 0.1/min for 10m",
			},
		},
		map[string]interface{}{
			"alert": "HermesMetricsDown",
			"expr":  `up{job="` + inst.Name + `"} == 0`,
			"for":   "5m",
			"labels": map[string]interface{}{
				"severity": "warning",
				"instance": inst.Name,
			},
			"annotations": map[string]interface{}{
				"summary":     "Hermes metrics endpoint is down",
				"description": "Pod {{$labels.pod}} stopped serving /metrics for 5m",
			},
		},
	}
}

// BuildPrometheusRule emits a PrometheusRule containing the operator-default
// alerts plus any spec.observability.prometheusRule.additionalRules.
func BuildPrometheusRule(inst *hermesv1.HermesInstance) *unstructured.Unstructured {
	rules := defaultPrometheusRules(inst)
	for _, r := range inst.Spec.Observability.PrometheusRule.AdditionalRules {
		entry := map[string]interface{}{
			"alert": r.Alert,
			"expr":  r.Expr,
		}
		if r.For != "" {
			entry["for"] = r.For
		}
		if len(r.Labels) > 0 {
			entry["labels"] = toIface(r.Labels)
		}
		if len(r.Annotations) > 0 {
			entry["annotations"] = toIface(r.Annotations)
		}
		rules = append(rules, entry)
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": PrometheusRuleGVK().GroupVersion().String(),
			"kind":       PrometheusRuleGVK().Kind,
			"metadata": map[string]interface{}{
				"name":      PrometheusRuleName(inst),
				"namespace": inst.Namespace,
				"labels":    toIface(LabelsForInstance(inst)),
			},
			"spec": map[string]interface{}{
				"groups": []interface{}{
					map[string]interface{}{
						"name":  "hermes-" + inst.Name,
						"rules": rules,
					},
				},
			},
		},
	}
}
