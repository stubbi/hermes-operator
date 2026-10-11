# Upgrade Notes

Per-release notes for changes that alter the behaviour of **existing**
instances, as opposed to adding opt-in surface. Read the entry for every
version you are skipping over, not just the one you are moving to.

Entries are newest first. Versions not listed here need no action beyond the
normal `helm upgrade` or OLM subscription bump.

## Unreleased

### `config.yaml` is now a writable file seeded by an init container

**What changed.** The agent pod gains a `config-seed` init container (agent
image, a few seconds per start). It copies the rendered `config.yaml` from the
instance ConfigMap onto the data volume at `/opt/data/config.yaml` and runs the
agent's config schema migration. The agent container no longer mounts
`config.yaml` read-only from the ConfigMap. See
[runtime](runtime.md#config-file-and-schema-migrations).

**Why.** With the read-only mount, agent config schema migrations failed
silently and the agent ran on an unmigrated config (#160).

**Who is affected.** Every instance: pods roll once on upgrade. Two things to
know:

- A config the agent cannot migrate now stops the pod at init with
  `Ready=False`, reason `ConfigMigrationFailed`, instead of starting on a
  silently wrong config. Set `HERMES_SKIP_CONFIG_MIGRATION=1` in `spec.env` to
  restore the old warn and continue behavior.
- Runtime writes to `config.yaml` (for example `/sethome`) now succeed, but they
  last only until the next pod restart, when the declared config is copied over
  them again. Persist such settings in `spec.config`.

## 0.2.0

### The agent container now has default resource requests and limits

**What changed.** A `HermesInstance` that left `spec.resources` unset used to
render `resources: {}`, so the agent container ran unbounded in the BestEffort
QoS class. The operator now supplies a floor:

```yaml
requests:
  cpu: 500m
  memory: 512Mi
limits:
  cpu: "2"
  memory: 4Gi
```

Requests and limits resolve independently, so an instance that set only one
side keeps it verbatim and picks up the default for the other. A block you set
is used as written, never key-merged.

**Why.** The agent executes model-driven code. Unbounded, a single runaway
instance can starve every other pod on its node, and a BestEffort pod is the
first thing the kubelet evicts under memory pressure.

**Who is affected.** Any instance that left `spec.resources` (or the side in
question) unset. After the upgrade its pods are recreated with the values
above. Two ways that bites:

- An instance whose working set exceeds **4Gi** of memory is now OOM-killed
  where it previously grew freely. The browser stack in the agent image
  (Playwright/Chromium) is the usual reason a real workload gets near this.
- An instance that burst past **2 CPUs** is now throttled.
- On a tight cluster, the new 500m/512Mi requests can leave pods `Pending`
  that previously scheduled anywhere, because BestEffort pods request nothing.

If you rely on a namespace `LimitRange` to supply these values, note that the
operator now wins over the LimitRange default.

**What to do before upgrading.** Check what your instances actually use:

```bash
kubectl top pod -l app.kubernetes.io/name=hermes-agent --all-namespaces
```

For anything close to or above the floor, pick one of:

```yaml
# Set the values you want. An explicit block always wins.
spec:
  resources:
    limits:
      cpu: "4"
      memory: 16Gi
```

```yaml
# Or keep the unset side genuinely unbounded, as before.
spec:
  resources:
    applyOperatorDefaults: false
```

`applyOperatorDefaults` also exists on `HermesClusterDefaults.spec.resources`,
so the opt-out can be applied cluster-wide for the duration of a migration:

```yaml
apiVersion: hermes.agent/v1
kind: HermesClusterDefaults
metadata:
  name: cluster
spec:
  resources:
    applyOperatorDefaults: false
```

Instances that set the flag themselves still win over the cluster default.

See `spec.resources` in [the API reference](api-reference.md#specresources).
