# Multi-platform gateways

Telegram, Discord, Slack, WhatsApp, and Signal all enabled on one
`HermesInstance`. Each gateway carries its own Secret so tokens can be
rotated independently.

## Apply

```bash
kubectl create namespace agents

# Replace the placeholder tokens before applying.
$EDITOR secrets.yaml
kubectl apply -n agents -f secrets.yaml
kubectl apply -n agents -f hermesinstance.yaml
```

## Verify gateway wiring

```bash
kubectl get hi multi-platform -n agents \
  -o jsonpath='{.status.gateways}' | jq
# {
#   "telegram":  { "enabled": true, "ready": true, "lastError": "" },
#   "discord":   { "enabled": true, "ready": true, "lastError": "" },
#   "slack":     { "enabled": true, "ready": true, "lastError": "" },
#   "whatsapp":  { "enabled": true, "ready": true, "lastError": "" },
#   "signal":    { "enabled": true, "ready": true, "lastError": "" }
# }
```

The aggregate `GatewayReady` condition is `True` only when every enabled
gateway is `ready: true`. If a single token Secret is missing, the condition
flips to `False` with reason `TokenSecretMissing` and the `message` names
the offending gateways.

## Rotating a token

Update the Secret in place (each `secretRef` watches its Secret via the
informer cache):

```bash
kubectl create secret generic hermes-slack \
  -n agents \
  --from-literal=token=NEW_TOKEN \
  --dry-run=client -o yaml | kubectl apply -f -
```

The operator picks up the change within ~30s and rolls the StatefulSet so
the new token is read. No restart needed for the other gateways.
