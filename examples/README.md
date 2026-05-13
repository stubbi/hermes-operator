# Hermes Operator: Examples

End-to-end worked YAML recipes. Every example folder contains a `README.md`
explaining the scenario and one or more manifests you can `kubectl apply` in
order.

All examples assume the operator and CRDs are already installed (see the
[Quickstart](../README.md#quickstart)) and target the `agents` namespace:
adjust as needed.

| Example | Scenario |
|---|---|
| [`minimal/`](minimal/) | Smallest possible `HermesInstance`: image + PVC, no gateways. |
| [`full-featured/`](full-featured/) | Every sub-spec exercised at least once: gateways, Honcho, auto-update, backup, observability, scheduling. |
| [`multi-platform/`](multi-platform/) | Telegram + Discord + Slack + WhatsApp + Signal all enabled. |
| [`honcho/`](honcho/) | Honcho profile store enabled with persistence. |
| [`auto-update/`](auto-update/) | OCI-registry-driven auto-update with rollback. |
| [`backup-s3/`](backup-s3/) | S3 backups against a kind-local MinIO. |
| [`migration-from-openclaw/`](migration-from-openclaw/) | One-shot OpenClaw → Hermes migration, both source modes. |
| [`gitops-fluxcd/`](gitops-fluxcd/) | FluxCD owns the `HermesInstance`; agent self-mutates via SSA without flap. |
| [`cluster-defaults/`](cluster-defaults/) | `HermesClusterDefaults` singleton with image/storage/observability defaults. |

If you have a scenario you would like to see worked through, open an issue
tagged `examples` or a PR: the format is intentionally small so additions
are cheap.
