# Runbook: Platform Gateway Credentials

> Covers credential acquisition, scoping, rotation, and common pitfalls for each gateway supported by `spec.gateways`. Intended for cluster operators and platform engineers who are setting up or maintaining hermes-agent deployments.

## Table of Contents

- [Telegram](#telegram)
- [Discord](#discord)
- [Slack](#slack)
- [WhatsApp](#whatsapp)
- [Signal](#signal)
- [Common Patterns](#common-patterns)

---

## Telegram

### Acquire

1. Open Telegram and message `@BotFather`.
2. Send `/newbot` and follow the prompts (choose a display name and a username ending in `bot`).
3. BotFather replies with a bot API token in the form `<numeric-id>:<random-string>`.
4. Store the token immediately — BotFather does not re-display it; revoke and re-issue if lost.

### Scope

- The bot token grants full Bot API access. There is no additional scope granularity at the token level.
- Restrict who can message the bot via `spec.gateways.telegram.allowedUserIDs`. Find your Telegram user ID by messaging `@userinfobot`.
- For production, enable privacy mode via BotFather (`/setprivacy`) so the bot only sees messages addressed to it in groups.

### Wire into Kubernetes

```bash
kubectl create secret generic my-agent-telegram \
  --from-literal=token='<BOT_TOKEN>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    telegram:
      enabled: true
      botTokenSecretRef:
        name: my-agent-telegram
        key: token
      allowedUserIDs: [123456789]
```

### Rotate

1. Message `@BotFather`, send `/mybots`, select your bot, then "API Token" > "Revoke current token".
2. BotFather issues a new token.
3. Update the Secret: `kubectl create secret generic my-agent-telegram --from-literal=token='<NEW_TOKEN>' -n <namespace> --dry-run=client -o yaml | kubectl apply -f -`
4. The operator mounts the Secret via `envFrom`; the agent pod picks up the change on next restart. Trigger a rolling restart: `kubectl rollout restart statefulset/<agent-name> -n <namespace>`.

### Webhook vs Long-Poll

- Leave `spec.gateways.telegram.webhookURL` empty for long-poll (simpler, works behind NAT).
- Set it to a public HTTPS URL for webhook mode. Telegram requires a valid TLS certificate on port 443, 80, 88, or 8443.
- When using webhooks, also open an Ingress (or equivalent) for `POST /telegram-webhook` and ensure `spec.networking.ingress.enabled: true`.

### Pitfalls

- **Token in plaintext**: Never commit the token to git. Use a Secret, not a ConfigMap.
- **Multiple bot instances**: Running two agents with the same token causes update conflicts. Each agent must have a unique bot.
- **Webhook conflicts**: If you switch from webhook to long-poll, deregister the webhook first (`curl -X POST https://api.telegram.org/bot<TOKEN>/deleteWebhook`) or Telegram will continue delivering to the old URL.
- **Group vs DM**: By default, the bot receives all group messages it is added to (unless privacy mode is on). Add `allowedUserIDs` to prevent abuse.

---

## Discord

### Acquire

1. Go to [discord.com/developers/applications](https://discord.com/developers/applications) and click "New Application".
2. Note the **Application ID** (snowflake) on the General Information page.
3. Navigate to the **Bot** tab, click "Add Bot", then "Reset Token" to get the bot token (`Bot <token>`). Store it immediately.
4. Under **Privileged Gateway Intents**, enable "Message Content Intent" if the agent needs to read message bodies.

### Scope

- OAuth2 scopes needed to add the bot to a guild: `bot`, `applications.commands`.
- Bot permissions (bitfield) needed: `Send Messages`, `Read Message History`, `Use Slash Commands`. Generate the invite URL from the OAuth2 URL Generator.
- `spec.gateways.discord.guildIDs`: restricts slash-command registration to listed guilds (faster propagation than global commands; global commands take up to an hour).

### Wire into Kubernetes

```bash
kubectl create secret generic my-agent-discord \
  --from-literal=token='<BOT_TOKEN>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    discord:
      enabled: true
      botTokenSecretRef:
        name: my-agent-discord
        key: token
      applicationID: "123456789012345678"
      guildIDs: ["987654321098765432"]
```

### Rotate

1. In the Developer Portal, Bot tab, click "Reset Token". Confirm.
2. Update the Secret as shown in the Telegram section.
3. Trigger a rolling restart of the agent StatefulSet.

### Pitfalls

- **Token format**: The bot token is used without the `Bot ` prefix in the Secret; the agent library prepends it.
- **Intent mismatch**: If "Message Content Intent" is not enabled in the portal, the bot receives empty message bodies — a silent failure.
- **Rate limits**: Discord enforces a 5 requests/second per bot global rate limit. Do not run multiple agents with the same token.
- **Guild ID type**: Discord snowflakes are large integers; store them as strings in `guildIDs` to avoid YAML integer overflow.

---

## Slack

### Acquire

1. Go to [api.slack.com/apps](https://api.slack.com/apps) and click "Create New App" > "From scratch".
2. Under **Socket Mode**, enable it and generate an **App-Level Token** (`xapp-` prefix) with scope `connections:write`. Store immediately.
3. Under **OAuth & Permissions**, add bot token scopes: `app_mentions:read`, `chat:write`, `im:history`, `im:read`, `im:write`. Then click "Install to Workspace" to get the **Bot Token** (`xoxb-` prefix).
4. Under **Basic Information** > "App Credentials", copy the **Signing Secret**.

### Scope

- `xoxb-` bot token: scoped by the OAuth permissions you granted (step 3).
- `xapp-` app-level token: only for Socket Mode; do not confuse with the bot token.
- Signing secret: used to verify incoming webhook request authenticity. Required when using HTTP webhooks (not Socket Mode).

### Wire into Kubernetes

```bash
kubectl create secret generic my-agent-slack \
  --from-literal=botToken='xoxb-...' \
  --from-literal=appToken='xapp-...' \
  --from-literal=signingSecret='<SIGNING_SECRET>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    slack:
      enabled: true
      botTokenSecretRef:
        name: my-agent-slack
        key: botToken
      appTokenSecretRef:
        name: my-agent-slack
        key: appToken
      signingSecretRef:
        name: my-agent-slack
        key: signingSecret
```

### Rotate

1. In the Slack app settings, rotate each credential independently:
   - **Bot token**: OAuth & Permissions > Revoke tokens.
   - **App-level token**: App-Level Tokens > delete and regenerate.
   - **Signing secret**: Basic Information > Rotate.
2. Update the corresponding Secret keys and trigger a rolling restart.

### Pitfalls

- **Token confusion**: `xoxb-` and `xapp-` tokens are distinct. Using one where the other is expected causes silent authentication failures.
- **Socket Mode vs HTTP**: Socket Mode uses the `xapp-` token and does not require a public endpoint. HTTP Events API requires the signing secret and a public `SLACK_REQUEST_URL`. Do not enable both simultaneously.
- **Reinstallation**: If you add new OAuth scopes, you must reinstall the app to the workspace. The bot token changes on reinstall — update the Secret.
- **Workspace vs Enterprise**: Enterprise Grid apps have a different token structure. This runbook covers single-workspace apps.

---

## WhatsApp

### Acquire (Meta Cloud API)

1. Go to [developers.facebook.com](https://developers.facebook.com), create an app of type "Business".
2. Add the "WhatsApp" product. Under WhatsApp > Getting Started, copy the **temporary access token** and **Phone Number ID**.
3. For production, create a **System User** in Meta Business Manager, assign the WhatsApp app, generate a permanent token with `whatsapp_business_messaging` and `whatsapp_business_management` permissions.
4. Note the **Phone Number ID** and **WhatsApp Business Account ID**.

### Scope

- The operator mounts the entire Secret as environment variables with prefix `WHATSAPP_`. Name your Secret keys accordingly: `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, etc.
- The agent uses these to call `https://graph.facebook.com/v<version>/<phone-number-id>/messages`.

### Wire into Kubernetes

```bash
kubectl create secret generic my-agent-whatsapp \
  --from-literal=WHATSAPP_ACCESS_TOKEN='<TOKEN>' \
  --from-literal=WHATSAPP_PHONE_NUMBER_ID='<PHONE_NUMBER_ID>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    whatsapp:
      enabled: true
      providerSecretRef:
        name: my-agent-whatsapp
        key: WHATSAPP_ACCESS_TOKEN   # key is informational; the whole Secret is mounted
```

> Note: `providerSecretRef` identifies the Secret; the operator mounts **all** keys from that Secret as `envFrom.secretRef`, not just the referenced key. Name all keys with the `WHATSAPP_` prefix.

### Rotate

1. In Meta Business Manager, revoke the system user token and generate a new one.
2. Update `WHATSAPP_ACCESS_TOKEN` in the Secret.
3. Trigger a rolling restart of the agent StatefulSet.

### Pitfalls

- **Temporary vs permanent tokens**: The "Getting Started" token expires in ~24 hours. Always create a System User token for production.
- **Webhook verification**: Incoming webhooks from Meta require responding to a verification challenge (`hub.verify_token`). Ensure the agent's HTTP server handles this.
- **Twilio alternative**: If using Twilio's WhatsApp API instead of Meta Cloud, replace `graph.facebook.com` egress with `api.twilio.com` and adjust `gatewayEgressEndpoints.whatsapp` in `values.yaml`.
- **Phone number registration**: A phone number must be registered and verified in the Meta WhatsApp Business Account before it can send messages.

---

## Signal

### Acquire (signal-cli-rest-api)

The hermes-operator integrates with [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api), a self-hosted HTTP wrapper around signal-cli.

1. Deploy signal-cli-rest-api in your cluster (or externally). Refer to its README for setup.
2. Register a phone number with the Signal network through signal-cli: `POST /v1/register/<number>` then `POST /v1/register/<number>/verify/<code>`.
3. Generate an auth token (or use basic auth, depending on your signal-cli-rest-api configuration).

### Scope

- `SIGNAL_PHONE_NUMBER`: the registered phone number (e.g. `+15551234567`).
- `SIGNAL_AUTH_TOKEN`: authentication token for the signal-cli-rest-api HTTP API.
- `SIGNAL_API_URL`: base URL of your signal-cli-rest-api deployment (not managed by the operator — set via `spec.env`).

### Wire into Kubernetes

```bash
kubectl create secret generic my-agent-signal \
  --from-literal=phoneNumber='+15551234567' \
  --from-literal=authToken='<AUTH_TOKEN>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    signal:
      enabled: true
      phoneNumberSecretRef:
        name: my-agent-signal
        key: phoneNumber
      authTokenSecretRef:
        name: my-agent-signal
        key: authToken
  env:
    - name: SIGNAL_API_URL
      value: "http://signal-cli-rest-api.signal-system.svc.cluster.local:8080"
```

### Rotate

1. Regenerate the auth token in your signal-cli-rest-api deployment.
2. Update the Secret.
3. Trigger a rolling restart of the agent StatefulSet.

### Pitfalls

- **Phone number re-registration**: Re-registering a Signal number invalidates all existing sessions. Do this only intentionally and expect a brief messaging outage.
- **Self-hosted latency**: signal-cli-rest-api has higher message latency than the native Signal protocol. For low-latency requirements, consider co-locating it with the agent.
- **NetworkPolicy egress**: The operator's generated NetworkPolicy adds `chat.signal.org:443` to the allowed egress. If you use a self-hosted signal-cli-rest-api in a different namespace, also add an egress rule for its Service via `spec.security.networkPolicy.additionalEgress`.
- **No FQDN peer support**: On CNIs without FQDN peer support, the `chat.signal.org` rule degrades to port-443 to any destination. See `docs/conventions.md` — Well-known egress endpoints.

---

## Common Patterns

### Single Secret, multiple keys

You can store all gateway credentials in one Secret and reference individual keys:

```bash
kubectl create secret generic my-agent-gateways \
  --from-literal=telegram-token='<TOKEN>' \
  --from-literal=discord-token='<TOKEN>' \
  -n <namespace>
```

```yaml
spec:
  gateways:
    telegram:
      enabled: true
      botTokenSecretRef:
        name: my-agent-gateways
        key: telegram-token
    discord:
      enabled: true
      botTokenSecretRef:
        name: my-agent-gateways
        key: discord-token
```

### Sealed Secrets / External Secrets Operator

Use [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) or [External Secrets Operator](https://external-secrets.io) to manage gateway credentials as GitOps-safe encrypted manifests or vault references. The operator does not require any specific secret management tool — it consumes standard `v1.Secret` objects.

### Checking which gateways are active

```bash
kubectl get hi <name> -n <namespace> -o jsonpath='{.status.conditions}' | jq '.[] | select(.type=="Ready")'
```

Gateway-specific env vars injected by the operator are visible in:

```bash
kubectl exec -it <pod-name> -n <namespace> -- env | grep -E 'TELEGRAM|DISCORD|SLACK|WHATSAPP|SIGNAL|HONCHO'
```

### Testing connectivity without a live agent

Use a temporary pod to verify egress reaches the gateway endpoints:

```bash
kubectl run curl-test --rm -it --image=curlimages/curl --restart=Never -n <namespace> -- \
  curl -s -o /dev/null -w "%{http_code}" https://api.telegram.org
```

Expected: `404` (not `000` which indicates a connection failure). The Bot API returns 404 for requests without a valid token path.
