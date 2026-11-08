# Upgrade Notes

Per-release notes for changes that alter the behaviour of **existing**
instances, as opposed to adding opt-in surface. Read the entry for every
version you are skipping over, not just the one you are moving to.

Entries are newest first. Versions not listed here need no action beyond the
normal `helm upgrade` or OLM subscription bump.

## Unreleased

### Images and the Helm chart are published under `ghcr.io/stubbi`

**What changed.** The repository moved from `paperclipinc` to `stubbi`, and
releases after 0.2.0 publish the operator image, the agent image and the Helm
chart under `ghcr.io/stubbi` instead of `ghcr.io/paperclipinc`:

| Artifact | Up to 0.2.0 | From the next release |
|---|---|---|
| Helm chart | `oci://ghcr.io/paperclipinc/charts/hermes-operator` | `oci://ghcr.io/stubbi/charts/hermes-operator` |
| Operator image | `ghcr.io/paperclipinc/hermes-operator` | `ghcr.io/stubbi/hermes-operator` |
| Agent image | `ghcr.io/paperclipinc/hermes-agent` | `ghcr.io/stubbi/hermes-agent` |

The CRD default for `spec.image.repository` (HermesInstance and
HermesClusterDefaults) follows, so an instance that leaves the repository
unset pulls its tag from `ghcr.io/stubbi/hermes-agent` after the upgrade.

**Who is affected.** Helm users must point `helm upgrade` at the new chart
location. Instances that set `spec.image.repository` explicitly keep pulling
from wherever they point; the old `ghcr.io/paperclipinc` images stay
available. Instances that rely on the default repository need their tag to
exist under `ghcr.io/stubbi/hermes-agent` (`v0.16.0` and newer agent images are
published there).

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
