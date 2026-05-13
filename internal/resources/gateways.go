package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BuildGatewayEnvFrom returns whole-Secret envFrom entries. Reserved for
// gateways that use a single Secret with multiple keys (e.g. WhatsApp).
func BuildGatewayEnvFrom(inst *hermesv1.HermesInstance) []corev1.EnvFromSource {
	var out []corev1.EnvFromSource
	g := inst.Spec.Gateways

	if BoolValue(g.WhatsApp.Enabled) && g.WhatsApp.ProviderSecretRef != nil {
		out = append(out, corev1.EnvFromSource{
			Prefix: "WHATSAPP_",
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: g.WhatsApp.ProviderSecretRef.Name},
			},
		})
	}
	return out
}

// BuildGatewayEnv returns explicit per-gateway env vars.
func BuildGatewayEnv(inst *hermesv1.HermesInstance) []corev1.EnvVar {
	var out []corev1.EnvVar
	g := inst.Spec.Gateways

	if BoolValue(g.Telegram.Enabled) {
		if ref := g.Telegram.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("TELEGRAM_BOT_TOKEN", ref))
		}
		if len(g.Telegram.AllowedUserIDs) > 0 {
			out = append(out, corev1.EnvVar{
				Name:  "TELEGRAM_ALLOWED_USER_IDS",
				Value: joinInt64s(g.Telegram.AllowedUserIDs, ","),
			})
		}
		if g.Telegram.WebhookURL != "" {
			out = append(out, corev1.EnvVar{Name: "TELEGRAM_WEBHOOK_URL", Value: g.Telegram.WebhookURL})
		}
	}

	if BoolValue(g.Discord.Enabled) {
		if ref := g.Discord.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("DISCORD_BOT_TOKEN", ref))
		}
		if g.Discord.ApplicationID != "" {
			out = append(out, corev1.EnvVar{Name: "DISCORD_APPLICATION_ID", Value: g.Discord.ApplicationID})
		}
		if len(g.Discord.GuildIDs) > 0 {
			out = append(out, corev1.EnvVar{
				Name:  "DISCORD_GUILD_IDS",
				Value: strings.Join(g.Discord.GuildIDs, ","),
			})
		}
	}

	if BoolValue(g.Slack.Enabled) {
		if ref := g.Slack.BotTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_BOT_TOKEN", ref))
		}
		if ref := g.Slack.AppTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_APP_TOKEN", ref))
		}
		if ref := g.Slack.SigningSecretRef; ref != nil {
			out = append(out, secretEnv("SLACK_SIGNING_SECRET", ref))
		}
	}

	if BoolValue(g.Signal.Enabled) {
		if ref := g.Signal.PhoneNumberSecretRef; ref != nil {
			out = append(out, secretEnv("SIGNAL_PHONE_NUMBER", ref))
		}
		if ref := g.Signal.AuthTokenSecretRef; ref != nil {
			out = append(out, secretEnv("SIGNAL_AUTH_TOKEN", ref))
		}
	}

	return out
}

// BuildGatewayConfigFragments returns the typed Go shape of the `gateways:`
// sub-tree of config.yaml. configmap.go merges this under the user's raw config.
func BuildGatewayConfigFragments(inst *hermesv1.HermesInstance) map[string]any {
	out := map[string]any{}
	g := inst.Spec.Gateways

	if BoolValue(g.Telegram.Enabled) {
		out["telegram"] = map[string]any{
			"enabled":        true,
			"webhookURL":     g.Telegram.WebhookURL,
			"allowedUserIDs": g.Telegram.AllowedUserIDs,
		}
	}
	if BoolValue(g.Discord.Enabled) {
		out["discord"] = map[string]any{
			"enabled":       true,
			"applicationID": g.Discord.ApplicationID,
			"guildIDs":      g.Discord.GuildIDs,
		}
	}
	if BoolValue(g.Slack.Enabled) {
		out["slack"] = map[string]any{"enabled": true}
	}
	if BoolValue(g.WhatsApp.Enabled) {
		out["whatsapp"] = map[string]any{"enabled": true}
	}
	if BoolValue(g.Signal.Enabled) {
		out["signal"] = map[string]any{"enabled": true}
	}
	return out
}

// BuildGatewayEgressEndpoints returns upstream hosts each enabled gateway needs.
func BuildGatewayEgressEndpoints(inst *hermesv1.HermesInstance) []string {
	var out []string
	g := inst.Spec.Gateways
	if BoolValue(g.Telegram.Enabled) {
		out = append(out, "api.telegram.org")
	}
	if BoolValue(g.Discord.Enabled) {
		out = append(out, "discord.com", "gateway.discord.gg")
	}
	if BoolValue(g.Slack.Enabled) {
		out = append(out, "slack.com", "wss-primary.slack.com")
	}
	if BoolValue(g.WhatsApp.Enabled) {
		out = append(out, "graph.facebook.com")
	}
	if BoolValue(g.Signal.Enabled) {
		out = append(out, "chat.signal.org")
	}
	return out
}

func secretEnv(name string, ref *corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: ref.LocalObjectReference,
				Key:                  ref.Key,
			},
		},
	}
}

func joinInt64s(in []int64, sep string) string {
	if len(in) == 0 {
		return ""
	}
	parts := make([]string, len(in))
	for i, n := range in {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, sep)
}
