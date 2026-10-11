# Secrets broker

`spec.secretsBroker` lets an agent pick up short-lived secret material (model
API keys, tool tokens) from an external broker every time its container
starts, without any plaintext value in the HermesInstance, its ConfigMap, or
the PVC.

## How it works

1. An init container (`init-secrets-broker`) copies the resolver, which is the
   operator binary itself, into a pod-local `emptyDir`.
2. The agent container's entrypoint becomes
   `resolver secrets-broker exec -- /init /opt/hermes/docker/main-wrapper.sh`
   (the args stay `gateway run`).
3. The resolver reads the broker config from the `HERMES_SECRETS_BROKER_CONFIG`
   env var, interpolates `${VAR}` placeholders from the container env, calls
   the broker, and on success `execve`s the real entrypoint with the secrets
   added to the environment. s6-overlay's `/init` still runs as PID 1 and
   passes the environment to the gateway.

Every container (re)start resolves afresh, so rotation and revocation are
owned by the broker and take effect on the next start.

## Broker contract

The broker answers `2xx` with a JSON object mapping secret name to plaintext
value for the currently active secrets only:

```json
{"OPENAI_API_KEY": "sk-...", "GITHUB_TOKEN": "ghp_..."}
```

Each name is exported as an env var (optionally prefixed by `envPrefix`). Names
must be valid env var names, and a broker may not override runtime variables
(`PATH`, `HOME`, `HERMES_HOME`, `HERMES_UID`/`HERMES_GID`, `API_SERVER_*`, and
anything starting with `LD_`, `S6_`, `PYTHON` or `SSL_CERT_`). A secret with
the same name as an existing env var (for example a placeholder
`OPENAI_API_KEY` in `spec.env`) replaces it.

## Example

```yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: employee-7
spec:
  env:
    - name: EMPLOYEE_ID
      value: emp-7
    - name: WORKSPACE_ID
      value: ws-1
  secretsBroker:
    url: https://broker.example.com/secrets/resolve/${EMPLOYEE_ID}
    headers:
      - name: Authorization
        valueFrom:
          secretKeyRef: { name: broker-auth, key: header }   # "Bearer <token>"
      - name: X-Employee-Id
        value: ${EMPLOYEE_ID}
      - name: X-Workspace-Id
        value: ${WORKSPACE_ID}
```

### Mapping from the `secrets_broker` block

The issue that motivated this feature (#138) proposed a `secrets_broker` job
config block. `spec.secretsBroker` is its operator-side form, and the
operator renders it into exactly that shape (`url`, `headers`, plus the knobs
below) in `HERMES_SECRETS_BROKER_CONFIG`:

| `secrets_broker` block | `spec.secretsBroker` |
|---|---|
| `url` with `${...}` placeholders | `url`, same placeholders |
| `headers` map | `headers` list; `value` keeps placeholders, `valueFrom.secretKeyRef` keeps the credential out of the CR |
| fetch timing | eager, at every container start |

A header sourced from a Secret reaches the pod only as a `secretKeyRef` env
var (`HERMES_SECRETS_BROKER_HEADER_<n>`); the rendered config refers to it by
placeholder. The broker config env var itself is removed from the agent's
environment before the entrypoint runs.

## Fields

| Field | Default | Description |
|---|---|---|
| `url` | required | Resolve endpoint. Must be `https://` unless `allowInsecureHTTP`. |
| `method` | `GET` | `GET` or `POST`. |
| `headers[]` | none | `name` plus exactly one of `value` (placeholders allowed) or `valueFrom.secretKeyRef`. A literal `Authorization` value without a placeholder is rejected. |
| `timeoutSeconds` | `10` | Per-attempt timeout (1 to 300). |
| `maxAttempts` | `3` | Attempts for transient failures (1 to 10). |
| `envPrefix` | none | Prefix added to every exported name. |
| `allowInsecureHTTP` | `false` | Permit `http://` for an in-cluster broker on a trusted network. |
| `resolverImage` | operator image | Image the resolver is copied from. |
| `entrypoint` | `["/init", "/opt/hermes/docker/main-wrapper.sh"]` | Entrypoint exec'd after resolving; set it for a custom agent image with a different `ENTRYPOINT`. |

## Failure behavior (fail closed)

The agent never starts on stale or empty secrets:

- `401`/`403` (bad broker credentials), `404` (no active secrets for this
  identity), any other `4xx`, a malformed body, or a refused secret name fail
  the container immediately with an actionable message in its log.
- Network errors, timeouts, `429` and `5xx` are retried with exponential
  backoff (1s, 2s, 4s, ...) up to `maxAttempts`, then fail.
- An unset or empty `${VAR}` placeholder fails before any request is sent.

A failing resolve shows up as a container in `CrashLoopBackOff` with the
resolver's error in `kubectl logs`.

## What is and is not logged

The resolver logs the broker scheme, host and path, the HTTP status, and the
names of resolved secrets. It never logs secret values, header values, the
query string, or the broker's response body (error bodies are discarded).

## Persistence

Secrets exist only in the environment of the agent process tree. Nothing is
written to the PVC (`/opt/data`), so they never reach snapshots or backups.
s6-overlay may keep a copy of the container environment under `/run/s6`
inside the container; that is container-local and discarded with the
container.

Env vars do not hot-reload: a rotated secret is picked up on the next
container start. Restart the pod (`kubectl rollout restart statefulset/<name>`)
to force a refresh.

## Networking

The default per-instance NetworkPolicy allows egress on TCP/443, so an HTTPS
broker on port 443 is reachable out of the box. A broker on another port, or
an instance whose egress is restricted, needs
`spec.security.networkPolicy.additionalEgress` or `allowedEgressCIDRs` for the
broker endpoint.

## Operator image

The resolver is copied out of the running operator image, which the operator
learns from its `HERMES_OPERATOR_IMAGE` env var. The Helm chart and the OLM
bundle set it. When deploying from `config/` with kustomize, set
`HERMES_OPERATOR_IMAGE` on the manager container, or set
`spec.secretsBroker.resolverImage` per instance. Without either, the
`init-secrets-broker` container fails with a message saying so, rather than
starting the agent without its secrets.
