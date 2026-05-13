package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func instWithGateways(g hermesv1.GatewaysSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{Gateways: g},
	}
}

func TestBuildGatewayEnvFrom_Telegram(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
				Key:                  "token",
			},
		},
	})
	envFrom := BuildGatewayEnvFrom(inst)
	assert.Empty(t, envFrom, "BotTokenSecretRef is a single-key selector, not whole-secret envFrom")
}

func TestBuildGatewayEnv_Telegram(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
				Key:                  "token",
			},
			AllowedUserIDs: []int64{42, 1337},
			WebhookURL:     "https://example.com/tg",
		},
	})
	env := BuildGatewayEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}

	tok, ok := byName["TELEGRAM_BOT_TOKEN"]
	assert.True(t, ok)
	assert.NotNil(t, tok.ValueFrom)
	assert.NotNil(t, tok.ValueFrom.SecretKeyRef)
	assert.Equal(t, "tg-secret", tok.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "token", tok.ValueFrom.SecretKeyRef.Key)

	allowed, ok := byName["TELEGRAM_ALLOWED_USER_IDS"]
	assert.True(t, ok)
	assert.Equal(t, "42,1337", allowed.Value)

	wh, ok := byName["TELEGRAM_WEBHOOK_URL"]
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/tg", wh.Value)
}

func TestBuildGatewayEnv_Discord(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Discord: hermesv1.DiscordGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "dc-secret"},
				Key:                  "token",
			},
			ApplicationID: "111222333",
			GuildIDs:      []string{"444", "555"},
		},
	})
	env := BuildGatewayEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}
	assert.Equal(t, "111222333", byName["DISCORD_APPLICATION_ID"].Value)
	assert.Equal(t, "444,555", byName["DISCORD_GUILD_IDS"].Value)
	assert.NotNil(t, byName["DISCORD_BOT_TOKEN"].ValueFrom)
}

func TestBuildGatewayEnv_Slack_AllThreeRefs(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Slack: hermesv1.SlackGatewaySpec{
			Enabled:           Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "bot"},
			AppTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "app"},
			SigningSecretRef:  &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "slk"}, Key: "sig"},
		},
	})
	env := BuildGatewayEnv(inst)
	names := map[string]bool{}
	for _, e := range env {
		names[e.Name] = true
	}
	assert.True(t, names["SLACK_BOT_TOKEN"])
	assert.True(t, names["SLACK_APP_TOKEN"])
	assert.True(t, names["SLACK_SIGNING_SECRET"])
}

func TestBuildGatewayEnv_Disabled(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(false)},
	})
	assert.Empty(t, BuildGatewayEnv(inst), "disabled gateway contributes no env vars")
}

func TestBuildGatewayConfigFragments_Shape(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
		Discord:  hermesv1.DiscordGatewaySpec{Enabled: Ptr(true), ApplicationID: "111"},
	})
	frags := BuildGatewayConfigFragments(inst)
	tg, ok := frags["telegram"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, tg["enabled"])

	dc, ok := frags["discord"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, dc["enabled"])
	assert.Equal(t, "111", dc["applicationID"])
}

func TestBuildGatewayEgressEndpoints(t *testing.T) {
	inst := instWithGateways(hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
		Discord:  hermesv1.DiscordGatewaySpec{Enabled: Ptr(true)},
		Slack:    hermesv1.SlackGatewaySpec{Enabled: Ptr(true)},
	})
	hosts := BuildGatewayEgressEndpoints(inst)
	assert.Contains(t, hosts, "api.telegram.org")
	assert.Contains(t, hosts, "discord.com")
	assert.Contains(t, hosts, "slack.com")
	assert.NotContains(t, hosts, "signal.org", "Signal endpoint only when enabled")
}
