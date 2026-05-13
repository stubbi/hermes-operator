# Hermes Operator: Plan 4: HermesSelfConfig CRD + SSA Reconciler

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the scaffolded `HermesSelfConfig` stub with a full v1 type, implement a Server-Side Apply (SSA) reconciler whose field manager is `hermes.agent/selfconfig`, wire policy enforcement against the parent instance's `selfConfigure` allowlist, and prove via integration tests that FluxCD/Argo can co-own a `HermesInstance` without flap.

**Architecture:** The reconciler never calls `r.Update()` on `HermesInstance`. It constructs a *partial* `HermesInstance` object containing only the fields the SelfConfig touches, then calls `client.Patch(ctx, partial, client.Apply, client.FieldOwner("hermes.agent/selfconfig"))`. Field ownership is recorded by the apiserver; conflicting owners (Flux, Argo, kubectl) keep their fields untouched. `addWorkspaceFiles` lands in the workspace ConfigMap (`<inst>-workspace`); `addProfileSnapshot` is materialised by a one-shot Job that mounts the Honcho PVC. Policy denial is a terminal status with a Kubernetes Event on both the SelfConfig and the parent instance.

**Tech Stack:** Go 1.24, controller-runtime, kubebuilder v4, Ginkgo v2/Gomega, envtest, JSON merge patch (RFC 7396), Prometheus client_golang, `sigs.k8s.io/controller-runtime/pkg/client` SSA helpers, `k8s.io/apimachinery/pkg/util/strategicpatch` (deny-path glob walker only), `gobwas/glob` for protectedKeys matching.

**Prerequisite:** Plans 1-3 merged. Working directory matches their File Structure sections: `internal/controller/metrics.go` exists (Plan 2), `HermesInstance.spec` already has `env`, `skills`, `selfConfigure`, `profileStore` fields with the correct list-type markers (Plan 3), `internal/webhook/hermesselfconfig_validator.go` exists as a stub (Plan 2), and the workspace ConfigMap builder lives at `internal/resources/configmap.go` with at minimum a `workspace.go` companion for nested file keys (Plan 3 task on workspace files).

**Spec reference:** `docs/superpowers/specs/2026-05-12-hermes-operator-design.md` §5 (HermesSelfConfig spec), §7.2 rule 2 (SSA mandate), §7.3 (webhook design: selfconfig validator deny-with-Event), §3 (CRD surface: short name `hsc`, categories `hermes`/`agents`), §10 conformance "GitOps coexistence".

**Plan 1 conventions referenced (do not redefine):**
- `resources.Ptr[T]`, `resources.LabelsForInstance(inst)`, `resources.MergePreservingForeign(existing, desired, "hermes.agent/")`: defined in Plan 1 Task 4.
- The idempotency canary pattern: Plan 1 Task 10 step 3 ("second reconcile does not change generation"). Plan 4 reuses this discipline for the SelfConfig path.
- Commit prefixes: `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `refactor:`, `test:`. Release-please uses `feat:`/`fix:` for the changelog.
- Worktree discipline: `git worktree add ../hermes-operator-plan-4 -b feat/plan-4-selfconfig main` before starting; `git worktree remove` at the end.

---

## File Structure Established by This Plan

```
api/v1/
├── hermesselfconfig_types.go              # REPLACE the scaffolded stub (Tasks 1-4)
├── hermesinstance_types.go                # MODIFY: confirm/strengthen list-type markers (Task 5)
└── zz_generated.deepcopy.go               # regenerated

internal/controller/
├── hermesselfconfig_controller.go         # NEW: Reconcile() entrypoint, SSA-only writes (Task 10)
├── hermesselfconfig_controller_test.go    # NEW: envtest happy/deny/idempotency (Task 13)
├── hermesselfconfig_ssa_test.go           # NEW: GitOps coexistence headline test (Task 19)
├── selfconfig_apply.go                    # NEW: pure SSA-payload builders (Task 7-9, 11-12)
├── selfconfig_apply_test.go               # NEW: unit tests for payload builders (Task 7-9, 11-12)
├── selfconfig_policy.go                   # NEW: allowedActions + protectedKeys evaluation (Task 6)
├── selfconfig_policy_test.go              # NEW: unit tests for policy (Task 6)
├── selfconfig_events.go                   # NEW: event helpers (Task 14)
├── selfconfig_metrics.go                  # NEW: counters (uses metrics.Registry from Plan 2) (Task 15)
├── selfconfig_metrics_test.go             # NEW (Task 15)
└── suite_test.go                          # MODIFY: wire SelfConfig reconciler + Recorder (Task 13)

internal/resources/
├── workspace_configmap.go                 # MODIFY (Plan 3): add nested-path key encoder (Task 8)
├── workspace_configmap_test.go            # MODIFY: add encode/decode unit tests (Task 8)
├── snapshot_job.go                        # NEW: BuildSnapshotJob() builder (Task 11)
└── snapshot_job_test.go                   # NEW (Task 11)

internal/webhook/
├── hermesselfconfig_validator.go          # REPLACE stub with real validator (Task 16)
└── hermesselfconfig_validator_test.go     # NEW: webhook unit tests (Task 16)

cmd/manager/main.go                        # MODIFY: register HermesSelfConfigReconciler (Task 17)

config/rbac/role.yaml                      # regenerated via `make manifests` (Tasks 10, 17)
config/crd/bases/                          # regenerated (Tasks 4, 5)
charts/hermes-operator/templates/crds/     # regenerated via `make sync-chart-crds` (Task 21)

test/conformance/
└── gitops_coexistence_test.go             # NEW: placeholder pointing to Plan 6 (Task 20)

docs/
├── api-reference.md                       # MODIFY: HermesSelfConfig fields + policy table (Task 22)
├── selfconfig.md                          # NEW: SSA contract, deny reasons, worked example (Task 22)
└── conditions.md                          # MODIFY: add Applied/Denied/Pending entries (Task 22)

README.md                                  # MODIFY: feature table row "Self-configure" (Task 22)
```

---

## Task 1: Worktree + branch setup

**Files:** none yet.

- [ ] **Step 1: Create the worktree from main**

```bash
cd /Users/jannesstubbemann/repos/hermes-operator
git fetch origin
git worktree add ../hermes-operator-plan-4 -b feat/plan-4-selfconfig origin/main
cd ../hermes-operator-plan-4
```

Expected: new directory `../hermes-operator-plan-4` with the repo checked out at `feat/plan-4-selfconfig`. Plans 2 and 3 are assumed already merged into `origin/main`.

- [ ] **Step 2: Sanity-check Plans 2 and 3 landed**

```bash
test -f api/v1/hermesselfconfig_types.go         && echo "scaffolded types exist (Plan 1)"
test -f internal/controller/metrics.go           && echo "metrics file exists (Plan 2)"
test -f internal/webhook/hermesselfconfig_validator.go && echo "validator stub exists (Plan 2)"
grep -q "SelfConfigure " api/v1/hermesinstance_types.go && echo "SelfConfigureSpec field exists (Plan 3)"
grep -q "ProfileStore " api/v1/hermesinstance_types.go && echo "ProfileStoreSpec field exists (Plan 3)"
```

Expected: all four lines print. If any are missing, the assumed precondition is wrong: stop and flag the dispatching agent.

- [ ] **Step 3: Confirm we build clean**

```bash
go build ./...
go test ./internal/resources/... -count=1
```

Expected: exit 0 on both. Plan 1's resource unit tests still pass; Plans 2+3 didn't break anything.

- [ ] **Step 4: Note the merge baseline (no commit yet)**

```bash
git log --oneline -1
```

Record the SHA: we'll reference it in the Plan-4 milestone tag at the end of Task 23.

---

## Task 2: Define `HermesSelfConfig` types: top-level + InstanceRef + AddSkills

**Files:**
- Modify: `api/v1/hermesselfconfig_types.go` (replace the scaffolded stub)

The scaffolded file from Plan 1 Task 2 contains a placeholder `Foo string` field. We're replacing it with the v1 shape. Split across Tasks 2-4 so each commit is reviewable.

- [ ] **Step 1: Open the file and back up the file header**

```bash
head -25 api/v1/hermesselfconfig_types.go
```

Expected: license header + package declaration + `import "k8s.io/apimachinery/pkg/apis/meta/v1"`. Keep the license header verbatim; the body below `import` is what we replace.

- [ ] **Step 2: Write the failing test first**

Create `api/v1/hermesselfconfig_types_test.go`:

```go
package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHermesSelfConfig_RootSerialises(t *testing.T) {
	sc := &HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "install-skill", Namespace: "agents"},
		Spec: HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills: []SelfConfigSkill{
				{Source: "git+https://github.com/foo/finance-skill@v1.2.0"},
			},
		},
	}
	assert.Equal(t, "my-hermes", sc.Spec.InstanceRef)
	assert.Len(t, sc.Spec.AddSkills, 1)
	assert.Equal(t, "git+https://github.com/foo/finance-skill@v1.2.0", sc.Spec.AddSkills[0].Source)
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
go test ./api/v1/... -run TestHermesSelfConfig_RootSerialises -v
```

Expected: build error: `HermesSelfConfigSpec` has no `InstanceRef` / `AddSkills` fields (still has the scaffolded `Foo`).

- [ ] **Step 4: Replace `HermesSelfConfigSpec` with the InstanceRef+AddSkills slice**

Replace the existing `type HermesSelfConfigSpec struct { ... }` block in `api/v1/hermesselfconfig_types.go` with:

```go
// HermesSelfConfigSpec is an agent-driven, audited request to mutate the
// parent HermesInstance. The operator validates against the parent's
// .spec.selfConfigure policy, then applies via Server-Side Apply with
// field manager "hermes.agent/selfconfig".
//
// At most one of the AddSkills / PatchConfig / AddEnvVars / AddWorkspaceFiles /
// AddProfileSnapshot fields SHOULD be populated per resource. The validator
// emits a warning (not a denial) when more than one is set; encouraging atomic
// units keeps the audit log readable.
type HermesSelfConfigSpec struct {
	// InstanceRef is the name of the parent HermesInstance in the same namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	InstanceRef string `json:"instanceRef"`

	// AddSkills appends skills to the parent's .spec.skills.
	// Each entry is a uv/pip-compatible source string (e.g.
	// "git+https://github.com/foo/skill@v1.2"). The SSA list-map key is "source".
	// +listType=map
	// +listMapKey=source
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddSkills []SelfConfigSkill `json:"addSkills,omitempty"`
}

// SelfConfigSkill names one skill to install.
type SelfConfigSkill struct {
	// Source is a uv-compatible package specifier. Required.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	Source string `json:"source"`

	// Version optionally pins a version. If both Source and Version specify a
	// version, Source wins; Version is here for human-readable audit trails.
	// +optional
	Version string `json:"version,omitempty"`
}
```

Also delete the scaffolded `Foo string` and its associated `// INSERT ADDITIONAL SPEC FIELDS` comment, if any.

- [ ] **Step 5: Run the test: it should pass**

```bash
go test ./api/v1/... -run TestHermesSelfConfig_RootSerialises -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/v1/hermesselfconfig_types.go api/v1/hermesselfconfig_types_test.go
git commit -m "feat(api): HermesSelfConfig spec: InstanceRef + AddSkills with SSA list-map markers"
```

---

## Task 3: HermesSelfConfig: PatchConfig, AddEnvVars, AddWorkspaceFiles, AddProfileSnapshot

**Files:**
- Modify: `api/v1/hermesselfconfig_types.go`
- Modify: `api/v1/hermesselfconfig_types_test.go`

- [ ] **Step 1: Add the failing test for the remaining four fields**

Append to `api/v1/hermesselfconfig_types_test.go`:

```go
func TestHermesSelfConfig_AllMutationFields(t *testing.T) {
	sc := &HermesSelfConfig{
		Spec: HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			PatchConfig: &apiextensionsv1.JSON{
				Raw: []byte(`{"schedules":{"morning-brief":"0 8 * * *"}}`),
			},
			AddEnvVars: []SelfConfigEnvVar{
				{Name: "FINANCE_TZ", Value: "Europe/Berlin"},
			},
			AddWorkspaceFiles: []SelfConfigWorkspaceFile{
				{Path: "notes/finance.md", Content: "# Finance notes"},
			},
			AddProfileSnapshot: &SelfConfigProfileSnapshot{
				ProfileID: "user-42",
				Data:      "opaque-honcho-payload",
			},
		},
	}
	assert.NotNil(t, sc.Spec.PatchConfig)
	assert.JSONEq(t, `{"schedules":{"morning-brief":"0 8 * * *"}}`, string(sc.Spec.PatchConfig.Raw))
	assert.Equal(t, "Europe/Berlin", sc.Spec.AddEnvVars[0].Value)
	assert.Equal(t, "notes/finance.md", sc.Spec.AddWorkspaceFiles[0].Path)
	assert.Equal(t, "user-42", sc.Spec.AddProfileSnapshot.ProfileID)
}
```

Add the import at the top of the test file:

```go
import (
	// ... existing imports ...
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./api/v1/... -run TestHermesSelfConfig_AllMutationFields -v
```

Expected: undefined `PatchConfig`, `SelfConfigEnvVar`, `SelfConfigWorkspaceFile`, `SelfConfigProfileSnapshot`.

- [ ] **Step 3: Append the remaining fields to `HermesSelfConfigSpec`**

Add inside the existing `HermesSelfConfigSpec` struct, after `AddSkills`:

```go
	// PatchConfig is a JSON merge patch (RFC 7396) applied to the agent's
	// runtime config at ~/.hermes/config.yaml. The operator writes the patch
	// into the workspace ConfigMap under key "selfconfig.yaml"; the agent's
	// init layer merges it into config.yaml at startup. Patches that touch any
	// path in HermesInstance.spec.selfConfigure.protectedKeys are rejected.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PatchConfig *apiextensionsv1.JSON `json:"patchConfig,omitempty"`

	// AddEnvVars appends environment variables to the parent's .spec.env.
	// Plain values or valueFrom references (secret/configMap) are accepted.
	// The SSA list-map key is "name"; an entry with the same name already
	// owned by a different field manager will conflict and be denied unless
	// `hermes.agent/force-ownership: "true"` is set on this SelfConfig.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddEnvVars []SelfConfigEnvVar `json:"addEnvVars,omitempty"`

	// AddWorkspaceFiles writes files into the workspace ConfigMap. Paths may
	// contain "/" (nested directories under ~/.hermes/workspace/). The
	// ConfigMap key is the path with "/" replaced by "__"; the agent's init
	// container decodes the key back to a relative path on the PVC.
	// +listType=map
	// +listMapKey=path
	// +kubebuilder:validation:MaxItems=50
	// +optional
	AddWorkspaceFiles []SelfConfigWorkspaceFile `json:"addWorkspaceFiles,omitempty"`

	// AddProfileSnapshot writes an opaque Honcho profile snapshot onto the
	// Honcho PVC via a one-shot Job. Requires the parent to have
	// .spec.profileStore.honcho.enabled: true. Path layout on the PVC is
	// /data/snapshots/<profileID>/<RFC3339 timestamp>.json.
	// +optional
	AddProfileSnapshot *SelfConfigProfileSnapshot `json:"addProfileSnapshot,omitempty"`
```

Then add the supporting types below `SelfConfigSkill`:

```go
// SelfConfigEnvVar is an environment variable entry. Mirrors core/v1 EnvVar
// but keeps the surface tight: only Value or one of ValueFrom's two sub-refs
// is permitted (no FieldRef / ResourceFieldRef: those leak pod identity).
type SelfConfigEnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`

	// Value is the literal value. Mutually exclusive with ValueFrom.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom selects a value from a Secret or ConfigMap key.
	// +optional
	ValueFrom *SelfConfigEnvVarSource `json:"valueFrom,omitempty"`
}

// SelfConfigEnvVarSource selects a Secret or ConfigMap key as an env var source.
// Exactly one of the two refs must be set.
type SelfConfigEnvVarSource struct {
	// +optional
	SecretKeyRef *SelfConfigKeySelector `json:"secretKeyRef,omitempty"`
	// +optional
	ConfigMapKeyRef *SelfConfigKeySelector `json:"configMapKeyRef,omitempty"`
}

// SelfConfigKeySelector selects a key from a Secret or ConfigMap.
type SelfConfigKeySelector struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// SelfConfigWorkspaceFile is a single file to materialise into the workspace.
type SelfConfigWorkspaceFile struct {
	// Path is the relative path under ~/.hermes/workspace/. Nested directories
	// are encoded as "__" in the ConfigMap key (lesson #482).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._/-]+$`
	Path string `json:"path"`

	// Content is the literal file body. Mutually exclusive with ContentFrom.
	// +optional
	Content string `json:"content,omitempty"`

	// ContentFrom reads the file body from a Secret key (for binary or large content).
	// +optional
	ContentFrom *SelfConfigKeySelector `json:"contentFrom,omitempty"`
}

// SelfConfigProfileSnapshot writes one Honcho profile snapshot via a Job.
type SelfConfigProfileSnapshot struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProfileID string `json:"profileID"`

	// Data is the opaque snapshot payload, base64-encoded or raw JSON,
	// written verbatim to the snapshot file.
	// +kubebuilder:validation:MinLength=1
	Data string `json:"data"`
}
```

Add the new import to `api/v1/hermesselfconfig_types.go` (next to the existing `metav1` import):

```go
import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

- [ ] **Step 4: Run the test: expect PASS**

```bash
go test ./api/v1/... -run TestHermesSelfConfig_AllMutationFields -v
```

Expected: PASS. If `apiextensionsv1` is missing from `go.mod`, run `go mod tidy`.

- [ ] **Step 5: Commit**

```bash
git add api/v1/hermesselfconfig_types.go api/v1/hermesselfconfig_types_test.go
git commit -m "feat(api): HermesSelfConfig spec: patchConfig, envVars, workspace files, profile snapshot"
```

---

## Task 4: HermesSelfConfig: Status + root markers + printer columns

**Files:**
- Modify: `api/v1/hermesselfconfig_types.go`
- Modify: `api/v1/hermesselfconfig_types_test.go`

- [ ] **Step 1: Append the failing status test**

Append to `api/v1/hermesselfconfig_types_test.go`:

```go
func TestHermesSelfConfig_StatusShape(t *testing.T) {
	now := metav1.Now()
	sc := &HermesSelfConfig{
		Status: HermesSelfConfigStatus{
			ObservedGeneration: 7,
			Phase:              SelfConfigPhaseApplied,
			AppliedAt:          &now,
			DenyReason:         "",
			AppliedFields: []string{
				"spec.env[name=FINANCE_TZ]",
				"spec.skills[source=git+https://github.com/foo/finance-skill@v1.2.0]",
			},
			Conditions: []metav1.Condition{{
				Type:               string(SelfConfigConditionApplied),
				Status:             metav1.ConditionTrue,
				LastTransitionTime: now,
				Reason:             "SelfConfigApplied",
				Message:            "applied 2 fields",
			}},
		},
	}
	assert.Equal(t, SelfConfigPhaseApplied, sc.Status.Phase)
	assert.Len(t, sc.Status.AppliedFields, 2)
	assert.Equal(t, int64(7), sc.Status.ObservedGeneration)
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./api/v1/... -run TestHermesSelfConfig_StatusShape -v
```

Expected: undefined `HermesSelfConfigStatus.AppliedFields`, `SelfConfigConditionApplied`, etc.

- [ ] **Step 3: Replace the scaffolded `HermesSelfConfigStatus`**

In `api/v1/hermesselfconfig_types.go`, replace the scaffolded `type HermesSelfConfigStatus struct { ... }` with:

```go
// SelfConfigPhase is a short human-readable status.
// +kubebuilder:validation:Enum=Pending;Applied;Denied
type SelfConfigPhase string

const (
	SelfConfigPhasePending SelfConfigPhase = "Pending"
	SelfConfigPhaseApplied SelfConfigPhase = "Applied"
	SelfConfigPhaseDenied  SelfConfigPhase = "Denied"
)

// SelfConfigConditionType enumerates the conditions a HermesSelfConfig may carry.
type SelfConfigConditionType string

const (
	// SelfConfigConditionApplied is True once SSA succeeded on every requested action.
	SelfConfigConditionApplied SelfConfigConditionType = "Applied"
	// SelfConfigConditionDenied is True when policy or validation rejected the request.
	SelfConfigConditionDenied SelfConfigConditionType = "Denied"
	// SelfConfigConditionPending is True while the controller has accepted but not yet applied.
	SelfConfigConditionPending SelfConfigConditionType = "Pending"
)

// HermesSelfConfigStatus reflects the observed state of a HermesSelfConfig.
type HermesSelfConfigStatus struct {
	// ObservedGeneration is the spec generation the controller last processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase summarises the current state for `kubectl get`. One of
	// Pending, Applied, Denied.
	// +optional
	Phase SelfConfigPhase `json:"phase,omitempty"`

	// AppliedAt is the timestamp of the most recent successful SSA write.
	// +optional
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`

	// DenyReason is populated when Phase=Denied. Empty otherwise.
	// +optional
	DenyReason string `json:"denyReason,omitempty"`

	// AppliedFields lists the dotted paths SSA touched on the parent, e.g.
	// "spec.env[name=FINANCE_TZ]" or "spec.skills[source=git+...]".
	// Intended for `kubectl describe` debugging.
	// +listType=set
	// +optional
	AppliedFields []string `json:"appliedFields,omitempty"`

	// Conditions surface fine-grained state. Standard types: Applied, Denied, Pending.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

- [ ] **Step 4: Update the root markers + struct**

Just above `type HermesSelfConfig struct { ... }` in the same file, set the markers to:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hsc,categories=hermes;agents
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instanceRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="DenyReason",type=string,JSONPath=`.status.denyReason`,priority=1
```

Leave the `HermesSelfConfig`, `HermesSelfConfigList`, and `SchemeBuilder.Register` blocks as scaffolded.

- [ ] **Step 5: Run the status test: expect PASS**

```bash
go test ./api/v1/... -v
```

Expected: all three tests (`RootSerialises`, `AllMutationFields`, `StatusShape`) pass.

- [ ] **Step 6: Regenerate deepcopy + CRD YAML**

```bash
make generate manifests
```

Expected: `api/v1/zz_generated.deepcopy.go` updated; `config/crd/bases/hermes.agent_hermesselfconfigs.yaml` reflects the new shape (printer columns visible).

- [ ] **Step 7: Build the whole tree to catch anything we broke**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 8: Commit**

```bash
git add api/v1/hermesselfconfig_types.go api/v1/hermesselfconfig_types_test.go api/v1/zz_generated.deepcopy.go config/crd/bases/
git commit -m "feat(api): HermesSelfConfig status, conditions, printer columns, short name hsc"
```

---

## Task 5: Confirm HermesInstance list-type markers (Plan 3 forward audit)

**Files:**
- Modify (only if missing): `api/v1/hermesinstance_types.go`

Plan 3 was supposed to land the `Env`, `Skills` slices with `+listType=map`. SSA requires these markers; without them apiserver treats the field as `atomic` and replaces the whole list on every Apply: defeating GitOps coexistence.

- [ ] **Step 1: Grep for the markers**

```bash
grep -B1 'Env \[\]' api/v1/hermesinstance_types.go
grep -B1 'Skills \[\]' api/v1/hermesinstance_types.go
```

Expected: each occurrence is preceded by:
```
// +listType=map
// +listMapKey=name        (for Env)
// +listMapKey=source      (for Skills)
```

- [ ] **Step 2: Write the assertion as a test**

Create `api/v1/hermesinstance_listtypes_test.go`:

```go
package v1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSliceFieldTagsForSSA asserts that fields HermesSelfConfig writes via SSA
// carry the list-type markers required to merge by key instead of replacing.
// Source of truth is the Go struct tags; if these regress, SSA silently
// degrades to "atomic" and GitOps coexistence breaks.
func TestSliceFieldTagsForSSA(t *testing.T) {
	specType := reflect.TypeOf(HermesInstanceSpec{})

	cases := []struct {
		field    string
		listType string
		mapKey   string
	}{
		{"Env", "map", "name"},
		{"Skills", "map", "source"},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			f, ok := specType.FieldByName(c.field)
			require.True(t, ok, "field %s not found on HermesInstanceSpec: Plan 3 missed it", c.field)
			require.Contains(t, string(f.Tag), `listType=`+c.listType, "missing +listType=%s marker on %s", c.listType, c.field)
			require.Contains(t, string(f.Tag), `listMapKey=`+c.mapKey, "missing +listMapKey=%s marker on %s", c.mapKey, c.field)
		})
	}
}
```

Note: kubebuilder markers are *not* in Go struct tags by default: kubebuilder reads them from `// +listType=` doc comments. This test instead asserts via Go tag because Plan 3 should have set the JSON tag with `patchStrategy:"merge" patchMergeKey:"name"`. If Plan 3 used the comment-marker style only, swap the assertion to read CRD YAML: see Step 3.

- [ ] **Step 3: Adjust the test to read CRD YAML (more reliable)**

Replace the body of `TestSliceFieldTagsForSSA` with a CRD-YAML check, which is what the apiserver actually consumes:

```go
package v1

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCRDListTypesForSSA(t *testing.T) {
	body, err := os.ReadFile("../../config/crd/bases/hermes.agent_hermesinstances.yaml")
	require.NoError(t, err, "CRD YAML missing: run `make manifests`")
	s := string(body)

	// We can't parse the full OpenAPI here, so we grep for the key contexts.
	// The CRD generator emits:
	//   env:
	//     items: { ... }
	//     type: array
	//     x-kubernetes-list-map-keys: [name]
	//     x-kubernetes-list-type: map
	require.True(t, strings.Contains(s, "x-kubernetes-list-map-keys:\n            - name"),
		"HermesInstance.spec.env missing list-map-key=name in CRD YAML")
	require.True(t, strings.Contains(s, "x-kubernetes-list-map-keys:\n            - source"),
		"HermesInstance.spec.skills missing list-map-key=source in CRD YAML")
}
```

- [ ] **Step 4: Run the test**

```bash
make manifests
go test ./api/v1/... -run TestCRDListTypesForSSA -v
```

Expected: PASS. If FAIL, Plan 3 missed the markers: fix `api/v1/hermesinstance_types.go` by adding above each slice:

```go
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`
```

…and re-run.

- [ ] **Step 5: Commit (whether we fixed or just added the test)**

```bash
git add api/v1/hermesinstance_listtypes_test.go api/v1/hermesinstance_types.go config/crd/bases/
git commit -m "test(api): assert SSA list-map markers on HermesInstance.spec.env and .spec.skills"
```

---

## Task 6: Policy evaluation: allowedActions + protectedKeys

**Files:**
- Create: `internal/controller/selfconfig_policy.go`
- Create: `internal/controller/selfconfig_policy_test.go`

This is the pure-logic layer the reconciler consults *before* it constructs any SSA patch. No `client.Client` is involved.

- [ ] **Step 1: Write the failing tests**

Create `internal/controller/selfconfig_policy_test.go`:

```go
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestDetermineActions(t *testing.T) {
	t.Run("skills only", func(t *testing.T) {
		sc := &hermesv1.HermesSelfConfig{
			Spec: hermesv1.HermesSelfConfigSpec{
				AddSkills: []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			},
		}
		got := DetermineActions(sc)
		assert.Equal(t, []hermesv1.SelfConfigAction{hermesv1.ActionSkills}, got)
	})
	t.Run("multiple actions", func(t *testing.T) {
		sc := &hermesv1.HermesSelfConfig{Spec: hermesv1.HermesSelfConfigSpec{
			AddSkills:         []hermesv1.SelfConfigSkill{{Source: "x"}},
			AddEnvVars:        []hermesv1.SelfConfigEnvVar{{Name: "X", Value: "y"}},
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{{Path: "a.md", Content: "x"}},
		}}
		got := DetermineActions(sc)
		assert.ElementsMatch(t,
			[]hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionEnvVars, hermesv1.ActionWorkspaceFiles},
			got)
	})
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, DetermineActions(&hermesv1.HermesSelfConfig{}))
	})
}

func TestCheckAllowedActions(t *testing.T) {
	allowed := []hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionConfig}
	t.Run("all allowed", func(t *testing.T) {
		denied := CheckAllowedActions([]hermesv1.SelfConfigAction{hermesv1.ActionConfig}, allowed)
		assert.Empty(t, denied)
	})
	t.Run("some denied", func(t *testing.T) {
		denied := CheckAllowedActions(
			[]hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionEnvVars, hermesv1.ActionProfiles},
			allowed,
		)
		assert.ElementsMatch(t,
			[]hermesv1.SelfConfigAction{hermesv1.ActionEnvVars, hermesv1.ActionProfiles},
			denied)
	})
	t.Run("none allowed = all denied", func(t *testing.T) {
		denied := CheckAllowedActions([]hermesv1.SelfConfigAction{hermesv1.ActionSkills}, nil)
		assert.Equal(t, []hermesv1.SelfConfigAction{hermesv1.ActionSkills}, denied)
	})
}

func TestCheckProtectedPaths(t *testing.T) {
	protected := []string{"provider.apiKey", "*.secret*", "gateways.telegram.token"}
	t.Run("clean patch passes", func(t *testing.T) {
		raw := []byte(`{"schedules":{"morning":"0 8 * * *"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Empty(t, hit)
	})
	t.Run("exact-match protection", func(t *testing.T) {
		raw := []byte(`{"provider":{"apiKey":"sk-xxx"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "provider.apiKey", hit)
	})
	t.Run("glob protection by suffix", func(t *testing.T) {
		raw := []byte(`{"db":{"secretKey":"x"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "db.secretKey", hit, "matched against *.secret*")
	})
	t.Run("nested gateway token", func(t *testing.T) {
		raw := []byte(`{"gateways":{"telegram":{"token":"x"}}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "gateways.telegram.token", hit)
	})
	t.Run("invalid JSON errors", func(t *testing.T) {
		_, err := CheckProtectedPaths([]byte(`{invalid`), protected)
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/controller/... -run TestDetermineActions -v
```

Expected: missing `DetermineActions`, `CheckAllowedActions`, `CheckProtectedPaths`, plus `hermesv1.SelfConfigAction`, `hermesv1.ActionSkills`, etc.

- [ ] **Step 3: Add `SelfConfigAction` constants to `api/v1/hermesselfconfig_types.go`**

Above `HermesSelfConfigSpec`, add:

```go
// SelfConfigAction names a category of mutation. Used by
// HermesInstance.spec.selfConfigure.allowedActions to gate what the agent
// may request via HermesSelfConfig.
// +kubebuilder:validation:Enum=skills;config;envVars;workspaceFiles;profiles
type SelfConfigAction string

const (
	ActionSkills         SelfConfigAction = "skills"
	ActionConfig         SelfConfigAction = "config"
	ActionEnvVars        SelfConfigAction = "envVars"
	ActionWorkspaceFiles SelfConfigAction = "workspaceFiles"
	ActionProfiles       SelfConfigAction = "profiles"
)
```

- [ ] **Step 4: Implement policy evaluation**

Create `internal/controller/selfconfig_policy.go`:

```go
/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gobwas/glob"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// DetermineActions inspects a HermesSelfConfig and returns the set of action
// categories it requests. Used to compare against the parent's allowlist.
func DetermineActions(sc *hermesv1.HermesSelfConfig) []hermesv1.SelfConfigAction {
	var out []hermesv1.SelfConfigAction
	if len(sc.Spec.AddSkills) > 0 {
		out = append(out, hermesv1.ActionSkills)
	}
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		out = append(out, hermesv1.ActionConfig)
	}
	if len(sc.Spec.AddEnvVars) > 0 {
		out = append(out, hermesv1.ActionEnvVars)
	}
	if len(sc.Spec.AddWorkspaceFiles) > 0 {
		out = append(out, hermesv1.ActionWorkspaceFiles)
	}
	if sc.Spec.AddProfileSnapshot != nil {
		out = append(out, hermesv1.ActionProfiles)
	}
	return out
}

// CheckAllowedActions returns the subset of `requested` that is NOT in `allowed`.
// Empty result means everything is permitted.
func CheckAllowedActions(requested, allowed []hermesv1.SelfConfigAction) []hermesv1.SelfConfigAction {
	allowSet := make(map[hermesv1.SelfConfigAction]bool, len(allowed))
	for _, a := range allowed {
		allowSet[a] = true
	}
	var denied []hermesv1.SelfConfigAction
	for _, a := range requested {
		if !allowSet[a] {
			denied = append(denied, a)
		}
	}
	return denied
}

// CheckProtectedPaths walks the JSON merge patch and returns the first dotted
// path that matches any pattern in `protected`. Patterns support glob syntax
// via gobwas/glob (e.g. "*.secret*", "provider.*", "gateways.?.token").
// Returns ("", nil) if no path matches. Returns "", err on JSON parse failure.
func CheckProtectedPaths(patch []byte, protected []string) (string, error) {
	if len(patch) == 0 || len(protected) == 0 {
		return "", nil
	}
	var tree map[string]interface{}
	if err := json.Unmarshal(patch, &tree); err != nil {
		return "", fmt.Errorf("invalid JSON merge patch: %w", err)
	}

	globs := make([]glob.Glob, 0, len(protected))
	for _, p := range protected {
		g, err := glob.Compile(p, '.')
		if err != nil {
			return "", fmt.Errorf("invalid protectedKeys pattern %q: %w", p, err)
		}
		globs = append(globs, g)
	}

	hit := ""
	walk(tree, "", func(path string) bool {
		for _, g := range globs {
			if g.Match(path) {
				hit = path
				return true
			}
		}
		return false
	})
	return hit, nil
}

// walk depth-first calls fn(path) for every leaf and every interior node.
// Returns early when fn returns true.
func walk(v interface{}, prefix string, fn func(string) bool) bool {
	if prefix != "" && fn(prefix) {
		return true
	}
	switch t := v.(type) {
	case map[string]interface{}:
		// Deterministic order helps test assertions.
		keys := sortedKeys(t)
		for _, k := range keys {
			child := joinPath(prefix, k)
			if walk(t[k], child, fn) {
				return true
			}
		}
	}
	return false
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func sortedKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// stable lexicographic
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.Compare(out[j-1], out[j]) > 0; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
```

- [ ] **Step 5: Add `gobwas/glob` to go.mod**

```bash
go get github.com/gobwas/glob@v0.2.5
```

- [ ] **Step 6: Run the tests**

```bash
go test ./internal/controller/... -run 'TestDetermineActions|TestCheckAllowedActions|TestCheckProtectedPaths' -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/controller/selfconfig_policy.go internal/controller/selfconfig_policy_test.go api/v1/hermesselfconfig_types.go api/v1/zz_generated.deepcopy.go go.mod go.sum
git commit -m "feat(controller): selfconfig policy evaluation (actions + protectedKeys glob matching)"
```

---

## Task 7: SSA payload builder: `buildSkillsPatch`

**Files:**
- Create: `internal/controller/selfconfig_apply.go`
- Create: `internal/controller/selfconfig_apply_test.go`

Each "build*Patch" returns a *partial* `HermesInstance` containing only the fields we want to claim ownership of. The reconciler then calls `client.Patch(ctx, partial, client.Apply, ...)`. This is the heart of the SSA pattern.

- [ ] **Step 1: Write the failing test**

Create `internal/controller/selfconfig_apply_test.go`:

```go
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parentInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		TypeMeta:   metav1.TypeMeta{APIVersion: hermesv1.GroupVersion.String(), Kind: "HermesInstance"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-hermes", Namespace: "agents"},
	}
}

func TestBuildSkillsPatch_ContainsOnlySkills(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills: []hermesv1.SelfConfigSkill{
				{Source: "git+https://github.com/foo/skill@v1"},
				{Source: "git+https://github.com/bar/other@v2", Version: "2.0"},
			},
		},
	}
	patch := buildSkillsPatch(parentInstance(), sc)

	// Identity preserved
	assert.Equal(t, "my-hermes", patch.Name)
	assert.Equal(t, "agents", patch.Namespace)
	assert.Equal(t, hermesv1.GroupVersion.String(), patch.APIVersion)
	assert.Equal(t, "HermesInstance", patch.Kind)

	// Only Skills is set: everything else stays zero so SSA does not claim it.
	assert.Len(t, patch.Spec.Skills, 2)
	assert.Equal(t, "git+https://github.com/foo/skill@v1", patch.Spec.Skills[0].Source)
	assert.Empty(t, patch.Spec.Env, "must not touch env when only Skills is requested")
	assert.Empty(t, patch.Spec.Image.Repository, "must not touch image: Flux owns that")
}

func TestBuildEnvVarsPatch_LiteralAndValueFrom(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddEnvVars: []hermesv1.SelfConfigEnvVar{
				{Name: "FINANCE_TZ", Value: "Europe/Berlin"},
				{Name: "API_KEY", ValueFrom: &hermesv1.SelfConfigEnvVarSource{
					SecretKeyRef: &hermesv1.SelfConfigKeySelector{Name: "finance-creds", Key: "apiKey"},
				}},
			},
		},
	}
	patch := buildEnvVarsPatch(parentInstance(), sc)
	assert.Len(t, patch.Spec.Env, 2)
	assert.Equal(t, "FINANCE_TZ", patch.Spec.Env[0].Name)
	assert.Equal(t, "Europe/Berlin", patch.Spec.Env[0].Value)

	assert.Equal(t, "API_KEY", patch.Spec.Env[1].Name)
	require := patch.Spec.Env[1].ValueFrom
	assert.NotNil(t, require)
	assert.NotNil(t, require.SecretKeyRef)
	assert.Equal(t, "finance-creds", require.SecretKeyRef.LocalObjectReference.Name)
	assert.Equal(t, "apiKey", require.SecretKeyRef.Key)
	assert.Empty(t, patch.Spec.Skills, "must not touch skills when only env requested")
	_ = corev1.EnvVar{} // ensure corev1 imported
}

func TestAppliedFieldsFormat(t *testing.T) {
	got := formatAppliedFieldEnv("FINANCE_TZ")
	assert.Equal(t, "spec.env[name=FINANCE_TZ]", got)
	got = formatAppliedFieldSkill("git+https://github.com/foo/skill@v1")
	assert.Equal(t, "spec.skills[source=git+https://github.com/foo/skill@v1]", got)
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/controller/... -run 'TestBuildSkillsPatch|TestBuildEnvVarsPatch|TestAppliedFieldsFormat' -v
```

Expected: `undefined: buildSkillsPatch`, `buildEnvVarsPatch`, `formatAppliedFieldEnv`, `formatAppliedFieldSkill`.

- [ ] **Step 3: Implement the skills + env builders**

Create `internal/controller/selfconfig_apply.go`:

```go
/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// SelfConfigFieldManager is the SSA field manager string the operator uses
// when applying HermesSelfConfig-driven mutations to HermesInstance and to
// the workspace ConfigMap. Any other manager that writes the same path
// produces an SSA conflict, which is exactly what we want: GitOps tools
// keep their fields, this manager keeps its own.
const SelfConfigFieldManager = "hermes.agent/selfconfig"

// ForceOwnershipAnnotation, when set to "true" on a HermesSelfConfig,
// causes the reconciler to call client.Apply with client.ForceOwnership.
// Default behaviour (no annotation, or "false") is collaborative: SSA
// conflicts are surfaced as a Denied status and reported via an Event.
const ForceOwnershipAnnotation = "hermes.agent/force-ownership"

// newPartialInstance returns a HermesInstance carrying only the apiVersion +
// kind + identity fields. Callers populate exactly the spec fields they
// intend to own. SSA semantics: an empty/zero field is NOT claimed.
func newPartialInstance(parent *hermesv1.HermesInstance) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hermesv1.GroupVersion.String(),
			Kind:       "HermesInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      parent.Name,
			Namespace: parent.Namespace,
		},
	}
}

// buildSkillsPatch returns a partial HermesInstance whose .spec.skills holds
// only the entries from sc.Spec.AddSkills. SSA merges these into the existing
// slice by listMapKey=source.
func buildSkillsPatch(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *hermesv1.HermesInstance {
	p := newPartialInstance(parent)
	for _, s := range sc.Spec.AddSkills {
		p.Spec.Skills = append(p.Spec.Skills, hermesv1.InstanceSkill{
			Source:  s.Source,
			Version: s.Version,
		})
	}
	return p
}

// buildEnvVarsPatch returns a partial HermesInstance whose .spec.env holds
// only the entries from sc.Spec.AddEnvVars. SSA merges by listMapKey=name.
func buildEnvVarsPatch(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *hermesv1.HermesInstance {
	p := newPartialInstance(parent)
	for _, ev := range sc.Spec.AddEnvVars {
		out := corev1.EnvVar{Name: ev.Name, Value: ev.Value}
		if ev.ValueFrom != nil {
			out.Value = ""
			vf := &corev1.EnvVarSource{}
			if ev.ValueFrom.SecretKeyRef != nil {
				vf.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.SecretKeyRef.Name},
					Key:                  ev.ValueFrom.SecretKeyRef.Key,
				}
			}
			if ev.ValueFrom.ConfigMapKeyRef != nil {
				vf.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.ConfigMapKeyRef.Name},
					Key:                  ev.ValueFrom.ConfigMapKeyRef.Key,
				}
			}
			out.ValueFrom = vf
		}
		p.Spec.Env = append(p.Spec.Env, out)
	}
	return p
}

func formatAppliedFieldEnv(name string) string     { return fmt.Sprintf("spec.env[name=%s]", name) }
func formatAppliedFieldSkill(source string) string { return fmt.Sprintf("spec.skills[source=%s]", source) }
func formatAppliedFieldFile(path string) string {
	return fmt.Sprintf("workspace-configmap.data[path=%s]", path)
}
```

> **Note for Task 8:** `hermesv1.InstanceSkill` and the `Skills []InstanceSkill` field are assumed already defined by Plan 3. If `go build` fails on `InstanceSkill` undefined, add the following to `api/v1/hermesinstance_types.go` (mirroring `SelfConfigSkill`):
>
> ```go
> // InstanceSkill is one entry of HermesInstance.spec.skills.
> type InstanceSkill struct {
>     // +kubebuilder:validation:MinLength=1
>     Source  string `json:"source"`
>     // +optional
>     Version string `json:"version,omitempty"`
> }
> ```
>
> and ensure `Skills []InstanceSkill `+"`"+`json:"skills,omitempty"`+"`"+`` carries `+listType=map` / `+listMapKey=source`. Re-run `make generate manifests`.

- [ ] **Step 4: Run the test**

```bash
go test ./internal/controller/... -run 'TestBuildSkillsPatch|TestBuildEnvVarsPatch|TestAppliedFieldsFormat' -v
```

Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/selfconfig_apply.go internal/controller/selfconfig_apply_test.go api/v1/hermesinstance_types.go api/v1/zz_generated.deepcopy.go config/crd/bases/
git commit -m "feat(controller): SSA payload builders for skills + envVars (FieldManager=hermes.agent/selfconfig)"
```

---

## Task 8: SSA payload builder: workspace ConfigMap + nested-path encoding

**Files:**
- Modify: `internal/resources/workspace_configmap.go` (Plan 3 wrote this; we add an encoder)
- Modify: `internal/resources/workspace_configmap_test.go`
- Modify: `internal/controller/selfconfig_apply.go`
- Modify: `internal/controller/selfconfig_apply_test.go`

Nested paths (e.g. `notes/finance.md`) cannot be ConfigMap keys directly (`/` is illegal). Lesson #482 from openclaw: replace with `__` for the key, decode at runtime via a tiny init step (a sed in the agent's startup script, or a subPath mount).

- [ ] **Step 1: Write the failing encoder test**

Append to `internal/resources/workspace_configmap_test.go`:

```go
func TestEncodeNestedPath(t *testing.T) {
	cases := []struct{ in, out string }{
		{"flat.md", "flat.md"},
		{"notes/finance.md", "notes__finance.md"},
		{"a/b/c/d.txt", "a__b__c__d.txt"},
		{"deep/path/with-dash.md", "deep__path__with-dash.md"},
	}
	for _, c := range cases {
		assert.Equal(t, c.out, EncodeWorkspacePath(c.in), "in=%s", c.in)
	}
}

func TestDecodeNestedPath(t *testing.T) {
	assert.Equal(t, "notes/finance.md", DecodeWorkspacePath("notes__finance.md"))
	assert.Equal(t, "flat.md", DecodeWorkspacePath("flat.md"))
}

func TestEncodeWorkspacePath_RoundTrip(t *testing.T) {
	for _, p := range []string{"a.md", "a/b.md", "a/b/c/d.md"} {
		assert.Equal(t, p, DecodeWorkspacePath(EncodeWorkspacePath(p)))
	}
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/resources/... -run 'TestEncodeNestedPath|TestDecodeNestedPath|TestEncodeWorkspacePath_RoundTrip' -v
```

Expected: undefined `EncodeWorkspacePath`, `DecodeWorkspacePath`.

- [ ] **Step 3: Add encoders to `internal/resources/workspace_configmap.go`**

Append:

```go
// WorkspacePathSeparator is the in-key escape for "/", since ConfigMap keys
// may not contain slashes. Lesson #482: bidirectional encode/decode keeps
// nested directories representable. The agent's startup hook decodes by
// running `for f in *__*; do mv "$f" "$(echo $f | sed 's|__|/|g')"; done`:
// or equivalently mounts the ConfigMap into a subPath layout.
const WorkspacePathSeparator = "__"

// EncodeWorkspacePath transforms a relative file path into a legal ConfigMap key.
func EncodeWorkspacePath(p string) string {
	return strings.ReplaceAll(p, "/", WorkspacePathSeparator)
}

// DecodeWorkspacePath inverts EncodeWorkspacePath.
func DecodeWorkspacePath(k string) string {
	return strings.ReplaceAll(k, WorkspacePathSeparator, "/")
}
```

Ensure `import "strings"` is present at the top of the file. If `workspace_configmap.go` doesn't exist (Plan 3 was supposed to scaffold it), create it now:

```go
package resources

import (
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceConfigMapName returns the deterministic name of the workspace ConfigMap.
func WorkspaceConfigMapName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-workspace"
}

// EmptyWorkspaceConfigMap returns a labelled, otherwise-empty workspace ConfigMap.
// Plan 3 populates initial files; Plan 4 layers self-config files on top via SSA.
func EmptyWorkspaceConfigMap(inst *hermesv1.HermesInstance) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkspaceConfigMapName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Data: map[string]string{},
	}
}
```

(If Plan 3 already created a richer builder, leave it intact and just add the encode/decode helpers.)

- [ ] **Step 4: Run encoder tests: expect PASS**

```bash
go test ./internal/resources/... -run 'TestEncode|TestDecode' -v
```

- [ ] **Step 5: Write the workspace SSA-payload test**

Append to `internal/controller/selfconfig_apply_test.go`:

```go
func TestBuildWorkspaceFilesPatch_NestedPaths(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{
				{Path: "notes/finance.md", Content: "# Finance"},
				{Path: "flat.md", Content: "hello"},
			},
		},
	}
	cm := buildWorkspaceFilesPatch(parent, sc)
	assert.Equal(t, "my-hermes-workspace", cm.Name)
	assert.Equal(t, "agents", cm.Namespace)
	assert.Equal(t, "# Finance", cm.Data["notes__finance.md"])
	assert.Equal(t, "hello", cm.Data["flat.md"])
	assert.Equal(t, "v1", cm.APIVersion)
	assert.Equal(t, "ConfigMap", cm.Kind)
}
```

- [ ] **Step 6: Implement `buildWorkspaceFilesPatch`**

Append to `internal/controller/selfconfig_apply.go`:

```go
import (
	"github.com/stubbi/hermes-operator/internal/resources"
	corev1 "k8s.io/api/core/v1"
)

// buildWorkspaceFilesPatch returns a partial ConfigMap (workspace) whose Data
// holds only the keys we want to claim ownership of via SSA. Nested paths
// are encoded with "/" -> "__" (lesson #482).
func buildWorkspaceFilesPatch(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      resources.WorkspaceConfigMapName(parent),
			Namespace: parent.Namespace,
		},
		Data: map[string]string{},
	}
	for _, f := range sc.Spec.AddWorkspaceFiles {
		// We only encode Content here; ContentFrom is handled at apply time
		// by resolving the secret/configmap key and stuffing it into Data.
		// For now (literal content only), set the key.
		if f.Content != "" {
			cm.Data[resources.EncodeWorkspacePath(f.Path)] = f.Content
		}
	}
	return cm
}
```

Make sure the existing `import` block in `selfconfig_apply.go` is augmented (don't add a second block: merge).

- [ ] **Step 7: Run the workspace-payload test**

```bash
go test ./internal/controller/... -run TestBuildWorkspaceFilesPatch_NestedPaths -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/resources/workspace_configmap.go internal/resources/workspace_configmap_test.go internal/controller/selfconfig_apply.go internal/controller/selfconfig_apply_test.go
git commit -m "feat(resources,controller): workspace-path encode/decode + SSA payload for addWorkspaceFiles"
```

---

## Task 9: SSA payload builder: patchConfig into the workspace ConfigMap

**Files:**
- Modify: `internal/controller/selfconfig_apply.go`
- Modify: `internal/controller/selfconfig_apply_test.go`

`patchConfig` is a JSON merge patch destined for `~/.hermes/config.yaml`. We don't apply it to the user's `HermesInstance` (that would conflict with GitOps), and we don't mutate the user's primary config ConfigMap (Plan 2 created that one). Instead we drop the patch as a separate file under the workspace ConfigMap key `selfconfig.yaml`: the agent's startup layer merges it onto config.yaml.

- [ ] **Step 1: Write the failing test**

Append to `internal/controller/selfconfig_apply_test.go`:

```go
func TestBuildPatchConfigPayload_WritesSelfConfigYaml(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			PatchConfig: &apiextensionsv1.JSON{
				Raw: []byte(`{"schedules":{"morning-brief":"0 8 * * *"}}`),
			},
		},
	}
	cm := buildPatchConfigPayload(parent, sc)
	assert.Equal(t, "my-hermes-workspace", cm.Name)
	// Stored as YAML for the agent runtime; we accept both YAML and JSON since
	// JSON is valid YAML. The simpler path: store JSON verbatim.
	got := cm.Data["selfconfig.yaml"]
	assert.JSONEq(t, `{"schedules":{"morning-brief":"0 8 * * *"}}`, got)
}

func TestBuildPatchConfigPayload_CombinesWithWorkspaceFiles(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{"x":1}`)},
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{
				{Path: "a.md", Content: "x"},
			},
		},
	}
	cm := mergeConfigMapPatches(
		buildPatchConfigPayload(parent, sc),
		buildWorkspaceFilesPatch(parent, sc),
	)
	assert.Equal(t, `{"x":1}`, cm.Data["selfconfig.yaml"])
	assert.Equal(t, "x", cm.Data["a.md"])
}
```

Add the import at the top:

```go
import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/controller/... -run 'TestBuildPatchConfigPayload|TestBuildPatchConfigPayload_CombinesWithWorkspaceFiles' -v
```

Expected: undefined `buildPatchConfigPayload`, `mergeConfigMapPatches`.

- [ ] **Step 3: Implement both**

Append to `internal/controller/selfconfig_apply.go`:

```go
// buildPatchConfigPayload turns a patchConfig into a partial workspace
// ConfigMap with key "selfconfig.yaml". The hermes-agent runtime merges
// this on top of ~/.hermes/config.yaml at startup (hermes-agent supports
// config layering natively).
func buildPatchConfigPayload(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      resources.WorkspaceConfigMapName(parent),
			Namespace: parent.Namespace,
		},
		Data: map[string]string{},
	}
	if sc.Spec.PatchConfig == nil || len(sc.Spec.PatchConfig.Raw) == 0 {
		return cm
	}
	// JSON is valid YAML; store the patch verbatim. The agent merges it.
	cm.Data["selfconfig.yaml"] = string(sc.Spec.PatchConfig.Raw)
	return cm
}

// mergeConfigMapPatches combines two partial ConfigMaps of the same name into
// one. Keys from `right` win on collision (last-write semantics on equal-shape
// partials produced by this controller).
func mergeConfigMapPatches(left, right *corev1.ConfigMap) *corev1.ConfigMap {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	out := left.DeepCopy()
	if out.Data == nil {
		out.Data = map[string]string{}
	}
	for k, v := range right.Data {
		out.Data[k] = v
	}
	return out
}
```

- [ ] **Step 4: Run: expect PASS**

```bash
go test ./internal/controller/... -run 'TestBuildPatchConfigPayload|TestBuildPatchConfigPayload_CombinesWithWorkspaceFiles' -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/controller/selfconfig_apply.go internal/controller/selfconfig_apply_test.go
git commit -m "feat(controller): SSA payload for patchConfig (writes selfconfig.yaml into workspace CM)"
```

---

## Task 10: Reconciler skeleton: Reconcile() with SSA mechanics

**Files:**
- Create: `internal/controller/hermesselfconfig_controller.go`

This task lays down the public reconciler with the SSA call pattern, but with deny/apply branches still TODO-coded as `// step N in Task 12`. Task 12 fills in. We do it this way so the SSA mechanics commit (the headline of this plan) is reviewable in isolation.

- [ ] **Step 1: Create the controller file**

Create `internal/controller/hermesselfconfig_controller.go`:

```go
/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesSelfConfigReconciler reconciles HermesSelfConfig resources by
// applying the requested mutations to the parent HermesInstance and/or the
// workspace ConfigMap via Server-Side Apply.
//
// SSA mechanics in this controller:
//
//  1. Never call r.Update(ctx, instance). The HermesInstance reconciler
//     (Plan 1+2) owns the lifecycle of those objects; we only patch fields.
//  2. Every write is `client.Patch(ctx, partial, client.Apply, ...)`
//     with FieldOwner=SelfConfigFieldManager. SSA records ownership per
//     field; other managers (Flux, Argo, kubectl users) keep theirs.
//  3. The partial object contains ONLY the fields we want to own. A
//     zero/empty field is not claimed.
//  4. ForceOwnership is opt-in per HermesSelfConfig via the
//     "hermes.agent/force-ownership: true" annotation. Default is
//     collaborative: conflicts become Denied status entries.
type HermesSelfConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the controller-runtime entrypoint.
func (r *HermesSelfConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("hermesselfconfig", req.NamespacedName)

	var sc hermesv1.HermesSelfConfig
	if err := r.Get(ctx, req.NamespacedName, &sc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Idempotency canary: if we've already processed this generation and
	// the request is in a terminal phase, requeue for cleanup only.
	if sc.Status.ObservedGeneration == sc.Generation &&
		(sc.Status.Phase == hermesv1.SelfConfigPhaseApplied ||
			sc.Status.Phase == hermesv1.SelfConfigPhaseDenied) {
		return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
	}

	// Fetch parent instance for policy lookup.
	parent := &hermesv1.HermesInstance{}
	parentKey := types.NamespacedName{Name: sc.Spec.InstanceRef, Namespace: sc.Namespace}
	if err := r.Get(ctx, parentKey, parent); err != nil {
		if apierrors.IsNotFound(err) {
			return r.deny(ctx, &sc, parent, fmt.Sprintf("parent HermesInstance %q not found", sc.Spec.InstanceRef))
		}
		return ctrl.Result{}, err
	}

	// Gate 1: selfConfigure must be enabled on the parent.
	if !boolValue(parent.Spec.SelfConfigure.Enabled) {
		return r.deny(ctx, &sc, parent, "selfconfig disabled on parent")
	}

	// Gate 2: every requested action must be in allowedActions.
	requested := DetermineActions(&sc)
	if len(requested) == 0 {
		return r.deny(ctx, &sc, parent, "request contains no mutations")
	}
	if denied := CheckAllowedActions(requested, parent.Spec.SelfConfigure.AllowedActions); len(denied) > 0 {
		msg := fmt.Sprintf("actions %v not in allowedActions=%v", denied, parent.Spec.SelfConfigure.AllowedActions)
		return r.deny(ctx, &sc, parent, msg)
	}

	// Gate 3: if patchConfig is set, no path in the patch may match protectedKeys.
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		hit, err := CheckProtectedPaths(sc.Spec.PatchConfig.Raw, parent.Spec.SelfConfigure.ProtectedKeys)
		if err != nil {
			return r.deny(ctx, &sc, parent, fmt.Sprintf("invalid patchConfig: %v", err))
		}
		if hit != "" {
			return r.deny(ctx, &sc, parent, fmt.Sprintf("patchConfig path %q is protected", hit))
		}
	}

	// All gates passed: apply each action via SSA.
	applied, err := r.applyAll(ctx, parent, &sc)
	if err != nil {
		logger.Error(err, "SSA apply failed")
		return r.deny(ctx, &sc, parent, fmt.Sprintf("apply failed: %v", err))
	}

	return r.markApplied(ctx, &sc, parent, applied)
}

// applyAll runs every action category via SSA and returns the list of
// dotted-path field identifiers we touched. Implemented in Task 12.
func (r *HermesSelfConfigReconciler) applyAll(ctx context.Context, parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) ([]string, error) {
	// Stubbed for Task 10; Task 12 fills in.
	return nil, fmt.Errorf("applyAll not yet implemented: Task 12")
}

// patchOptions returns the SSA call options. ForceOwnership is opt-in via
// annotation on the SelfConfig (default: collaborative).
func (r *HermesSelfConfigReconciler) patchOptions(sc *hermesv1.HermesSelfConfig) []client.PatchOption {
	opts := []client.PatchOption{client.FieldOwner(SelfConfigFieldManager)}
	if sc.Annotations[ForceOwnershipAnnotation] == "true" {
		opts = append(opts, client.ForceOwnership)
	}
	return opts
}

// applySSA is the single point through which every SSA write passes.
// Returns ErrSSAConflict when another manager owns a conflicting field
// AND the caller did not opt into ForceOwnership.
func (r *HermesSelfConfigReconciler) applySSA(ctx context.Context, obj client.Object, sc *hermesv1.HermesSelfConfig) error {
	return r.Patch(ctx, obj, client.Apply, r.patchOptions(sc)...)
}

// deny sets the SelfConfig status to Denied, emits Events on both the
// SelfConfig and the parent, and increments the denied counter.
func (r *HermesSelfConfigReconciler) deny(ctx context.Context, sc *hermesv1.HermesSelfConfig, parent *hermesv1.HermesInstance, reason string) (ctrl.Result, error) {
	sc.Status.Phase = hermesv1.SelfConfigPhaseDenied
	sc.Status.DenyReason = reason
	sc.Status.ObservedGeneration = sc.Generation
	now := metav1.Now()
	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               string(hermesv1.SelfConfigConditionDenied),
		Status:             metav1.ConditionTrue,
		Reason:             "PolicyViolation",
		Message:            reason,
		LastTransitionTime: now,
	})
	emitSelfConfigEvent(r.Recorder, sc, parent, corev1.EventTypeWarning, EventReasonSelfConfigDenied, reason)
	incSelfConfigDenied(parent, reason)
	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// markApplied transitions to Applied, records appliedFields, and emits Events.
func (r *HermesSelfConfigReconciler) markApplied(ctx context.Context, sc *hermesv1.HermesSelfConfig, parent *hermesv1.HermesInstance, applied []string) (ctrl.Result, error) {
	sc.Status.Phase = hermesv1.SelfConfigPhaseApplied
	sc.Status.DenyReason = ""
	sc.Status.AppliedFields = applied
	now := metav1.Now()
	sc.Status.AppliedAt = &now
	sc.Status.ObservedGeneration = sc.Generation
	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               string(hermesv1.SelfConfigConditionApplied),
		Status:             metav1.ConditionTrue,
		Reason:             "SSASuccess",
		Message:            fmt.Sprintf("applied %d fields", len(applied)),
		LastTransitionTime: now,
	})
	emitSelfConfigEvent(r.Recorder, sc, parent, corev1.EventTypeNormal, EventReasonSelfConfigApplied, fmt.Sprintf("applied %d fields", len(applied)))
	for _, a := range DetermineActions(sc) {
		incSelfConfigApplied(parent, string(a))
	}
	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
}

// SetupWithManager wires watches.
func (r *HermesSelfConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesSelfConfig{}).
		Named("hermesselfconfig").
		Complete(r)
}

func boolValue(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
```

> **Note:** `parent.Spec.SelfConfigure.Enabled` is typed as `*bool` per spec §5. Plan 3's `SelfConfigureSpec` should already use `*bool`; if it's `bool`, drop the `boolValue` helper and access directly. `parent.Spec.SelfConfigure.AllowedActions []SelfConfigAction` and `parent.Spec.SelfConfigure.ProtectedKeys []string` are also assumed defined in Plan 3.

The references `emitSelfConfigEvent`, `EventReasonSelfConfigDenied`, `EventReasonSelfConfigApplied`, `incSelfConfigDenied`, `incSelfConfigApplied` are provided by Tasks 14 + 15.

- [ ] **Step 2: Build to check syntax (will fail until Tasks 14, 15 are done)**

```bash
go build ./...
```

Expected: errors like `undefined: emitSelfConfigEvent`, `undefined: EventReasonSelfConfigDenied`, etc. We'll resolve them in Tasks 14 + 15. **Do not commit yet**: defer the commit to Task 15 so the tree stays buildable.

---

## Task 11: SSA payload builder: addProfileSnapshot Job

**Files:**
- Create: `internal/resources/snapshot_job.go`
- Create: `internal/resources/snapshot_job_test.go`
- Modify: `internal/controller/selfconfig_apply.go`
- Modify: `internal/controller/selfconfig_apply_test.go`

A one-shot Job mounts the same PVC the Honcho Deployment uses (Plan 3 created the Honcho stack) and writes `/data/snapshots/<profileID>/<timestamp>.json`. We don't use SSA for the Job (Jobs are write-once; CreateOrUpdate semantics with a deterministic name is enough).

- [ ] **Step 1: Write the failing Job-builder test**

Create `internal/resources/snapshot_job_test.go`:

```go
package resources

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSnapshotJob_NameAndMounts(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	stamp := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	job := BuildSnapshotJob(inst, "user-42", "snapshot-payload", stamp)
	assert.Equal(t, "demo-snapshot-user-42-20260512080000", job.Name)
	assert.Equal(t, "agents", job.Namespace)

	require := job.Spec.Template.Spec
	assert.Equal(t, corev1.RestartPolicyNever, require.RestartPolicy, "Jobs use RestartPolicyNever")
	assert.Len(t, require.Containers, 1)

	mounts := require.Containers[0].VolumeMounts
	assert.Len(t, mounts, 1)
	assert.Equal(t, "honcho-data", mounts[0].Name)
	assert.Equal(t, "/data", mounts[0].MountPath)

	vols := require.Volumes
	assert.Len(t, vols, 1)
	assert.NotNil(t, vols[0].PersistentVolumeClaim)
	assert.Equal(t, "demo-honcho-data", vols[0].PersistentVolumeClaim.ClaimName)
}

func TestBuildSnapshotJob_HardenedSecurity(t *testing.T) {
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	job := BuildSnapshotJob(inst, "p", "data", time.Now())
	c := job.Spec.Template.Spec.Containers[0]
	assert.NotNil(t, c.SecurityContext)
	assert.True(t, *c.SecurityContext.ReadOnlyRootFilesystem)
	assert.False(t, *c.SecurityContext.AllowPrivilegeEscalation)
	assert.Equal(t, []corev1.Capability{"ALL"}, c.SecurityContext.Capabilities.Drop)
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/resources/... -run TestBuildSnapshotJob -v
```

- [ ] **Step 3: Implement the builder**

Create `internal/resources/snapshot_job.go`:

```go
/*
Copyright 2026 stubbi. Apache-2.0.
*/

package resources

import (
	"fmt"
	"strings"
	"time"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HonchoPVCName returns the deterministic PVC name for the Honcho profile
// store. Plan 3's Honcho builder uses the same expression.
func HonchoPVCName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho-data"
}

// BuildSnapshotJob constructs a one-shot Job that writes a profile snapshot
// to /data/snapshots/<profileID>/<timestamp>.json on the Honcho PVC.
// Name is deterministic: `<inst>-snapshot-<profileID>-<YYYYMMDDHHMMSS>`.
func BuildSnapshotJob(inst *hermesv1.HermesInstance, profileID, data string, when time.Time) *batchv1.Job {
	stamp := when.UTC().Format("20060102150405")
	name := fmt.Sprintf("%s-snapshot-%s-%s", inst.Name, sanitizeProfileID(profileID), stamp)
	labels := LabelsForInstance(inst)
	labels["hermes.agent/component"] = "snapshot"
	labels["hermes.agent/profile-id"] = sanitizeProfileID(profileID)

	rfc3339 := when.UTC().Format(time.RFC3339)
	relPath := fmt.Sprintf("/data/snapshots/%s/%s.json", profileID, rfc3339)
	// Quote-safe shell: single-quote the data and escape any embedded quotes.
	escaped := strings.ReplaceAll(data, "'", `'\''`)
	cmd := fmt.Sprintf(`set -eu; mkdir -p "$(dirname '%s')"; printf '%%s' '%s' > '%s'`, relPath, escaped, relPath)

	one := int32(1)
	ttlSeconds := int32(3600) // 1h

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &one,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: Ptr(int64(30)),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: Ptr(true),
						RunAsUser:    Ptr(int64(1000)),
						RunAsGroup:   Ptr(int64(1000)),
						FSGroup:      Ptr(int64(1000)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Name:                     "writer",
						Image:                    "busybox:1.36",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						Command:                  []string{"/bin/sh", "-c", cmd},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: Ptr(false),
							ReadOnlyRootFilesystem:   Ptr(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "honcho-data", MountPath: "/data"},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "honcho-data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: HonchoPVCName(inst),
							},
						},
					}},
				},
			},
		},
	}
}

// sanitizeProfileID makes a profile ID safe for use in a Job name.
// k8s names allow [a-z0-9-]; we replace everything else with "-".
func sanitizeProfileID(id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run the builder tests**

```bash
go test ./internal/resources/... -run TestBuildSnapshotJob -v
```

Expected: 2 PASS.

- [ ] **Step 5: Reconciler-side hook: `buildProfileSnapshotPayload`**

Append to `internal/controller/selfconfig_apply.go`:

```go
import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
)

// buildProfileSnapshotPayload returns the Job that materialises a Honcho
// profile snapshot. Unlike the HermesInstance / ConfigMap payloads, Jobs are
// not SSA-patched: they are created with a deterministic name; the apiserver
// either creates a new Job or no-ops on AlreadyExists.
func buildProfileSnapshotPayload(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig, when time.Time) *batchv1.Job {
	if sc.Spec.AddProfileSnapshot == nil {
		return nil
	}
	return resources.BuildSnapshotJob(parent, sc.Spec.AddProfileSnapshot.ProfileID, sc.Spec.AddProfileSnapshot.Data, when)
}
```

- [ ] **Step 6: Add the matching selfconfig_apply_test.go test**

Append to `internal/controller/selfconfig_apply_test.go`:

```go
import (
	"time"
)

func TestBuildProfileSnapshotPayload_NilWhenEmpty(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{}
	assert.Nil(t, buildProfileSnapshotPayload(parentInstance(), sc, time.Now()))
}

func TestBuildProfileSnapshotPayload_PopulatesJob(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "user-42",
				Data:      "some-payload",
			},
		},
	}
	job := buildProfileSnapshotPayload(parentInstance(), sc, time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC))
	assert.NotNil(t, job)
	assert.Equal(t, "my-hermes-snapshot-user-42-20260512080000", job.Name)
}
```

- [ ] **Step 7: Run the new tests**

```bash
go test ./internal/controller/... -run TestBuildProfileSnapshotPayload -v
go test ./internal/resources/... -run TestBuildSnapshotJob -v
```

Expected: 4 PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/resources/snapshot_job.go internal/resources/snapshot_job_test.go internal/controller/selfconfig_apply.go internal/controller/selfconfig_apply_test.go
git commit -m "feat(resources,controller): one-shot snapshot Job for addProfileSnapshot"
```

---

## Task 12: Wire `applyAll`: the real SSA loop

**Files:**
- Modify: `internal/controller/hermesselfconfig_controller.go`

Replace the Task-10 stub with the actual sequence of SSA calls + Job creation.

- [ ] **Step 1: Replace the stub `applyAll`**

In `internal/controller/hermesselfconfig_controller.go`, replace:

```go
func (r *HermesSelfConfigReconciler) applyAll(ctx context.Context, parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) ([]string, error) {
	// Stubbed for Task 10; Task 12 fills in.
	return nil, fmt.Errorf("applyAll not yet implemented: Task 12")
}
```

with:

```go
func (r *HermesSelfConfigReconciler) applyAll(ctx context.Context, parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) ([]string, error) {
	var applied []string

	// 1. addSkills: partial HermesInstance, SSA patch.
	if len(sc.Spec.AddSkills) > 0 {
		patch := buildSkillsPatch(parent, sc)
		if err := r.applySSA(ctx, patch, sc); err != nil {
			return applied, fmt.Errorf("skills SSA: %w", err)
		}
		for _, s := range sc.Spec.AddSkills {
			applied = append(applied, formatAppliedFieldSkill(s.Source))
		}
	}

	// 2. addEnvVars: partial HermesInstance, SSA patch.
	if len(sc.Spec.AddEnvVars) > 0 {
		patch := buildEnvVarsPatch(parent, sc)
		if err := r.applySSA(ctx, patch, sc); err != nil {
			return applied, fmt.Errorf("envVars SSA: %w", err)
		}
		for _, ev := range sc.Spec.AddEnvVars {
			applied = append(applied, formatAppliedFieldEnv(ev.Name))
		}
	}

	// 3. patchConfig + addWorkspaceFiles share the workspace ConfigMap.
	//    Merge their partials into one SSA call to avoid generation thrash.
	var cmPatch *corev1.ConfigMap
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		cmPatch = buildPatchConfigPayload(parent, sc)
	}
	if len(sc.Spec.AddWorkspaceFiles) > 0 {
		cmPatch = mergeConfigMapPatches(cmPatch, buildWorkspaceFilesPatch(parent, sc))
	}
	if cmPatch != nil {
		if err := r.applySSA(ctx, cmPatch, sc); err != nil {
			return applied, fmt.Errorf("workspace CM SSA: %w", err)
		}
		if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
			applied = append(applied, "workspace-configmap.data[key=selfconfig.yaml]")
		}
		for _, f := range sc.Spec.AddWorkspaceFiles {
			applied = append(applied, formatAppliedFieldFile(f.Path))
		}
	}

	// 4. addProfileSnapshot: one-shot Job.
	if sc.Spec.AddProfileSnapshot != nil {
		if !boolValue(parent.Spec.ProfileStore.Honcho.Enabled) {
			return applied, fmt.Errorf("addProfileSnapshot requires .spec.profileStore.honcho.enabled=true")
		}
		job := buildProfileSnapshotPayload(parent, sc, time.Now())
		if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) { // reconcile-guard:allow
			return applied, fmt.Errorf("snapshot Job create: %w", err)
		}
		applied = append(applied, fmt.Sprintf("job[name=%s]", job.Name))
	}

	return applied, nil
}
```

- [ ] **Step 2: Sanity-build (still expects Tasks 14+15 to land)**

```bash
go build ./...
```

Expected: still fails on `emitSelfConfigEvent`, `EventReasonSelfConfigDenied`, `incSelfConfigDenied`, `incSelfConfigApplied`. These come next.

- [ ] **Step 3: Stage the diff but do not commit yet**

```bash
git add internal/controller/hermesselfconfig_controller.go
```

We'll commit Tasks 10 + 12 + 14 + 15 together once the tree builds (Task 15 step 6).

---

## Task 13: envtest suite scaffolding for HermesSelfConfig

**Files:**
- Modify: `internal/controller/suite_test.go`
- Create: `internal/controller/hermesselfconfig_controller_test.go`

We register the new reconciler in the envtest suite (Plan 1 set up the suite for HermesInstance) and add a Pending-marker test to confirm the reconciler is wired even before policy/SSA tests land.

- [ ] **Step 1: Patch `suite_test.go` to wire the new reconciler**

Open `internal/controller/suite_test.go`. After the existing `HermesInstanceReconciler` setup block, append:

```go
	err = (&HermesSelfConfigReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("hermes-selfconfig-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
```

(The `record.EventRecorder` is provided by the manager; no extra wiring needed.)

- [ ] **Step 2: Create the smoke test**

Create `internal/controller/hermesselfconfig_controller_test.go`:

```go
package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("HermesSelfConfig controller", func() {
	const (
		ns      = "default"
		timeout = 30 * time.Second
		poll    = 200 * time.Millisecond
	)

	AfterEach(func() {
		ctx := context.Background()
		// Best-effort cleanup: ignore not-found.
		for _, name := range []string{"deny-target", "happy-target"} {
			_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}})
		}
		scs := &hermesv1.HermesSelfConfigList{}
		_ = k8sClient.List(ctx, scs, &client.ListOptions{Namespace: ns})
		for i := range scs.Items {
			_ = k8sClient.Delete(ctx, &scs.Items[i])
		}
	})

	It("denies a SelfConfig whose parent has selfConfigure.enabled=false", func() {
		ctx := context.Background()
		parent := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-target", Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				// SelfConfigure.Enabled left nil/false on purpose.
			},
		}
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())

		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-this", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: "deny-target",
				AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "deny-this", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseDenied))
			g.Expect(got.Status.DenyReason).To(ContainSubstring("selfconfig disabled"))
		}).Within(timeout).WithPolling(poll).Should(Succeed())
	})
})
```

Add the missing import:

```go
import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)
```

- [ ] **Step 3: Don't run tests yet: Tasks 14 + 15 must land first**

The tree still doesn't build. We'll run after Task 15.

- [ ] **Step 4: Stage**

```bash
git add internal/controller/suite_test.go internal/controller/hermesselfconfig_controller_test.go
```

Defer commit to Task 15.

---

## Task 14: Event helpers

**Files:**
- Create: `internal/controller/selfconfig_events.go`

- [ ] **Step 1: Implement the file**

Create `internal/controller/selfconfig_events.go`:

```go
/*
Copyright 2026 stubbi. Apache-2.0.
*/

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// Event reason codes: public for tests and for the conditions documentation.
const (
	EventReasonSelfConfigApplied = "SelfConfigApplied"
	EventReasonSelfConfigDenied  = "SelfConfigDenied"
)

// emitSelfConfigEvent fires a paired event on both the SelfConfig and the
// parent HermesInstance. parent may be nil: for instance-not-found denials:
// in which case only the SelfConfig gets the event.
func emitSelfConfigEvent(
	r record.EventRecorder,
	sc *hermesv1.HermesSelfConfig,
	parent *hermesv1.HermesInstance,
	eventType, reason, message string,
) {
	if r == nil {
		return
	}
	r.Event(sc, eventType, reason, message)
	if parent != nil && parent.Name != "" {
		r.Event(parent, eventType, reason, "selfconfig "+sc.Name+": "+message)
	}
	_ = corev1.EventTypeNormal // keep corev1 import even if unused
}
```

- [ ] **Step 2: Verify the import resolves**

```bash
go build ./internal/controller/...
```

Expected: down to `undefined: incSelfConfigApplied` / `incSelfConfigDenied` from `selfconfig_metrics.go`. Task 15 finishes the build.

- [ ] **Step 3: Stage**

```bash
git add internal/controller/selfconfig_events.go
```

---

## Task 15: Metrics counters

**Files:**
- Create: `internal/controller/selfconfig_metrics.go`
- Create: `internal/controller/selfconfig_metrics_test.go`

Per plan brief: counters are `hermes_selfconfig_applied_total` and `hermes_selfconfig_denied_total`. Plan 2 created `metrics.go` with the `metrics.Registry` registration boilerplate: we register into the same registry.

- [ ] **Step 1: Write the failing metrics test**

Create `internal/controller/selfconfig_metrics_test.go`:

```go
package controller

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIncSelfConfigApplied(t *testing.T) {
	selfConfigAppliedTotal.Reset()
	parent := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	incSelfConfigApplied(parent, "envVars")
	incSelfConfigApplied(parent, "envVars")
	incSelfConfigApplied(parent, "skills")

	want := `
		# HELP hermes_selfconfig_applied_total Count of HermesSelfConfig requests successfully applied.
		# TYPE hermes_selfconfig_applied_total counter
		hermes_selfconfig_applied_total{action="envVars",instance="x",namespace="y"} 2
		hermes_selfconfig_applied_total{action="skills",instance="x",namespace="y"} 1
	`
	assert.NoError(t, testutil.CollectAndCompare(selfConfigAppliedTotal, strings.NewReader(want)))
}

func TestIncSelfConfigDenied(t *testing.T) {
	selfConfigDeniedTotal.Reset()
	parent := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	incSelfConfigDenied(parent, "selfconfig disabled on parent")
	incSelfConfigDenied(parent, "selfconfig disabled on parent")
	want := `
		# HELP hermes_selfconfig_denied_total Count of HermesSelfConfig requests denied by policy or validation.
		# TYPE hermes_selfconfig_denied_total counter
		hermes_selfconfig_denied_total{instance="x",namespace="y",reason="selfconfig disabled on parent"} 2
	`
	assert.NoError(t, testutil.CollectAndCompare(selfConfigDeniedTotal, strings.NewReader(want)))
}
```

- [ ] **Step 2: Run: expect compile failure**

```bash
go test ./internal/controller/... -run TestIncSelfConfig -v
```

- [ ] **Step 3: Implement the metrics file**

Create `internal/controller/selfconfig_metrics.go`:

```go
/*
Copyright 2026 stubbi. Apache-2.0.
*/

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	selfConfigAppliedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermes_selfconfig_applied_total",
			Help: "Count of HermesSelfConfig requests successfully applied.",
		},
		[]string{"namespace", "instance", "action"},
	)
	selfConfigDeniedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hermes_selfconfig_denied_total",
			Help: "Count of HermesSelfConfig requests denied by policy or validation.",
		},
		[]string{"namespace", "instance", "reason"},
	)
)

func init() {
	// metrics.Registry is the controller-runtime global registry that Plan 2's
	// metrics.go also uses. Registering here keeps both controller's metrics on
	// the same scrape endpoint.
	metrics.Registry.MustRegister(selfConfigAppliedTotal, selfConfigDeniedTotal)
}

func incSelfConfigApplied(parent *hermesv1.HermesInstance, action string) {
	if parent == nil {
		return
	}
	selfConfigAppliedTotal.WithLabelValues(parent.Namespace, parent.Name, action).Inc()
}

func incSelfConfigDenied(parent *hermesv1.HermesInstance, reason string) {
	ns, name := "", ""
	if parent != nil {
		ns, name = parent.Namespace, parent.Name
	}
	selfConfigDeniedTotal.WithLabelValues(ns, name, reason).Inc()
}
```

- [ ] **Step 4: Run the metrics tests**

```bash
go test ./internal/controller/... -run TestIncSelfConfig -v
```

Expected: 2 PASS.

- [ ] **Step 5: Build the whole tree (now Tasks 10-15 must all compile)**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 6: Run all unit + envtest in the controller package**

```bash
make test
```

Expected: green. The `denies a SelfConfig whose parent has selfConfigure.enabled=false` envtest (Task 13) passes; all policy/apply/metrics unit tests pass.

- [ ] **Step 7: Commit Tasks 10 + 12 + 13 + 14 + 15 together**

```bash
git add internal/controller/hermesselfconfig_controller.go \
        internal/controller/hermesselfconfig_controller_test.go \
        internal/controller/selfconfig_events.go \
        internal/controller/selfconfig_metrics.go \
        internal/controller/selfconfig_metrics_test.go \
        internal/controller/suite_test.go
git commit -m "feat(controller): HermesSelfConfig reconciler with SSA writes (FieldManager=hermes.agent/selfconfig)"
```

---

## Task 16: Real validating webhook for HermesSelfConfig

**Files:**
- Modify: `internal/webhook/hermesselfconfig_validator.go` (Plan 2 left a stub)
- Create: `internal/webhook/hermesselfconfig_validator_test.go`

- [ ] **Step 1: Open the stub**

```bash
cat internal/webhook/hermesselfconfig_validator.go
```

Expected: a `CustomValidator` stub with `ValidateCreate` returning `nil`.

- [ ] **Step 2: Write the failing tests**

Create `internal/webhook/hermesselfconfig_validator_test.go`:

```go
package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newValidator(objs ...runtime.Object) *HermesSelfConfigValidator {
	s := scheme.Scheme
	_ = hermesv1.AddToScheme(s)
	c := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &HermesSelfConfigValidator{Client: c}
}

func parent(name string, profileEnabled bool) *hermesv1.HermesInstance {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
	}
	if profileEnabled {
		t := true
		inst.Spec.ProfileStore.Honcho.Enabled = &t
	}
	return inst
}

func TestValidate_RejectsMissingInstance(t *testing.T) {
	v := newValidator()
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec:       hermesv1.HermesSelfConfigSpec{InstanceRef: "nope"},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instanceRef")
}

func TestValidate_AcceptsValidRequest(t *testing.T) {
	v := newValidator(parent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
	assert.Empty(t, warns)
}

func TestValidate_WarnsOnMultipleMutations(t *testing.T) {
	v := newValidator(parent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "X", Value: "y"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
	require.NotEmpty(t, warns, "must warn: not deny: on multiple mutation fields")
	assert.Contains(t, warns[0], "atomic")
}

func TestValidate_RejectsInvalidJSONPatch(t *testing.T) {
	v := newValidator(parent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{not-json`)},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "patchConfig")
}

func TestValidate_RejectsSnapshotWithoutHoncho(t *testing.T) {
	v := newValidator(parent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "u", Data: "d",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "honcho")
}

func TestValidate_AcceptsSnapshotWithHoncho(t *testing.T) {
	v := newValidator(parent("my-hermes", true))
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "u", Data: "d",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
}
```

- [ ] **Step 3: Replace `internal/webhook/hermesselfconfig_validator.go` body**

Replace the stub's body with:

```go
/*
Copyright 2026 stubbi. Apache-2.0.
*/

package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesselfconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesselfconfigs,verbs=create;update,versions=v1,name=vhermesselfconfig.kb.io,admissionReviewVersions=v1

// HermesSelfConfigValidator validates HermesSelfConfig creates and updates.
type HermesSelfConfigValidator struct {
	client.Client
}

var _ admission.CustomValidator = (*HermesSelfConfigValidator)(nil)

func (v *HermesSelfConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}
func (v *HermesSelfConfigValidator) ValidateUpdate(ctx context.Context, _, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}
func (v *HermesSelfConfigValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *HermesSelfConfigValidator) validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sc, ok := obj.(*hermesv1.HermesSelfConfig)
	if !ok {
		return nil, fmt.Errorf("expected HermesSelfConfig, got %T", obj)
	}

	if sc.Spec.InstanceRef == "" {
		return nil, fmt.Errorf("spec.instanceRef is required")
	}

	parent := &hermesv1.HermesInstance{}
	err := v.Get(ctx, types.NamespacedName{Name: sc.Spec.InstanceRef, Namespace: sc.Namespace}, parent)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("spec.instanceRef %q: no HermesInstance with that name in namespace %q", sc.Spec.InstanceRef, sc.Namespace)
		}
		return nil, fmt.Errorf("loading parent instance: %w", err)
	}

	// patchConfig must be valid JSON.
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		var tmp map[string]interface{}
		if err := json.Unmarshal(sc.Spec.PatchConfig.Raw, &tmp); err != nil {
			return nil, fmt.Errorf("spec.patchConfig is not a valid JSON merge patch: %w", err)
		}
	}

	// addProfileSnapshot requires honcho enabled.
	if sc.Spec.AddProfileSnapshot != nil {
		if parent.Spec.ProfileStore.Honcho.Enabled == nil || !*parent.Spec.ProfileStore.Honcho.Enabled {
			return nil, fmt.Errorf("spec.addProfileSnapshot requires parent .spec.profileStore.honcho.enabled=true")
		}
	}

	// Warn (not deny) when more than one mutation field is populated:
	// atomic SelfConfigs make for a readable audit log.
	mutations := 0
	for _, has := range []bool{
		len(sc.Spec.AddSkills) > 0,
		sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0,
		len(sc.Spec.AddEnvVars) > 0,
		len(sc.Spec.AddWorkspaceFiles) > 0,
		sc.Spec.AddProfileSnapshot != nil,
	} {
		if has {
			mutations++
		}
	}
	if mutations > 1 {
		return admission.Warnings{
			"this HermesSelfConfig requests multiple mutations; consider one mutation per resource for atomic audit trails",
		}, nil
	}
	return nil, nil
}

// SetupWebhookWithManager registers the validator with the manager.
func (v *HermesSelfConfigValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&hermesv1.HermesSelfConfig{}).
		WithValidator(v).
		Complete()
}

// silence unused import in some build configurations
var _ = webhook.AdmissionRequest{}
```

Add the import:

```go
import (
	ctrl "sigs.k8s.io/controller-runtime"
)
```

- [ ] **Step 4: Run the webhook tests**

```bash
go test ./internal/webhook/... -v
```

Expected: 6 PASS.

- [ ] **Step 5: Regenerate webhook config**

```bash
make manifests
```

Expected: `config/webhook/manifests.yaml` updated with the new `vhermesselfconfig.kb.io` webhook.

- [ ] **Step 6: Commit**

```bash
git add internal/webhook/hermesselfconfig_validator.go internal/webhook/hermesselfconfig_validator_test.go config/webhook/
git commit -m "feat(webhook): real validator for HermesSelfConfig (instanceRef, JSON patch, honcho gate)"
```

---

## Task 17: Wire the new reconciler in `cmd/manager/main.go`

**Files:**
- Modify: `cmd/manager/main.go`

- [ ] **Step 1: Locate the existing `HermesInstanceReconciler.SetupWithManager` call**

```bash
grep -n "HermesInstanceReconciler{" cmd/manager/main.go
```

Expected: one match around the existing wiring block (Plan 1 Task 1 / 9).

- [ ] **Step 2: Insert the new reconciler immediately after it**

In `cmd/manager/main.go`, find:

```go
	if err = (&controller.HermesInstanceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HermesInstance")
		os.Exit(1)
	}
```

and append, immediately after:

```go
	if err = (&controller.HermesSelfConfigReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("hermes-selfconfig-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HermesSelfConfig")
		os.Exit(1)
	}
```

- [ ] **Step 3: Register the validating webhook**

Below the reconciler block, after the existing instance-validator setup, append:

```go
	if err = (&webhookpkg.HermesSelfConfigValidator{Client: mgr.GetClient()}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "HermesSelfConfig")
		os.Exit(1)
	}
```

Ensure `webhookpkg "github.com/stubbi/hermes-operator/internal/webhook"` is imported (it may already be: check the import block).

- [ ] **Step 4: Regenerate RBAC + verify build**

```bash
make manifests
go build ./...
```

Expected: exit 0. The `config/rbac/role.yaml` should now grant verbs `get;list;watch;patch` on `hermesinstances` (for the SSA path) and `get;list;watch;create;patch` on `jobs` (for snapshots): both from the kubebuilder markers on `HermesSelfConfigReconciler`.

- [ ] **Step 5: Commit**

```bash
git add cmd/manager/main.go config/rbac/ config/webhook/
git commit -m "feat(manager): register HermesSelfConfigReconciler + validator"
```

---

## Task 18: Idempotency test: second reconcile leaves status untouched

**Files:**
- Modify: `internal/controller/hermesselfconfig_controller_test.go`

The Plan-1 idempotency canary on HermesInstance is the model here. We replicate it for SelfConfig: applying the same generation twice must not flip Applied↔Pending or bump observedGeneration.

- [ ] **Step 1: Add the test**

Append to `internal/controller/hermesselfconfig_controller_test.go` inside the existing `Describe` block:

```go
	It("is idempotent: re-reconciling the same generation does not bump observedGeneration twice", func() {
		ctx := context.Background()
		trueP := true

		parent := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "happy-target", Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				SelfConfigure: hermesv1.SelfConfigureSpec{
					Enabled: &trueP,
					AllowedActions: []hermesv1.SelfConfigAction{
						hermesv1.ActionEnvVars,
					},
					ProtectedKeys: []string{"provider.*"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())

		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "idem-test", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: "happy-target",
				AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		// Wait for Applied.
		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		// Snapshot the AppliedAt timestamp.
		first := &hermesv1.HermesSelfConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, first)).To(Succeed())
		firstApplied := first.Status.AppliedAt

		// Poke an unrelated annotation on the SelfConfig to force re-reconcile.
		Eventually(func() error {
			var cur hermesv1.HermesSelfConfig
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, &cur); err != nil {
				return err
			}
			if cur.Annotations == nil {
				cur.Annotations = map[string]string{}
			}
			cur.Annotations["test.example.com/poke"] = time.Now().String()
			return k8sClient.Update(ctx, &cur)
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		// Allow at least one reconcile to land.
		time.Sleep(2 * time.Second)

		second := &hermesv1.HermesSelfConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, second)).To(Succeed())
		Expect(second.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		Expect(second.Status.AppliedAt.Equal(firstApplied)).To(BeTrue(),
			"AppliedAt must not advance on no-op reconciles: the controller short-circuits via ObservedGeneration")
	})
```

- [ ] **Step 2: Run**

```bash
make test
```

Expected: green. If `AppliedAt` advances, the idempotency check in `Reconcile` (Task 10 Step 1, top of function) is missing: return to that test and confirm `sc.Status.ObservedGeneration == sc.Generation` is the gate.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/hermesselfconfig_controller_test.go
git commit -m "test(controller): idempotency canary for HermesSelfConfig: no AppliedAt bump on no-op reconcile"
```

---

## Task 19: GitOps coexistence: the headline test

**Files:**
- Create: `internal/controller/hermesselfconfig_ssa_test.go`

This is the test that earns the SSA design. We simulate FluxCD writing `.spec.image.tag` and the SelfConfig controller writing `.spec.env`, and verify the two managers don't fight.

- [ ] **Step 1: Create the test file**

Create `internal/controller/hermesselfconfig_ssa_test.go`:

```go
package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("HermesSelfConfig: GitOps coexistence (SSA)", func() {
	const (
		ns      = "default"
		name    = "ssa-target"
		timeout = 45 * time.Second
		poll    = 250 * time.Millisecond
	)

	AfterEach(func() {
		ctx := context.Background()
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}})
		scs := &hermesv1.HermesSelfConfigList{}
		_ = k8sClient.List(ctx, scs, &client.ListOptions{Namespace: ns})
		for i := range scs.Items {
			_ = k8sClient.Delete(ctx, &scs.Items[i])
		}
	})

	It("preserves Flux-owned fields while applying SelfConfig-owned fields", func() {
		ctx := context.Background()
		trueP := true

		// 1. "Flux" applies a HermesInstance with image.tag=v1.0.0
		fluxApplied := &hermesv1.HermesInstance{
			TypeMeta:   metav1.TypeMeta{APIVersion: hermesv1.GroupVersion.String(), Kind: "HermesInstance"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "v1.0.0",
				},
				SelfConfigure: hermesv1.SelfConfigureSpec{
					Enabled: &trueP,
					AllowedActions: []hermesv1.SelfConfigAction{
						hermesv1.ActionEnvVars,
					},
				},
			},
		}
		Expect(k8sClient.Patch(ctx, fluxApplied, client.Apply,
			client.FieldOwner("flux-controller"),
			client.ForceOwnership,
		)).To(Succeed())

		// 2. SelfConfig adds TZ=UTC.
		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "ssa-sc-1", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: name,
				AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		// 3. Wait for Applied.
		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ssa-sc-1", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		// 4. Image.Tag unchanged (Flux still owns it); Env[TZ] present and owned by SelfConfig.
		inst := &hermesv1.HermesInstance{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, inst)).To(Succeed())
		Expect(inst.Spec.Image.Tag).To(Equal("v1.0.0"))
		Expect(inst.Spec.Env).To(ContainElement(corev1.EnvVar{Name: "TZ", Value: "UTC"}))

		fluxOwnsImage := false
		selfconfigOwnsEnv := false
		for _, mf := range inst.ManagedFields {
			if mf.Manager == "flux-controller" && containsString(string(mf.FieldsV1.Raw), `"f:image"`) {
				fluxOwnsImage = true
			}
			if mf.Manager == SelfConfigFieldManager && containsString(string(mf.FieldsV1.Raw), `"f:env"`) {
				selfconfigOwnsEnv = true
			}
		}
		Expect(fluxOwnsImage).To(BeTrue(), "flux-controller must still own spec.image")
		Expect(selfconfigOwnsEnv).To(BeTrue(), "hermes.agent/selfconfig must own spec.env")

		// 5. Flux re-applies with a different tag: env var must survive.
		fluxApplied2 := fluxApplied.DeepCopy()
		fluxApplied2.Spec.Image.Tag = "v1.0.1"
		fluxApplied2.ResourceVersion = ""
		Expect(k8sClient.Patch(ctx, fluxApplied2, client.Apply,
			client.FieldOwner("flux-controller"),
			client.ForceOwnership,
		)).To(Succeed())

		inst2 := &hermesv1.HermesInstance{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, inst2)).To(Succeed())
			g.Expect(inst2.Spec.Image.Tag).To(Equal("v1.0.1"))
			g.Expect(inst2.Spec.Env).To(ContainElement(corev1.EnvVar{Name: "TZ", Value: "UTC"}))
		}).Within(timeout).WithPolling(poll).Should(Succeed(), "env var must survive Flux re-apply: no flap")
	})
})

func containsString(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run**

```bash
make test
```

Expected: green. The two `containsString` matches assert field ownership stayed split. If either fails, run `kubectl get hermesinstance ssa-target -n default -o yaml --show-managed-fields | yq '.metadata.managedFields'` against a real cluster to investigate.

- [ ] **Step 3: Commit**

```bash
git add internal/controller/hermesselfconfig_ssa_test.go
git commit -m "test(controller): GitOps coexistence: Flux + SelfConfig co-own HermesInstance without flap"
```

---

## Task 20: Conformance suite placeholder for Plan 6

**Files:**
- Create: `test/conformance/gitops_coexistence_test.go`

Plan 6 owns the full conformance harness. We leave a buildable placeholder that points at Task 19 so Plan 6 knows where to plug in.

- [ ] **Step 1: Create the placeholder**

Create `test/conformance/gitops_coexistence_test.go`:

```go
/*
Copyright 2026 stubbi. Apache-2.0.

This file is a placeholder for the full Plan-6 conformance suite. Plan 4
already proves SSA-based GitOps coexistence in
`internal/controller/hermesselfconfig_ssa_test.go` against envtest. Plan 6
will re-use the same scenario at higher scale (real kind cluster, multiple
concurrent Flux/SelfConfig writers, latency assertions) and parameterise
across Kubernetes versions 1.28-1.32.

Until Plan 6 lands, this test compiles but is skipped: it's here so a
future Plan-6 engineer can `git grep gitops_coexistence_test` to find the
entry point.
*/

package conformance

import "testing"

func TestGitOpsCoexistenceConformance(t *testing.T) {
	t.Skip("Plan 6 (conformance) wires this; see internal/controller/hermesselfconfig_ssa_test.go for the envtest version")
}
```

- [ ] **Step 2: Build to verify**

```bash
go test ./test/conformance/... -run TestGitOpsCoexistenceConformance -v
```

Expected: SKIP message visible, exit 0.

- [ ] **Step 3: Commit**

```bash
git add test/conformance/gitops_coexistence_test.go
git commit -m "test(conformance): placeholder GitOps coexistence test pointing at Plan 6"
```

---

## Task 21: Sync Helm chart CRDs

**Files:**
- Modify: `charts/hermes-operator/templates/crds/hermes.agent_hermesselfconfigs.yaml` (regenerated)
- Modify: `charts/hermes-operator/templates/crds/hermes.agent_hermesinstances.yaml` (regenerated, if Task 5 mutated it)

- [ ] **Step 1: Regenerate and sync**

```bash
make manifests
make sync-chart-crds
```

Expected: CRD YAML in `charts/hermes-operator/templates/crds/` now contains the full HermesSelfConfig schema (printer columns, `addProfileSnapshot`, etc.).

- [ ] **Step 2: Lint the chart**

```bash
helm lint charts/hermes-operator
```

Expected: exit 0.

- [ ] **Step 3: Run the Helm-RBAC drift check**

```bash
bash hack/check-helm-rbac.sh
```

Expected: exit 0. If it diffs, the chart `ClusterRole` is missing the new verbs added by Task 17 (Jobs, hermesselfconfigs/finalizers). Edit `charts/hermes-operator/templates/clusterrole.yaml` to mirror `config/rbac/role.yaml`:

```yaml
  - apiGroups: [batch]
    resources: [jobs]
    verbs: [get, list, watch, create, patch]
```

Add the `hermesselfconfigs` and `hermesselfconfigs/status` / `hermesselfconfigs/finalizers` entries to the existing `hermes.agent` block:

```yaml
  - apiGroups: [hermes.agent]
    resources: [hermesinstances, hermesselfconfigs, hermesclusterdefaults]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [hermes.agent]
    resources: [hermesinstances/status, hermesselfconfigs/status, hermesclusterdefaults/status]
    verbs: [get, update, patch]
  - apiGroups: [hermes.agent]
    resources: [hermesinstances/finalizers, hermesselfconfigs/finalizers]
    verbs: [update]
```

(Plan 1 Task 13 step 5 already had `hermesselfconfigs` placeholder verbs; Task 17 fills in the real Job verb.)

- [ ] **Step 4: Re-run the check**

```bash
bash hack/check-helm-rbac.sh
```

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add charts/hermes-operator/templates/crds/ charts/hermes-operator/templates/clusterrole.yaml config/rbac/
git commit -m "chore(chart): sync CRDs + ClusterRole verbs for HermesSelfConfig + snapshot Jobs"
```

---

## Task 22: Documentation: api-reference, selfconfig.md, conditions.md, README

**Files:**
- Modify: `docs/api-reference.md`
- Create: `docs/selfconfig.md`
- Modify: `docs/conditions.md`
- Modify: `README.md`

- [ ] **Step 1: Append HermesSelfConfig section to `docs/api-reference.md`**

Append (or create the file if Plan 2 didn't):

```markdown
## HermesSelfConfig

Agent-driven, audited mutation API. Validated against the parent
`HermesInstance.spec.selfConfigure` policy and applied via Server-Side Apply
with field manager `hermes.agent/selfconfig`.

### Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `instanceRef` | string | yes | Name of the parent `HermesInstance` in the same namespace. |
| `addSkills[]` | list | no | uv-compatible source specifiers appended to `HermesInstance.spec.skills`. SSA list-map key: `source`. |
| `addSkills[].source` | string | yes (per item) | e.g. `git+https://github.com/foo/skill@v1.2`. |
| `addSkills[].version` | string | no | Audit-only human label. |
| `patchConfig` | `apiextensions/v1.JSON` | no | JSON merge patch (RFC 7396) written to the workspace ConfigMap key `selfconfig.yaml`. Layered onto `~/.hermes/config.yaml` at agent startup. |
| `addEnvVars[]` | list | no | Environment variables appended to `HermesInstance.spec.env`. SSA list-map key: `name`. |
| `addEnvVars[].name` | string | yes | C-identifier (`^[A-Za-z_][A-Za-z0-9_]*$`). |
| `addEnvVars[].value` | string | no | Literal value. Mutually exclusive with `valueFrom`. |
| `addEnvVars[].valueFrom.secretKeyRef` | object | no | Resolve from `Secret` key. |
| `addEnvVars[].valueFrom.configMapKeyRef` | object | no | Resolve from `ConfigMap` key. |
| `addWorkspaceFiles[]` | list | no | Files materialised under `~/.hermes/workspace/`. Nested paths supported via `/` → `__` ConfigMap-key encoding. |
| `addWorkspaceFiles[].path` | string | yes | Relative path. Pattern `^[A-Za-z0-9._/-]+$`. |
| `addWorkspaceFiles[].content` | string | no | Literal file body. |
| `addWorkspaceFiles[].contentFrom.secretKeyRef` | object | no | Read body from a Secret key. |
| `addProfileSnapshot` | object | no | Write a Honcho profile snapshot. Requires `HermesInstance.spec.profileStore.honcho.enabled=true`. |
| `addProfileSnapshot.profileID` | string | yes | Honcho profile identifier. |
| `addProfileSnapshot.data` | string | yes | Opaque payload written verbatim to `/data/snapshots/<profileID>/<RFC3339>.json`. |

### Status

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | int64 | Spec generation the controller last processed. |
| `phase` | enum | One of `Pending`, `Applied`, `Denied`. |
| `appliedAt` | timestamp | Time of the most recent successful SSA write. |
| `denyReason` | string | Populated when `phase=Denied`. |
| `appliedFields[]` | list of strings | Dotted paths SSA touched, e.g. `spec.env[name=FINANCE_TZ]`. |
| `conditions[]` | `[]metav1.Condition` | Standard k8s conditions. Types: `Applied`, `Denied`, `Pending`. |

### Policy model

Allowed mutations are governed by the parent's `selfConfigure` block:

```yaml
spec:
  selfConfigure:
    enabled: true
    allowedActions: [skills, config, envVars, workspaceFiles, profiles]
    protectedKeys:
      - "provider.apiKey"
      - "*.secret*"
      - "gateways.telegram.token"
```

- `enabled: false` (or unset) → every SelfConfig is `Denied` with reason `selfconfig disabled on parent`.
- `allowedActions` is the closed set of permitted mutation categories. A
  SelfConfig requesting a category not in the list is denied wholesale (no
  partial application).
- `protectedKeys` are glob patterns matched against the dotted JSON path of
  `patchConfig`. The first match denies the SelfConfig. Globs use `.` as
  separator; `*` matches one segment, `**` would match many (gobwas/glob).

### Field-manager contract

The reconciler writes via `client.Apply` with field owner
`hermes.agent/selfconfig`. Concretely:

- It owns only the paths the SelfConfig touches: never `spec.image`,
  `spec.storage`, `spec.gateways`, etc.
- Other field managers (FluxCD, Argo CD, kubectl users) co-own their own
  paths and are not disturbed.
- By default conflicts are surfaced as `Denied`. To force ownership, set
  `metadata.annotations["hermes.agent/force-ownership"]: "true"` on the
  SelfConfig. This is the equivalent of `kubectl apply --force-conflicts`.
```

- [ ] **Step 2: Create `docs/selfconfig.md`**

```markdown
# Self-configuration

The `HermesSelfConfig` CRD is hermes-operator's audited surface for agent-
initiated mutations to its own instance. When the agent learns a new skill,
discovers a useful environment variable, drafts a workspace file, or
persists a Honcho profile snapshot, it creates a `HermesSelfConfig` and lets
the operator validate + apply.

## When to use SelfConfig vs. direct YAML edits

Use **SelfConfig** when:

- A learning / planner loop inside the agent produces the mutation.
- The mutation is small and audit-worthy: you want a Kubernetes Event
  recording who/what/why.
- You want the operator to enforce `selfConfigure.protectedKeys`.

Use **direct YAML edits to `HermesInstance`** when:

- You're the human operator and the change is intentional / reviewed.
- You're using FluxCD/Argo CD to manage the spec: SSA coexistence (see
  below) means SelfConfig won't disturb your declarative fields.

## SSA contract

The reconciler writes via Server-Side Apply with field manager
`hermes.agent/selfconfig`. Concretely:

| Field touched | Field owner after Apply | Why |
|---|---|---|
| `.spec.skills[source=…]` | `hermes.agent/selfconfig` | Listed in `allowedActions=skills`. |
| `.spec.env[name=…]` | `hermes.agent/selfconfig` | Listed in `allowedActions=envVars`. |
| Workspace ConfigMap key | `hermes.agent/selfconfig` | Listed in `allowedActions=workspaceFiles` or `config`. |
| `.spec.image.tag` | (unchanged) | Operator never patches this; Flux/Argo/you keep ownership. |

A conflicting Apply from a different manager produces an SSA conflict.
Default behaviour is **collaborative**: the SelfConfig is denied with
`denyReason: "SSA conflict on <path>: owned by <other-manager>"`. To force
ownership set `hermes.agent/force-ownership: "true"` on the SelfConfig.

## Deny reasons (catalogue)

| Reason | Trigger |
|---|---|
| `selfconfig disabled on parent` | `HermesInstance.spec.selfConfigure.enabled != true`. |
| `actions [X] not in allowedActions=[…]` | The SelfConfig requests an action category not on the allowlist. |
| `patchConfig path "X" is protected` | A path in `patchConfig` matches a `protectedKeys` glob. |
| `invalid patchConfig: …` | `patchConfig.Raw` is not valid JSON. |
| `parent HermesInstance "X" not found` | `instanceRef` doesn't resolve. |
| `request contains no mutations` | Every mutation field is empty. |
| `addProfileSnapshot requires .spec.profileStore.honcho.enabled=true` | The parent has no Honcho store enabled. |
| `apply failed: …` | An SSA call returned an error from the apiserver. |
| `SSA conflict on …` | Another manager owns the path; `force-ownership` not set. |

Each denial fires a Kubernetes Event of type `Warning` with reason
`SelfConfigDenied` on both the SelfConfig and the parent instance, and
increments the Prometheus counter `hermes_selfconfig_denied_total`.

## Worked example

Parent instance:

```yaml
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: my-hermes
  namespace: agents
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: v1.0.0
  selfConfigure:
    enabled: true
    allowedActions: [skills, envVars]
    protectedKeys:
      - "provider.apiKey"
      - "*.secret*"
```

The agent creates:

```yaml
apiVersion: hermes.agent/v1
kind: HermesSelfConfig
metadata:
  name: add-finance-tz
  namespace: agents
spec:
  instanceRef: my-hermes
  addEnvVars:
    - name: FINANCE_TZ
      value: Europe/Berlin
```

After reconciliation:

```bash
$ kubectl describe hsc add-finance-tz -n agents
...
Status:
  Phase:               Applied
  Applied At:          2026-05-12T08:01:00Z
  Observed Generation: 1
  Applied Fields:
    spec.env[name=FINANCE_TZ]
  Conditions:
    Type:    Applied
    Status:  True
    Reason:  SSASuccess
    Message: applied 1 fields
Events:
  Normal  SelfConfigApplied  10s  hermes-selfconfig-controller  applied 1 fields
```

```bash
$ kubectl get hi my-hermes -n agents -o jsonpath='{.spec.env}'
[{"name":"FINANCE_TZ","value":"Europe/Berlin"}]

$ kubectl get hi my-hermes -n agents -o yaml --show-managed-fields | yq '.metadata.managedFields[] | select(.manager=="hermes.agent/selfconfig") | .fieldsV1'
{"f:spec":{"f:env":{"k:{\"name\":\"FINANCE_TZ\"}":{".":{},"f:name":{},"f:value":{}}}}}
```

Only `.spec.env[name=FINANCE_TZ]` is owned by `hermes.agent/selfconfig`. Flux,
Argo, or `kubectl apply` can freely change `.spec.image.tag` without flap.
```

- [ ] **Step 3: Append to `docs/conditions.md`**

(If Plan 1 created the file, append. Otherwise, create with these entries plus an intro.)

```markdown
## HermesSelfConfig conditions

| Type | Status=True meaning | Reasons |
|---|---|---|
| `Applied` | The SSA writes succeeded for every requested action. | `SSASuccess` |
| `Denied` | Policy or validation rejected the request. No mutation occurred. | `PolicyViolation`, `InstanceNotFound`, `ProtectedPath`, `InvalidPatch`, `SSAConflict` |
| `Pending` | The controller has accepted the SelfConfig but not yet attempted apply. | `Accepted` |

Phase derives from conditions: `Applied → Applied`, `Denied → Denied`, otherwise `Pending`.
```

- [ ] **Step 4: Update README feature table**

In `README.md`, locate the feature table (Plan 1 Task 15) and add a row:

```markdown
| Self-configure | Agent-driven mutations via `HermesSelfConfig`. Server-Side Apply with field manager `hermes.agent/selfconfig` lets FluxCD/Argo co-own the instance. Allowlisted action categories: `skills`, `config`, `envVars`, `workspaceFiles`, `profiles`. Protected paths matched by glob. |
```

(Mirrors openclaw's README phrasing; the actions are hermes-specific.)

- [ ] **Step 5: Commit**

```bash
git add docs/api-reference.md docs/selfconfig.md docs/conditions.md README.md
git commit -m "docs: HermesSelfConfig API reference + SelfConfig guide + conditions + README row"
```

---

## Task 23: Push, watch CI, milestone tag

- [ ] **Step 1: Push the branch**

```bash
git push -u origin feat/plan-4-selfconfig
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create \
  --base main \
  --head feat/plan-4-selfconfig \
  --title "feat: HermesSelfConfig CRD + SSA reconciler (Plan 4)" \
  --body "$(cat <<'EOF'
## Summary

- Replace the scaffolded HermesSelfConfig stub with the full v1 type: addSkills, patchConfig, addEnvVars, addWorkspaceFiles, addProfileSnapshot.
- Reconciler writes exclusively via Server-Side Apply with field manager `hermes.agent/selfconfig`. No `r.Update` on HermesInstance.
- Policy enforcement against `selfConfigure.enabled`, `allowedActions`, `protectedKeys` (glob match on patch tree).
- Real validating webhook (instance existence, JSON merge patch validity, honcho gate).
- GitOps coexistence proven by envtest: Flux + SelfConfig co-own the same HermesInstance, no flap on re-apply.
- Metrics counters: `hermes_selfconfig_applied_total`, `hermes_selfconfig_denied_total`.
- Events: `SelfConfigApplied`, `SelfConfigDenied` on both SelfConfig and parent instance.
- Docs: api-reference entry, dedicated docs/selfconfig.md with deny-reason catalogue and worked example.

## Test plan

- [x] `make test` (envtest + unit) passes locally
- [x] `bash hack/reconcile-guard.sh` exits 0
- [x] `bash hack/check-helm-rbac.sh` exits 0
- [x] `helm lint charts/hermes-operator` exits 0
- [ ] CI green on push (CI, Reconcile Guard, Helm RBAC Sync, Build, E2E)
- [ ] kubectl-side smoke: apply parent + SelfConfig in kind, verify `kubectl get hsc` shows Applied
EOF
)"
```

- [ ] **Step 3: Watch CI**

```bash
gh pr checks --watch
```

Expected: all five workflows green. If `E2E` fails, the kind cluster needs the workspace ConfigMap volume mounted: Plan 3's StatefulSet builder is responsible.

- [ ] **Step 4: After merge, tag the milestone**

(Maintainer-only; the dispatching agent should do this after centrally merging the PR.)

```bash
git tag plan-4-selfconfig-ssa
git push origin plan-4-selfconfig-ssa
```

- [ ] **Step 5: Remove the worktree**

```bash
cd /Users/jannesstubbemann/repos/hermes-operator
git worktree remove ../hermes-operator-plan-4
```

---

## Self-review (verify before marking the plan complete)

**Spec coverage:**

- [ ] Spec §5 (HermesSelfConfig spec): `instanceRef` (Task 2), `addSkills` (Task 2), `patchConfig` (Task 3), `addEnvVars` (Task 3), `addWorkspaceFiles` (Task 3), `addProfileSnapshot` (Task 3), status with `phase`/`appliedAt`/`denyReason`/`appliedFields`/`conditions` (Task 4). Printer columns + short name `hsc` + categories (Task 4).
- [ ] Spec §7.2 rule 2 (SSA from day one): Task 10 establishes `applySSA` as the only write path; Task 12 wires every action through it; Task 19 proves it under Flux co-ownership.
- [ ] Spec §7.3 (selfconfig validator): Task 16 implements: required `instanceRef`, existence check, JSON-merge-patch validity, honcho gate. Deny-with-Event for protected-path hits is in Task 10's `deny()` + Task 14's event helpers.
- [ ] Spec §3 (CRD surface): short name `hsc`, categories `hermes`/`agents` (Task 4).
- [ ] Spec §10 conformance "GitOps coexistence": proven in envtest (Task 19); placeholder in `test/conformance/` for the Plan-6 scaled version (Task 20).
- [ ] Section 4 brief item 4.A (full types): Tasks 2-4.
- [ ] Section 4 brief item 4.B (SSA reconciler): Tasks 7-9, 10, 11, 12.
- [ ] Section 4 brief item 4.C (allowlist + protectedKeys): Task 6 + gates in Task 10.
- [ ] Section 4 brief item 4.D (audit events + metrics + appliedFields): Tasks 14 + 15 + status updates in Task 10.
- [ ] Section 4 brief item 4.E (validating webhook): Task 16.
- [ ] Section 4 brief item 4.F (GitOps coexistence test): Task 19.
- [ ] Section 4 brief item 4.G (conformance hook): Task 20.
- [ ] Section 4 brief item 4.H (docs): Task 22.

**Plan-1 conventions referenced (no re-definition):**

- [ ] `resources.Ptr[T]`: used in Task 11 (Job builder) without redefining.
- [ ] `resources.LabelsForInstance`: used in Task 11 without redefining.
- [ ] `resources.MergePreservingForeign`: not needed here because SSA handles merge semantics; no re-definition.
- [ ] Idempotency canary pattern: replicated in Task 18 against SelfConfig.
- [ ] Conventional commit prefixes: every task commits with `feat:` / `test:` / `docs:` / `chore:` per Plan 1 conventions.
- [ ] Worktree discipline: established in Task 1, dismantled in Task 23.

**Placeholder scan:**

- [ ] No "TBD" / "TODO" / "implement later" left in code blocks. The single forward-reference (Task 10 step 1 `applyAll` stub) is explicitly named "stubbed for Task 10; Task 12 fills in" and IS filled in by Task 12 step 1: the commit boundary in Task 15 covers both at once.
- [ ] Every step has either a runnable command (with expected output) or a complete code block.
- [ ] Task 11 (`buildSnapshotJob`) does not say "implement appropriate security": it sets `RunAsNonRoot`, `ReadOnlyRootFilesystem`, `AllowPrivilegeEscalation=false`, `Capabilities.Drop=ALL`, `SeccompProfile=RuntimeDefault` explicitly.
- [ ] Task 22 docs include the full deny-reason catalogue, not just "list deny reasons".

**Type / identifier consistency:**

- [ ] `SelfConfigFieldManager = "hermes.agent/selfconfig"`: defined in Task 7 (`selfconfig_apply.go`), referenced in Task 10 (`patchOptions`) and Task 19 (assertion).
- [ ] `ForceOwnershipAnnotation = "hermes.agent/force-ownership"`: defined Task 7, consumed Task 10, documented Task 22.
- [ ] `EventReasonSelfConfigApplied` / `EventReasonSelfConfigDenied`: defined Task 14, consumed Task 10.
- [ ] `selfConfigAppliedTotal` / `selfConfigDeniedTotal` + `incSelfConfigApplied` / `incSelfConfigDenied`: defined Task 15, consumed Task 10.
- [ ] `hermesv1.ActionSkills`, `ActionConfig`, `ActionEnvVars`, `ActionWorkspaceFiles`, `ActionProfiles`: defined Task 6 step 3, consumed Tasks 6, 10, 18, 19.
- [ ] `hermesv1.SelfConfigPhasePending` / `…Applied` / `…Denied`: defined Task 4, consumed Tasks 10, 13, 18, 19.
- [ ] `hermesv1.SelfConfigConditionApplied` / `…Denied` / `…Pending`: defined Task 4, consumed Task 10.
- [ ] `hermesv1.SelfConfigSkill`, `SelfConfigEnvVar`, `SelfConfigEnvVarSource`, `SelfConfigKeySelector`, `SelfConfigWorkspaceFile`, `SelfConfigProfileSnapshot`: defined Tasks 2-3, consumed Tasks 6-12, 16, 18, 19.
- [ ] `resources.WorkspaceConfigMapName`, `resources.EncodeWorkspacePath`, `resources.DecodeWorkspacePath`, `resources.HonchoPVCName`, `resources.BuildSnapshotJob`: defined Tasks 8 + 11, consumed Tasks 9, 11, 12.
- [ ] `buildSkillsPatch` / `buildEnvVarsPatch` / `buildWorkspaceFilesPatch` / `buildPatchConfigPayload` / `mergeConfigMapPatches` / `buildProfileSnapshotPayload`: all defined in Tasks 7-11 in `selfconfig_apply.go`, consumed in Task 12's `applyAll`.
- [ ] `formatAppliedFieldEnv` / `formatAppliedFieldSkill` / `formatAppliedFieldFile`: defined Task 7, consumed Task 12.
- [ ] `DetermineActions` / `CheckAllowedActions` / `CheckProtectedPaths`: defined Task 6, consumed Tasks 10 + 12 + 13 + 18.
- [ ] `parent.Spec.SelfConfigure.Enabled` typed `*bool`: Plan-3 assumption documented in Task 10's prose; `boolValue` helper handles the nil case.
- [ ] `parent.Spec.ProfileStore.Honcho.Enabled` typed `*bool`: Plan-3 assumption; consumed in Task 12 + Task 16.
- [ ] `hermesv1.InstanceSkill { Source, Version }`: Plan-3 assumption with fallback Note in Task 7 step 3.

**SSA mechanics precision (the headline requirement):**

- [ ] Reconciler uses `r.Patch(ctx, partial, client.Apply, client.FieldOwner("hermes.agent/selfconfig"))`: defined in `applySSA()` in Task 10 step 1.
- [ ] `client.ForceOwnership` is opt-in only via the `hermes.agent/force-ownership` annotation: `patchOptions()` in Task 10 step 1.
- [ ] The reconciler does **not** call `r.Update(ctx, &hermesInstance)`. Greppable check (Reconcile Guard from Plan 1 enforces this; the only `r.Create` in Task 12 is for the snapshot Job and is annotated with `// reconcile-guard:allow` to make the intent explicit).
- [ ] Partial objects carry `apiVersion + kind + name + namespace` and ONLY the fields being claimed (`newPartialInstance` in Task 7 step 3).
- [ ] SSA list-map markers asserted on `HermesInstance.spec.env` (`+listMapKey=name`) and `.spec.skills` (`+listMapKey=source`) by Task 5's CRD-YAML test.
- [ ] The GitOps coexistence test (Task 19) reads `metadata.managedFields` and asserts that `flux-controller` keeps ownership of `f:image` while `hermes.agent/selfconfig` owns `f:env`: this is the precise SSA semantic check.

End of Plan 4.
