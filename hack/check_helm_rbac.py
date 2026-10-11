#!/usr/bin/env python3
"""Verify the Helm chart's manager RBAC against config/rbac/role.yaml.

Rules are compared semantically as (apiGroup, resource, verb) triples, so how
the chart groups rules does not matter. Two render modes are checked:

1. Default (watchNamespaces empty): the ClusterRole grants exactly the
   kubebuilder-generated permissions.
2. Namespaced (watchNamespaces set, #126): the ClusterRole grants only
   cluster-scoped resources, every watched namespace gets a Role with the
   namespaced remainder plus a RoleBinding to the manager ServiceAccount, the
   manager runs with --watch-namespaces, and the namespaced admission webhooks
   are limited to the watched namespaces. In particular nothing grants secrets
   cluster-wide.
"""

import glob
import subprocess
import sys

import yaml

CHART = "charts/hermes-operator"
ROLE = "config/rbac/role.yaml"
WATCHED = ["team-a", "team-b"]
NAMESPACED_WEBHOOKS = {
    "mhermesinstance.hermes.agent",
    "vhermesinstance.hermes.agent",
    "vhermesselfconfig.hermes.agent",
}
# Built-in cluster-scoped resources the manager could plausibly need. CRD scope
# for the operator's own groups is read from config/crd/bases.
BUILTIN_CLUSTER_SCOPED = {
    ("", "namespaces"),
    ("", "nodes"),
    ("", "persistentvolumes"),
    ("apiextensions.k8s.io", "customresourcedefinitions"),
    ("node.k8s.io", "runtimeclasses"),
    ("rbac.authorization.k8s.io", "clusterroles"),
    ("rbac.authorization.k8s.io", "clusterrolebindings"),
    ("storage.k8s.io", "storageclasses"),
}

errors = []


def fail(msg):
    errors.append(msg)


def triples(rules):
    out = set()
    for r in rules or []:
        for g in r.get("apiGroups", []):
            for res in r.get("resources", []):
                for v in r.get("verbs", []):
                    out.add((g, res, v))
    return out


def cluster_scoped():
    scoped = set(BUILTIN_CLUSTER_SCOPED)
    for path in glob.glob("config/crd/bases/*.yaml"):
        with open(path) as f:
            crd = yaml.safe_load(f)
        if crd["spec"].get("scope") == "Cluster":
            scoped.add((crd["spec"]["group"], crd["spec"]["names"]["plural"]))
    return scoped


def is_cluster_scoped(group, resource, scoped):
    return (group, resource.split("/")[0]) in scoped


def render(*extra):
    proc = subprocess.run(
        ["helm", "template", "hermes-operator", CHART, "--namespace", "hermes-system", *extra],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        print(f"::error::helm template {' '.join(extra)} failed:\n{proc.stderr}", file=sys.stderr)
        sys.exit(1)
    out = proc.stdout
    return [d for d in yaml.safe_load_all(out) if d]


def by_kind(docs, kind):
    return [d for d in docs if d.get("kind") == kind]


def fmt(ts):
    return "\n".join(f"    {g or 'core'}/{r}: {v}" for g, r, v in sorted(ts))


def check_default(generated):
    docs = render()
    crs = by_kind(docs, "ClusterRole")
    if len(crs) != 1:
        fail(f"default render: want 1 ClusterRole, got {len(crs)}")
        return
    got = triples(crs[0]["rules"])
    if got != generated:
        fail(
            "default render: ClusterRole drifted from config/rbac/role.yaml\n"
            f"  missing from chart:\n{fmt(generated - got)}\n"
            f"  extra in chart:\n{fmt(got - generated)}"
        )
    if by_kind(docs, "Role") or by_kind(docs, "RoleBinding"):
        fail("default render: no per-namespace Role/RoleBinding expected")


def check_namespaced(generated, scoped):
    docs = render("--set", "watchNamespaces={" + ",".join(WATCHED) + "}")
    want_cluster = {t for t in generated if is_cluster_scoped(t[0], t[1], scoped)}
    want_ns = generated - want_cluster

    crs = by_kind(docs, "ClusterRole")
    cluster = set()
    for cr in crs:
        cluster |= triples(cr["rules"])
    if cluster != want_cluster:
        fail(
            "namespaced render: ClusterRole must grant exactly the cluster-scoped permissions\n"
            f"  missing:\n{fmt(want_cluster - cluster)}\n  extra:\n{fmt(cluster - want_cluster)}"
        )
    if any(r == "secrets" for _, r, _ in cluster):
        fail("namespaced render: secrets must not be granted cluster-wide")

    roles = {d["metadata"].get("namespace"): d for d in by_kind(docs, "Role")}
    bindings = {d["metadata"].get("namespace"): d for d in by_kind(docs, "RoleBinding")}
    for ns in WATCHED:
        role = roles.get(ns)
        if role is None:
            fail(f"namespaced render: no Role in watched namespace {ns}")
        elif triples(role["rules"]) != want_ns:
            got = triples(role["rules"])
            fail(
                f"namespaced render: Role in {ns} drifted from the namespaced part of role.yaml\n"
                f"  missing:\n{fmt(want_ns - got)}\n  extra:\n{fmt(got - want_ns)}"
            )
        rb = bindings.get(ns)
        if rb is None:
            fail(f"namespaced render: no RoleBinding in watched namespace {ns}")
        else:
            subj = rb.get("subjects") or [{}]
            if rb["roleRef"]["kind"] != "Role" or subj[0].get("namespace") != "hermes-system":
                fail(f"namespaced render: RoleBinding in {ns} must bind the Role to the manager ServiceAccount")
    extra_ns = set(roles) - set(WATCHED)
    if extra_ns:
        fail(f"namespaced render: unexpected Roles in {sorted(extra_ns)}")

    args = []
    for deploy in by_kind(docs, "Deployment"):
        for c in deploy["spec"]["template"]["spec"]["containers"]:
            args += c.get("args") or []
    if "--watch-namespaces=" + ",".join(WATCHED) not in args:
        fail(f"namespaced render: manager args lack --watch-namespaces: {args}")

    for kind in ("ValidatingWebhookConfiguration", "MutatingWebhookConfiguration"):
        for cfg in by_kind(docs, kind):
            for wh in cfg["webhooks"]:
                if wh["name"] not in NAMESPACED_WEBHOOKS:
                    continue
                exprs = (wh.get("namespaceSelector") or {}).get("matchExpressions") or []
                ok = any(
                    e.get("key") == "kubernetes.io/metadata.name"
                    and e.get("operator") == "In"
                    and sorted(e.get("values", [])) == sorted(WATCHED)
                    for e in exprs
                )
                if not ok:
                    fail(f"namespaced render: webhook {wh['name']} is not limited to the watched namespaces")


def main():
    with open(ROLE) as f:
        generated = triples(yaml.safe_load(f)["rules"])
    if not generated:
        print(f"::error::{ROLE} has no rules", file=sys.stderr)
        return 1
    check_default(generated)
    check_namespaced(generated, cluster_scoped())
    if errors:
        for e in errors:
            print(f"::error::{e}", file=sys.stderr)
        return 1
    print("Helm chart RBAC matches config/rbac/role.yaml in both default and watchNamespaces modes.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
