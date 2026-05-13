package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildServiceMonitor_Basics(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true)},
				ServiceMonitor: hermesv1.ServiceMonitorSpec{
					Enabled:       Ptr(true),
					Interval:      "60s",
					ScrapeTimeout: "20s",
					Labels:        map[string]string{"release": "kps"},
				},
			},
		},
	}
	sm := BuildServiceMonitor(inst)
	assert.Equal(t, "monitoring.coreos.com/v1", sm.GetAPIVersion())
	assert.Equal(t, "ServiceMonitor", sm.GetKind())
	assert.Equal(t, "demo", sm.GetName())
	assert.Equal(t, "agents", sm.GetNamespace())
	labels := sm.GetLabels()
	assert.Equal(t, "kps", labels["release"])
	spec, _, _ := getNestedMap(sm.Object, "spec")
	endpoints := spec["endpoints"].([]interface{})
	ep := endpoints[0].(map[string]interface{})
	assert.Equal(t, "60s", ep["interval"])
	assert.Equal(t, "20s", ep["scrapeTimeout"])
	assert.Equal(t, "metrics", ep["port"])
	assert.Equal(t, "http", ep["scheme"])
}

func TestBuildServiceMonitor_SecureSchemeMatchesMetricsSecure(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics:        hermesv1.MetricsSpec{Enabled: Ptr(true), Secure: Ptr(true)},
				ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: Ptr(true)},
			},
		},
	}
	sm := BuildServiceMonitor(inst)
	spec, _, _ := getNestedMap(sm.Object, "spec")
	ep := spec["endpoints"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "https", ep["scheme"], "lesson #435: scheme must follow metrics.secure")
}

func TestServiceMonitorName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", ServiceMonitorName(inst))
}

func getNestedMap(obj map[string]interface{}, key string) (map[string]interface{}, bool, error) {
	v, ok := obj[key]
	if !ok {
		return nil, false, nil
	}
	m, _ := v.(map[string]interface{})
	return m, true, nil
}
