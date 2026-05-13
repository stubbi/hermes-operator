# Hermes Operator — Deprecation Policy

> Canonical policy for deprecating fields, conditions, reason codes, RBAC
> verbs, metric names, Helm values, and CLI flags. Paired with
> `docs/api-versioning.md`. Pull requests that deprecate anything are
> reviewed against this document.

## TL;DR

- **Deprecation is a 3-step flow.** A field is not "deprecated" until all
  three steps are done in the same release.
- **Removal lead time is ≥ 2 minor releases AND ≥ 6 months**, whichever is
  longer. A field deprecated in v1.4 (released January) cannot be removed
  before v1.6 *and* not before July. If we ship v1.5 and v1.6 within four
  months, removal still waits for the calendar.
- **Active deprecations live in the table at the bottom of this file.** When
  empty (as it is at v1.0.0), the table header is still present — the next
  contributor appends a row, never invents the format.

## The 3-step deprecation flow

A change is "deprecated" only when all three of these are true in the same
release:

### Step 1 — Godoc + CRD description warning

In the Go type definition:

```go
// SelfImproveLegacyKnob enables the old learning loop. Use
// spec.learning.knobs.* instead.
//
// Deprecated: scheduled for removal in v2.0.0 (no earlier than 2027-01-01).
// See docs/deprecations.md.
// +kubebuilder:validation:Description="DEPRECATED: use spec.learning.knobs.*; scheduled for removal in v2.0.0."
SelfImproveLegacyKnob *bool `json:"selfImproveLegacyKnob,omitempty"`
```

The `// Deprecated:` comment is read by `staticcheck` and surfaces in IDE
tooling. The `+kubebuilder:validation:Description` field is the human-facing
text in `kubectl explain`.

### Step 2 — Webhook warning on use

In `internal/webhook/hermesinstance_validator.go`:

```go
if instance.Spec.SelfImproveLegacyKnob != nil {
    warnings = append(warnings,
        "spec.selfImproveLegacyKnob is deprecated; use spec.learning.knobs.* instead. " +
        "Scheduled for removal in v2.0.0 (no earlier than 2027-01-01).")
}
```

The warning fires on every `kubectl apply`/`create`/`update` that includes
the deprecated field. `kubectl` surfaces these as
`Warning: ...` lines below the apply result. GitOps tools (Argo, Flux) also
surface them in their audit logs.

### Step 3 — CHANGELOG entry + `docs/deprecations.md` row

In `CHANGELOG.md` under the release that ships the deprecation:

```markdown
### Deprecated

- `spec.selfImproveLegacyKnob` — replaced by `spec.learning.knobs.*`.
  Target removal: v2.0.0 (no earlier than 2027-01-01).
  See [docs/deprecations.md](docs/deprecations.md).
```

In `docs/deprecations.md`, append a row to the active-deprecations table
(at the bottom of this file).

If any of the three steps is missing, the deprecation does not count and
reviewers must reject the removal-in-vNext PR that depends on it.

## Removal lead time

The clock starts when the release that introduces all three steps ships.

| Trigger | Minimum lead time |
|---|---|
| Deprecation flow Step 3 entry in `CHANGELOG.md` | 2 minor releases AND 6 calendar months, whichever is longer |
| Major version bump (v1 → v2) | Always allowed; v2 may remove anything deprecated in any v1.x |

We are deliberately conservative. The single dominant failure mode of
operator versioning is GitOps controllers that lag the cluster by a release;
6 months gives even an aggressively-pinned GitOps workflow time to roll
through.

## What counts as a "deprecation"

The deprecation flow applies to every public surface listed in
`docs/api-versioning.md` under "Versioning surfaces", plus:

- **CRD fields and sub-fields** (most common).
- **Condition types.** Removing a condition type that was previously set is
  breaking; deprecating it via the 3-step flow keeps it informational for
  the lead-time window.
- **Condition reason codes.** Renaming a reason code requires deprecating
  the old one.
- **Webhook warnings.** Deprecating a warning means promising not to remove
  it for the lead-time window — useful when scripts grep for it.
- **RBAC verbs in the Helm chart.** Removing a verb the operator no longer
  uses still breaks GitOps workflows that pin the `ClusterRole` diff.
- **Helm `values.yaml` keys.** Renaming a key requires the chart to accept
  both for the lead-time window; the chart raises a `helm.sh/hook` warning
  on use of the old key.
- **Metric names and labels.** Renaming is a deprecation of the old name;
  the `# HELP` line carries the deprecation notice and both names are
  exposed for at least one minor release.
- **CLI flags on `cmd/manager`.** Renaming requires accepting both for the
  lead-time window with the old one emitting a log line.

The deprecation flow does **not** apply to:

- Internal Go package layout. We rearrange `internal/` freely.
- Log line formats (best-effort stable, not contractual).
- Container image internals (base image version, layer ordering).

## Observability of deprecations

We surface deprecations in three places so users see them whether or not
they read the changelog:

1. **`kubectl` warning sink.** Every webhook warning fires on every apply.
   `kubectl` highlights them.
2. **Release notes channel.** GitHub Releases lists deprecations under a
   `### Deprecated` heading. Subscribed users (watching the repo for
   releases) get an email.
3. **GitHub Discussions topic.** Each major deprecation gets a pinned
   discussion thread under the "Announcements" category. This is where
   we collect migration questions and refine the doc.

When a deprecation is **2 minor releases out from removal**, we additionally:

- Add a `panic-on-set` opt-in flag (`HERMES_OPERATOR_FAIL_ON_DEPRECATED=true`)
  that turns warnings into rejections — for GitOps workflows that want to
  fail fast on lagging configs.
- Emit a `Warning` Kubernetes Event on the instance object so
  `kubectl describe hi` shows the deprecation prominently.

## Adding a deprecation — checklist

When you open a PR that deprecates anything, the PR description must include
this checklist (the PR template has it pre-filled):

- [ ] `// Deprecated:` godoc with target version + earliest removal date.
- [ ] `+kubebuilder:validation:Description` updated to start with `DEPRECATED:`.
- [ ] Webhook warning added in the appropriate validator.
- [ ] `CHANGELOG.md` entry under `### Deprecated` (release-please picks this up).
- [ ] Row added to the table in `docs/deprecations.md` (see below).
- [ ] If applicable: GitHub Discussion thread opened under "Announcements".
- [ ] Migration guidance added to `docs/api-reference.md` (the affected field's
      description gets a "Migration:" sub-section).

## Removing a deprecation — checklist

When the lead-time window has elapsed and you open a PR removing the
deprecated surface:

- [ ] Confirm both gates have passed: 2+ minor releases since deprecation AND
      ≥ 6 months wall-clock since deprecation release.
- [ ] Confirm the removal lands in the next major (v2.0+) or, for non-CRD
      surfaces (metrics, log flags), in the next minor with a `feat!:` commit.
- [ ] Move the row from the active table to the "Historical removals"
      table at the bottom of this file.
- [ ] Update `CHANGELOG.md` under `### Removed` with a back-reference to
      the original deprecation release.

## Active deprecations

There are no active deprecations as of v1.0.0. The table header below is
maintained for the next contributor.

| Surface | Type | Deprecated in | Replaced by | Earliest removal | Status |
|---|---|---|---|---|---|
| _(none)_ | — | — | — | — | — |

## Historical removals

| Surface | Type | Deprecated in | Removed in | Notes |
|---|---|---|---|---|
| _(none)_ | — | — | — | — |
