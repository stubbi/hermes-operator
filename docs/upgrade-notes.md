# Upgrade Notes

Per-release notes for changes that alter the behaviour of **existing**
instances, as opposed to adding opt-in surface. Read the entry for every
version you are skipping over, not just the one you are moving to.

Entries are newest first. Versions not listed here need no action beyond the
normal `helm upgrade` or OLM subscription bump.

## Unreleased

### Config changes now restart the agent pod

**What changed.** `config.yaml` is subPath-mounted, which kubelet never
refreshes in a running pod, and the pod template carried no digest, so
editing `spec.config` (or a `HermesSelfConfig` patch landing) updated the
ConfigMap while the agent kept running the old config. The pod template now
carries `hermes.agent/config-hash` and `hermes.agent/workspace-hash`
annotations. Any change to the rendered config or the workspace seed rolls
the StatefulSet.

**Action.** Expect one pod restart per instance on upgrade, when the
annotations first appear. After that, config edits take effect on their own;
drop any `kubectl delete pod` step from your workflow.

### `HermesSelfConfig.spec.patchConfig` is now merged into the config ConfigMap

**What changed.** A `patchConfig` used to be written as `selfconfig.yaml` into
the `<instance>-workspace` ConfigMap, where the next `HermesInstance` reconcile
wiped it and nothing read it in the first place. The operator now merges every
`Applied` patch into `<instance>-config` (`config.yaml`) on each instance
reconcile, so the patch survives reconciles and is retracted when the
`HermesSelfConfig` is deleted. `status.appliedFields` reports
`config-configmap.data[key=config.yaml]` instead of the old workspace key.

**Action.** None for most users. If you relied on `selfconfig.yaml` in the
workspace ConfigMap, drop that dependency. The rendered ConfigMap is
subPath-mounted, so restart the pod after a patch lands.

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
