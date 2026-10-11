# Pod security posture

A security review of a rendered `HermesInstance` pod usually flags three
things: the agent container's PID 1 starts as **root**, it keeps the container
runtime's **default capability set**, and its **root filesystem is writable**.
All three are deliberate. This page explains why, what actually runs as root,
where the trust boundary is, and how to tighten the pod when you run an image
that does not need the default runtime's setup.

## Why the agent container starts as root

The operator runs the upstream `nousresearch/hermes-agent` image (see
[Agent runtime](../runtime.md)). That image is supervised by
[s6-overlay](https://github.com/just-containers/s6-overlay). Its `/init`
program has two hard requirements:

1. **It must be PID 1.** `s6-overlay-suexec` aborts with
   `fatal: can only run as pid 1` otherwise. This is why
   `shareProcessNamespace` defaults to `false`: with a shared process
   namespace the pause container becomes PID 1 and `/init` refuses to start.
   s6 reaps zombies itself, so no tini or shared namespace is needed.
2. **It must start as root.** The s6 stage2 hook remaps the in-image `hermes`
   user to `HERMES_UID`/`HERMES_GID` (both set to `1000` by the operator) and
   chowns the `/opt/data` volume to that user. Changing uids and file
   ownership needs root plus `CHOWN`, `SETUID`, `SETGID`, `DAC_OVERRIDE` and
   `FOWNER`.

After that setup phase every supervised service, including the gateway and the
agent itself, is started through `s6-setuidgid` and **runs as uid 1000**. The
root part of the container's life is the s6 bootstrap: `/init`, the stage2
hook, and the supervision tree (`s6-svscan`, `s6-supervise`). Model-driven
code, tool execution and the HTTP API run unprivileged.

You can confirm this on a running instance:

```bash
kubectl exec -n <ns> <instance>-0 -c hermes -- sh -c \
  'for p in /proc/[0-9]*; do echo "$(stat -c %u $p) $(cat $p/comm)"; done'
```

## What the operator does and does not pin

The defaults are built in `internal/resources/statefulset.go`.

| Setting | Default | Why |
|---|---|---|
| `runAsNonRoot` / `runAsUser` | not set | `/init` must start as root (see above). Pinning them makes the pod fail to start. |
| `readOnlyRootFilesystem` | not set (writable) | s6 writes its supervision tree and runtime state under `/run` and edits `/etc/passwd` and `/etc/group` during the uid remap. |
| `capabilities.drop: [ALL]` | not set (runtime default set) | The uid remap and chown need `CHOWN`, `SETUID`, `SETGID`, `DAC_OVERRIDE`, `FOWNER`. No capability is **added**. |
| `allowPrivilegeEscalation` | `false` | Kept. Nothing in the image needs setuid binaries or `no_new_privs` off. |
| `seccompProfile` | `RuntimeDefault` (pod level) | Kept. |
| `fsGroup` | `1000` (pod level) | Gives the dropped uid 1000 group ownership of the PVC. |
| `shareProcessNamespace` | `false` | s6 `/init` must be PID 1. An explicit `spec.shareProcessNamespace: true` is honored but breaks the default image. |
| `privileged`, host namespaces, `hostPath`, `hostPort` | never used | |

The rendered `config.yaml` is mounted read-only from a ConfigMap, and `/tmp`
is an `emptyDir`.

The other containers the operator adds are already fully locked down, because
they do not run s6:

- the optional **Tailscale sidecar** runs with `runAsNonRoot: true`,
  `runAsUser: 1000`, `readOnlyRootFilesystem: true`,
  `allowPrivilegeEscalation: false` and `drop: [ALL]`;
- the **restore** and **migration** init containers and the **backup**,
  **snapshot** and **Honcho** workloads use `readOnlyRootFilesystem: true`,
  `allowPrivilegeEscalation: false` and `drop: [ALL]`.

## Pod Security Standards

With the defaults the agent pod satisfies the Kubernetes
[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
**`baseline`** profile: it is not privileged, uses no host namespaces, host
paths or host ports, adds no capabilities, and does not use an `Unconfined`
seccomp profile.

It does **not** satisfy **`restricted`**, which additionally requires
`runAsNonRoot: true` and `capabilities.drop: [ALL]` on every container. A
namespace labelled `pod-security.kubernetes.io/enforce: restricted` rejects the
pod. Use `baseline` for namespaces that run the default image, optionally with
`warn: restricted` or `audit: restricted` to keep the gap visible:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: agents
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/warn: restricted
```

Note that `readOnlyRootFilesystem` is not part of either profile.

On OpenShift the same reasoning applies to SCCs: the default image is
incompatible with `restricted`/`restricted-v2` and needs an SCC that permits a
root start, for example `anyuid`.

## Where the trust boundary is

Root inside the container is root inside the container's namespaces, with the
runtime default capability set, seccomp `RuntimeDefault` and
`allowPrivilegeEscalation: false`. It is not node root. The meaningful
boundaries around an agent are:

- **The pod.** The agent process (uid 1000) can read and write `/opt/data`
  and `/tmp`, and nothing else persistent. Treat everything on the PVC,
  including conversation history and any credentials the agent writes there,
  as reachable by the model-driven workload.
- **Secrets.** Every Secret you inject through `spec.env`, `spec.envFrom`,
  gateway token or LLM key settings is visible to the agent. Inject only what
  the agent needs, scope API keys on the provider side, and prefer short-lived
  credentials.
- **The Kubernetes API.** The per-instance ServiceAccount has
  `automountServiceAccountToken: false` unless `spec.selfConfigure.enabled` is
  true, and its Role only grants `get`/`watch` on the instance's own
  ConfigMaps and gateway token Secret (plus the self-configuration verbs when
  enabled). See [`spec.security.rbac`](../api-reference.md#specsecurity).
- **The network.** `spec.security.networkPolicy` renders a default-deny
  NetworkPolicy that allows DNS and outbound TCP/443 egress, plus ingress on
  the Service ports from the instance's own namespace. This is what limits
  where a compromised or misbehaving agent can reach. Note that the TCP/443
  baseline is open to any destination today.
- **The node.** For agents that execute untrusted, model-generated code, the
  container boundary alone is weak regardless of uid. Put such instances on a
  sandboxed runtime with `spec.scheduling.runtimeClassName` (gVisor `runsc`,
  Kata Containers).

In short: dropping root inside the container would buy little, because the
workload already runs as uid 1000. The controls that matter are what the agent
can read (secrets, PVC), what it can call (RBAC, NetworkPolicy) and what
isolates it from the node (RuntimeClass).

## Tightening the pod for images that do not need s6

If you run a custom image that starts directly as a non-root user and does not
need the s6 uid remap, you can apply a `restricted` compliant context through
`spec.security.podSecurityContext` and `spec.security.containerSecurityContext`.

Both fields **replace** the operator default wholesale rather than merging
with it, so restate everything you want to keep (`fsGroup`, `seccompProfile`):

```yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: hardened
spec:
  image:
    repository: registry.example.com/my-hermes-agent
    tag: "1.2.3"
  security:
    podSecurityContext:
      runAsNonRoot: true
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
      seccompProfile:
        type: RuntimeDefault
    containerSecurityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
```

Things to check before you do this:

- The image must not use s6-overlay `/init` as its entrypoint, or it fails
  with `s6-overlay-suexec: fatal: can only run as pid 1` or a permission error
  during the uid remap. **The default upstream image does not work with this
  context.**
- With `readOnlyRootFilesystem: true` the agent can still write to `/opt/data`
  (the PVC) and `/tmp` (an `emptyDir`). Any other writable path the image
  needs must be added through `spec.extraVolumes` and `spec.extraVolumeMounts`.
- The pod level `runAsNonRoot: true` also applies to the operator's init
  containers (restore, migration), which then run as uid 1000. They already
  drop all capabilities and use a read-only root filesystem.
- Sidecars and init containers you add through `spec.sidecars` and
  `spec.initContainers` need their own compliant security contexts for the
  namespace to accept the pod under `restricted`.
