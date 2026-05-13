package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildConfigMap_EmptyConfig(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	cm := BuildConfigMap(inst, "")
	assert.Equal(t, "demo-config", cm.Name)
	assert.Equal(t, "{}\n", cm.Data["config.yaml"])
}

func TestBuildConfigMap_RawBody(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"telegram":{"enabled":true}}`)}},
			},
		},
	}
	cm := BuildConfigMap(inst, "")
	body := cm.Data["config.yaml"]
	assert.Contains(t, body, "telegram:")
	assert.Contains(t, body, "enabled: true")
}

func TestBuildConfigMap_RefOnly_PassesResolvedBody(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	resolved := "discord:\n  enabled: true\n"
	cm := BuildConfigMap(inst, resolved)
	assert.Equal(t, resolved, cm.Data["config.yaml"])
}

func TestBuildConfigMap_MergeMode(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw:       &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"telegram":{"enabled":true}}`)}},
				MergeMode: hermesv1.ConfigMergeModeMerge,
			},
		},
	}
	cm := BuildConfigMap(inst, "discord:\n  enabled: true\ntelegram:\n  enabled: true\n")
	assert.Contains(t, cm.Data["config.yaml"], "discord:")
	assert.Contains(t, cm.Data["config.yaml"], "telegram:")
}

func TestMergeYAMLBodies(t *testing.T) {
	t.Parallel()
	base := "discord:\n  enabled: true\n"
	overlay := `{"telegram":{"enabled":true},"discord":{"enabled":false}}`
	got, err := MergeYAMLBodies(base, overlay)
	assert.NoError(t, err)
	assert.Contains(t, got, "telegram:")
	assert.Contains(t, got, "enabled: false")
}

func TestBuildConfigMap_MergesGatewayFragments(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"schedules":{"morning":"0 8 * * *"}}`)}},
			},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true), WebhookURL: "https://x/tg"},
			},
		},
	}
	cm := BuildConfigMap(inst, "")
	body := cm.Data["config.yaml"]
	assert.Contains(t, body, "schedules:")
	assert.Contains(t, body, "gateways:")
	assert.Contains(t, body, "telegram:")
	assert.Contains(t, body, "webhookURL: https://x/tg")
}

func TestBuildConfigMap_NoGatewaysWhenAllDisabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	cm := BuildConfigMap(inst, "")
	assert.NotContains(t, cm.Data["config.yaml"], "gateways:")
}
