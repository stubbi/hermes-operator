# Operator RBAC and Namespace Scoping

The manager needs broad write access inside every namespace it reconciles: it
creates the agent's StatefulSet, Services, ConfigMaps, Secrets (the generated
API server key), backup CronJobs and Jobs, NetworkPolicies, PDBs, and a
per-instance Role and RoleBinding. Where that access is granted depends on how
the operator is scoped.

## Cluster-wide (default)

With the Helm value `watchNamespaces: []` (the default), the manager watches
every namespace and the chart binds one ClusterRole that grants all of its
permissions cluster-wide. This matches `config/rbac/role.yaml`.

## Namespaced (`watchNamespaces`)

Setting `watchNamespaces` narrows both the watch and the grant:

```yaml
watchNamespaces:
  - team-a
  - team-b
```

- The manager runs with `--watch-namespaces=team-a,team-b`, so its informer
  cache only lists and watches namespaced objects in those namespaces.
- Each listed namespace gets a `Role` plus `RoleBinding` holding the
  namespaced permissions (Secrets, ConfigMaps, StatefulSets, Jobs, and so on).
- The `ClusterRole` keeps only what is genuinely cluster-scoped: read and
  status access to the `HermesClusterDefaults` singleton. Nothing grants
  Secrets cluster-wide.
- The namespaced admission webhooks (`HermesInstance` defaulting and
  validation, `HermesSelfConfig` validation) get a `namespaceSelector` on
  `kubernetes.io/metadata.name`, so objects outside the watched namespaces are
  neither admitted through the operator nor reconciled by it.

The namespaces must exist before `helm install`, because Helm creates the
Roles inside them. Adding a namespace later is a `helm upgrade` with the
extended list.

`hack/check-helm-rbac.sh` renders the chart in both modes and fails CI if the
union of the ClusterRole and the per-namespace Roles drifts from the
kubebuilder-generated role, if any namespaced permission leaks into the
ClusterRole, or if the manager flag or webhook selectors go missing.

### What stays out of scope

The OpenClaw migration (`spec.migration.fromOpenClaw`) reads the source PVC in
the source instance's namespace. If that namespace is not watched, grant the
access separately (see [Migration](../migration.md)).

## OLM

The bundle CSV carries the same split:

- `clusterPermissions`: the cluster-scoped rules only.
- `permissions`: the namespaced rules plus the leader election rules.

OLM turns `permissions` into Roles in each target namespace for
`OwnNamespace` and `SingleNamespace` installs, and into a ClusterRole for
`AllNamespaces` installs. The manager reads its scope from `WATCH_NAMESPACE`,
which the CSV wires to the `olm.targetNamespaces` annotation, so the cache and
the grant always follow the OperatorGroup. `hack/sync-bundle-rbac.sh` keeps
the CSV in sync and the `Bundle RBAC Sync` check enforces it.
