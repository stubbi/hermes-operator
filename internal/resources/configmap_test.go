package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

func TestBuildConfigMap_EmptyConfig(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	cm := BuildConfigMap(inst, "")
	assert.Equal(t, "demo-config", cm.Name)
	// With no top-level `model`, the builder injects a non-routable placeholder
	// provider so `gateway run` can start and serve /health without live LLM calls.
	body := cm.Data["config.yaml"]
	assert.Contains(t, body, "model: gpt-4o-mini")
	assert.Contains(t, body, "base_url: http://127.0.0.1:9/v1")
	assert.Contains(t, body, "api_key: placeholder-no-live-calls")
	assert.NotContains(t, body, "gateways:")
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
	body := cm.Data["config.yaml"]
	// The resolved body is preserved verbatim except that, since it has no
	// top-level `model`, the placeholder provider is injected on top.
	assert.Contains(t, body, "discord:")
	assert.Contains(t, body, "enabled: true")
	assert.Contains(t, body, "model: gpt-4o-mini")
	assert.Contains(t, body, "base_url: http://127.0.0.1:9/v1")
	assert.Contains(t, body, "api_key: placeholder-no-live-calls")
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

func TestBuildConfigMap_UserModelNotOverridden(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"model":"gpt-5"}`)}},
			},
		},
	}
	cm := BuildConfigMap(inst, "")
	body := cm.Data["config.yaml"]
	// A user-supplied top-level `model` suppresses the placeholder entirely.
	assert.Contains(t, body, "model: gpt-5")
	assert.NotContains(t, body, "gpt-4o-mini")
	assert.NotContains(t, body, "placeholder-no-live-calls")
}

func TestBuildConfigMap_NoGatewaysWhenAllDisabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	cm := BuildConfigMap(inst, "")
	assert.NotContains(t, cm.Data["config.yaml"], "gateways:")
}

func TestApplyJSONMergePatch_NullDeletesKey(t *testing.T) {
	t.Parallel()
	base := "memory:\n  provider: builtin\n  ttl: 30\nmodel: gpt-4o\n"
	got, err := ApplyJSONMergePatch(base, `{"memory":{"provider":"hindsight","ttl":null},"extra":{"a":1}}`)
	assert.NoError(t, err)
	assert.Contains(t, got, "provider: hindsight")
	assert.NotContains(t, got, "ttl")
	assert.NotContains(t, got, "null")
	assert.Contains(t, got, "model: gpt-4o")
	assert.Contains(t, got, "a: 1")
}

func TestBuildConfigMapWithPatches_LayersInOrderBelowGateways(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"model":"gpt-4o","memory":{"provider":"builtin"}}`)}},
			},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true), WebhookURL: "https://x/tg"},
			},
		},
	}
	patches := []string{
		`{"memory":{"provider":"hindsight"},"timezone":"UTC"}`,
		`{"timezone":"Europe/Berlin"}`,
		`{"gateways":{"telegram":{"webhookURL":"https://evil/tg"}}}`,
		`{not json`,
	}
	body := BuildConfigMapWithPatches(inst, "", patches).Data["config.yaml"]
	assert.Contains(t, body, "model: gpt-4o")
	assert.Contains(t, body, "provider: hindsight")
	assert.Contains(t, body, "timezone: Europe/Berlin")
	assert.Contains(t, body, "webhookURL: https://x/tg", "operator gateway fragment must win over a patch")
	assert.NotContains(t, body, "evil")
	// BuildConfigMap is the no-patch special case and must stay identical.
	assert.Equal(t, BuildConfigMap(inst, "").Data, BuildConfigMapWithPatches(inst, "", nil).Data)
}
