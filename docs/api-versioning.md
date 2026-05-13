# Hermes Operator: API Versioning Policy

> Canonical policy for the `hermes.agent` API group. This document governs every
> change to `HermesInstance`, `HermesSelfConfig`, and `HermesClusterDefaults`
> for the lifetime of v1.x. Pull requests that change CRD types are reviewed
> against this document.

## TL;DR

- **API group:** `hermes.agent`.
- **Served versions in v1.x:** `v1` (storage = hub).
- **No `v1alpha1` / `v1beta1` spoke.** v1 ships as the only version.
- **`hermes.agent/v1` will not have breaking changes for the lifetime of v1.x.**
  A breaking change requires `hermes.agent/v2` plus a conversion webhook and at
  least 6 months of overlap (see §"Breaking changes").
- **New optional fields are non-breaking** and may be added in any minor
  release. Required-field additions are breaking.

## Scope

This policy covers:

- CRD schemas (the OpenAPI under `config/crd/bases/`).
- Status condition types and their reason codes (catalogued in
  `docs/conditions.md`).
- Validating- and defaulting-webhook semantics.
- RBAC verbs requested by the operator's `ClusterRole` (additions are non-
  breaking; removals are breaking: see "RBAC changes" below).

It does **not** cover:

- Operator container image internals (Go package layout, controller
  implementation, log lines). These may change at any time.
- The Helm chart's `values.yaml` schema. The chart has its own semver and its
  own changelog; chart breaking changes are flagged in `CHANGELOG.md` with the
  `chart!:` Conventional Commit prefix.
- Metrics names and labels. These follow Prometheus convention (deprecation
  notice on the metric `# HELP` line for one minor release before rename).

## Versioning surfaces

Three independent versions, all semver:

| Surface | Version | Stability |
|---|---|---|
| API group `hermes.agent` | `v1` | This document. Stable for v1.x. |
| Operator image | `vX.Y.Z` (e.g. `ghcr.io/stubbi/hermes-operator:v1.0.0`) | Semver. Breaking ops-surface changes (RBAC removals, metric renames) require a major bump. |
| Helm chart | `vX.Y.Z` (chart `version`) | Semver of the chart itself, decoupled from `appVersion`. |

`appVersion` in the chart tracks the operator image version. The chart can
release patches independent of the operator image (e.g. fixing a template bug
without rebuilding the operator).

## Non-breaking changes

Allowed at any minor release. Listed exhaustively so reviewers have a checklist.

1. **New optional fields** on any spec, marked `omitempty` and with a default
   that preserves prior behaviour. Example: adding
   `spec.observability.tracing.enabled` defaulting to `false`.
2. **New conditions** appended to `docs/conditions.md`. Consumers MUST treat
   unknown condition types as informational.
3. **New reason codes** for an existing condition, as long as the prior reason
   codes remain valid for their original triggers.
4. **New status sub-trees** (e.g. a new `status.tracing` object) populated
   alongside existing status fields.
5. **New printer columns** added to existing CRDs.
6. **New RBAC verbs** added to the operator's `ClusterRole`. The webhook never
   relies on permissions the operator does not hold, so widening is safe.
7. **New webhook warnings** (`admission.Warnings`). Warnings are advisory and
   never block a request.
8. **New defaults supplied by `HermesClusterDefaults`**, as long as the
   defaulter still only fills `nil` fields.
9. **New CRDs** in the `hermes.agent` group. Adding a CRD does not invalidate
   any existing CR.
10. **Performance improvements** to reconciliation (fewer API calls, smaller
    requeue intervals, additional indexes) that do not change observable state.

## Breaking changes (require v2)

The following changes are breaking and may only land via a new served version
`hermes.agent/v2`. See "v2 plumbing" below for the conversion-webhook plan.

1. **Field removal** from any spec. Removed fields must first be deprecated
   per `docs/deprecations.md`, kept for at least 2 minor releases AND 6 months,
   and then removed only in v2.
2. **Semantic change** to an existing field. Example: changing the default
   of `spec.security.rootFilesystemReadOnly` from `true` to `false` is
   breaking even though the field name and type are unchanged.
3. **Required-field addition.** Adding a field with no default that the
   reconciler dereferences is breaking even if the OpenAPI marks it
   `optional`: if the operator panics on `nil`, that's a break.
4. **Type change** on any field (string → enum, int → string, etc.).
5. **Validation tightening** that would reject an instance that previously
   validated. Loosening validation is non-breaking.
6. **Condition removal.** Removing a condition type that previously was set
   is breaking because dashboards key off it.
7. **Condition semantic change**: changing the meaning of a reason code
   without renaming it.
8. **RBAC verb removal.** A v2 operator may need fewer permissions; the
   chart's `ClusterRole` shrinking will break GitOps workflows that pin
   the role.
9. **Finalizer rename.** Existing CRs may carry the old finalizer; rename
   requires a migration path.
10. **CRD short-name removal** (`hi`, `hsc`, `hcd`). Users have these in
    scripts.

## v2 plumbing (in place at v1.0)

To make a future v2 cheap, v1.0 ships with conversion-webhook scaffolding
**even though there are no spokes yet**. Concretely:

- `api/v1/hermesinstance_types.go` declares `HermesInstance` with the
  `+kubebuilder:storageversion` marker. `v1` is both hub and storage.
- `config/crd/patches/cainjection_in_hermes*.yaml` and
  `config/crd/patches/webhook_in_hermes*.yaml` are committed. They are no-ops
  while there is only one version, but the kustomize overlay includes them so
  conversion plumbing is one PR away from being live.
- `cmd/manager/main.go` already registers `(&HermesInstance{}).SetupWebhookWithManager`
  with both defaulter and validator. A future conversion implementation
  attaches to the same builder.
- `Makefile` target `make conversion-stub` exists and produces a skeleton
  `api/v2/conversion.go` whenever a v2 is introduced.

When v2 lands:

1. `api/v2/` is added with the v2 types. `v2` is marked
   `+kubebuilder:storageversion`; `v1` keeps `+kubebuilder:served=true` but
   loses `storageversion`.
2. `api/v1/hermesinstance_conversion.go` implements
   `ConvertTo` / `ConvertFrom` against `v2`.
3. The conversion webhook is enabled in the kustomize overlay.
4. Both versions must serve in parallel for **≥ 6 months** before v1 can be
   marked `+kubebuilder:served=false`. The 6-month clock starts at the first
   tagged release that serves v2.

## Worked example: a future v1 → v2 rename

Suppose in v1.4 we want to rename `spec.runtime.python` to
`spec.runtime.interpreter.python` (because we are adding `runtime.interpreter.node`
for a future hypothetical TypeScript backend). The migration path is:

1. **v1.4 (additive):** introduce `spec.runtime.interpreter.python` as an
   optional field. The defaulter populates it from `spec.runtime.python` when
   the new field is nil and the old field is set. The validator emits a
   `Warning` if the user provides only the old field. Both fields are honoured.
   This is **non-breaking** because the old field is still served.
2. **v1.4 release notes** include a deprecation entry per
   `docs/deprecations.md`: "`spec.runtime.python` is deprecated, target
   removal v2.0.0, no earlier than v1.4 release date + 6 months."
3. **v1.5, v1.6:** the deprecation persists. The warning becomes stronger if
   adoption is slow (`Warning` becomes `Warning + audit-log Event`).
4. **v2.0 (breaking):** `api/v2/hermesinstance_types.go` removes
   `spec.runtime.python`. The conversion webhook reads v1 CRs with the old
   field set and produces a v2 object with `runtime.interpreter.python`
   populated. The reverse conversion (v2 → v1) writes the legacy field for
   compatibility with v1 clients.
5. **v2.0 + 6 months (or v2.1, whichever is later):** `v1` is marked
   `served=false`. The CRD still exists but `kubectl apply -f` against v1
   returns a clear error pointing at the conversion webhook.
6. **v2.x later:** `v1` is dropped from the CRD entirely. At this point all
   stored objects are v2 (the storage version flipped to v2 at v2.0.0, and
   any objects written since then are v2).

This is the only sanctioned shape for a breaking change. Reviewers reject PRs
that try to "just rename it, it's a minor field": the contract is binding.

## Reading list for reviewers

- Kubernetes API conventions:
  https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- CRD versioning:
  https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/
- Kubebuilder conversion webhooks:
  https://book.kubebuilder.io/multiversion-tutorial/conversion.html

## How this document changes

This policy itself follows the same rules: tightening (e.g. expanding the
list of breaking changes) requires a `docs!:` Conventional Commit, an entry
in `CHANGELOG.md`, and a discussion thread before merge. Loosening (more
things become non-breaking) is a plain `docs:` change.
