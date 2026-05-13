package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildPrometheusRule_DefaultGroup(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				PrometheusRule: hermesv1.PrometheusRuleSpec{Enabled: Ptr(true)},
			},
		},
	}
	pr := BuildPrometheusRule(inst)
	assert.Equal(t, "monitoring.coreos.com/v1", pr.GetAPIVersion())
	assert.Equal(t, "PrometheusRule", pr.GetKind())
	assert.Equal(t, "demo", pr.GetName())
	spec := pr.Object["spec"].(map[string]interface{})
	groups := spec["groups"].([]interface{})
	assert.Len(t, groups, 1)
	g := groups[0].(map[string]interface{})
	rules := g["rules"].([]interface{})
	assert.NotEmpty(t, rules, "default rules: HermesHighRestartRate + HermesMetricsDown")
	var names []string
	for _, r := range rules {
		rm := r.(map[string]interface{})
		names = append(names, rm["alert"].(string))
	}
	assert.Contains(t, names, "HermesHighRestartRate")
}

func TestBuildPrometheusRule_AdditionalRulesAppended(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				PrometheusRule: hermesv1.PrometheusRuleSpec{
					Enabled: Ptr(true),
					AdditionalRules: []hermesv1.PrometheusRule{
						{Alert: "MyAlert", Expr: "up == 0", For: "5m"},
					},
				},
			},
		},
	}
	pr := BuildPrometheusRule(inst)
	rules := pr.Object["spec"].(map[string]interface{})["groups"].([]interface{})[0].(map[string]interface{})["rules"].([]interface{})
	names := []string{}
	for _, r := range rules {
		names = append(names, r.(map[string]interface{})["alert"].(string))
	}
	assert.Contains(t, names, "MyAlert")
}
