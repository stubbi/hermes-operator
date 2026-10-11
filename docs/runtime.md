# Agent runtime

The operator runs the agent on the **upstream NousResearch/hermes-agent
container image**. The published `ghcr.io/paperclipinc/hermes-agent` image is
built `FROM` that upstream image (pinned by digest) with only operator metadata
layered on top — there is no hand-rolled venv build any more.

## What the image bundles

The upstream image is an **s6-overlay** runtime. A single image ships:

- the **gateway** (the long-lived agent daemon),
- a **dashboard**,
- the **OpenAI-compatible API server**,
- a **Playwright/Chromium browser**,
- node, ffmpeg, ripgrep, and every Python dependency.

Because the runtime lives entirely in the image, the old init-container chain
(`init-apt` / `init-uv` / `init-pip` running `uv sync` against a lockfile onto
the PVC) is **gone**. Only mutable state lives on the volume.

## How the operator runs it

- **Args:** the container runs `["gateway", "run"]` — the foreground gateway
  daemon.
- **API server:** enabled via env: `API_SERVER_ENABLED=true`,
  `API_SERVER_HOST=0.0.0.0`, `API_SERVER_PORT=8443`, and `API_SERVER_KEY` (see
  below). This serves the OpenAI-compatible `/v1/...` endpoint plus `/health` on
  the gateway port.
- **Probes:** both **readiness and liveness** are `HTTPGet /health` on the
  gateway port (8443) — not a TCP socket.

## State and `HERMES_HOME`

Persistent state lives at **`/opt/data`**, the PVC mount, and `HERMES_HOME` is
set to `/opt/data`. (The previous runtime used `/home/hermes/.hermes`.) The
rendered `config.yaml` is mounted read-only at `/opt/data/config.yaml`.
Because that is a subPath mount, kubelet never refreshes it inside a running
pod. The operator stamps the pod template with `hermes.agent/config-hash` (a
digest of the rendered ConfigMap) and `hermes.agent/workspace-hash` (the
workspace seed ConfigMap), so any change to either rolls the StatefulSet and
the agent restarts on the new content.

## Security posture and the SCC tradeoff

The s6 runtime changes the pod's security posture. `/init` **must be PID 1 and
start as root** so the s6 stage2 hook can remap the in-image `hermes` user to
`HERMES_UID`/`HERMES_GID` (1000) and chown `/opt/data`. After that, every
supervised service drops privileges via `s6-setuidgid`, so the actual workload
runs as uid 1000.

Consequences for the pod:

- **No** `runAsNonRoot`/`runAsUser` (the container starts as root by design).
- **No** read-only root filesystem (s6 needs a writable `/run` and `/etc` for
  the supervision tree).
- **No** drop-ALL capabilities — s6 needs `CHOWN`, `SETUID`, `SETGID`,
  `DAC_OVERRIDE`, and `FOWNER` to remap the user and chown the volume.
- `allowPrivilegeEscalation=false`, `fsGroup=1000`, and seccomp
  `RuntimeDefault` are retained.
- `shareProcessNamespace` defaults to **false**: s6 reaps zombies itself, and
  its `/init` must be PID 1 (the pause container becoming PID 1 would break it).

This is a deliberate tradeoff to adopt the supported upstream runtime. It means
the workload is **incompatible with OpenShift's `restricted`/`restricted-v2`
SCC**, and requires an SCC that permits a root-start container (for example
`anyuid`) — even though the workload internally drops to uid 1000.

## Configuring an LLM provider

Set the model and endpoint via `spec.config.raw`, and inject the API key via
`spec.env`:

```yaml
spec:
  config:
    raw:
      model: gpt-4o-mini
      base_url: https://api.openai.com/v1
  env:
    - name: OPENAI_API_KEY
      valueFrom:
        secretKeyRef:
          name: hermes-llm
          key: apiKey
```

When no `model` is configured, the operator injects a non-routable placeholder
provider so the gateway and API server still come up and `/health` passes,
without making any live LLM calls. Inference then fails clearly until a real
provider is configured.

## API server authentication

Each instance gets an operator-managed random `api_server_key` in its
`<name>-gateway-tokens` Secret. It is wired into the container as
`API_SERVER_KEY` and authenticates the OpenAI-compatible `/v1/...` API. The
`/health` endpoint is **unauthenticated**, which is what the readiness and
liveness probes hit.
