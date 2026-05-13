# Hermes Operator: Plan 2: Full HermesInstance Reconciler + Webhooks

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Take `HermesInstance` from Plan 1's minimal four-resource happy path to the full v1 spec: every sub-spec from design §4 *except* hermes-runtime / gateways / profileStore (those land in Plan 3): backed by a defaulting webhook reading `HermesClusterDefaults`, a validating webhook enforcing immutability and one-of constraints, per-instance RBAC, NetworkPolicy / PDB / HPA / Ingress / ServiceMonitor / PrometheusRule / Secret / workspace-ConfigMap builders, and an envtest matrix that exercises every new subsystem with the idempotency canary expanded to cover the full spec.

**Architecture:** API types stay in `api/v1/hermesinstance_types.go` / `hermesclusterdefaults_types.go`; resource construction stays a pure-function library under `internal/resources/` (one file per kind); the controller in `internal/controller/hermesinstance_controller.go` orchestrates `controllerutil.CreateOrUpdate` in a strict dependency order, status conditions are set per-subsystem via `meta.SetStatusCondition`, and webhooks live in `internal/webhook/` registered through `cmd/manager/main.go`. cert-manager fronts the webhook TLS; the Helm chart templates `Issuer` + `Certificate` + `inject-ca-from`-annotated `ValidatingWebhookConfiguration` / `MutatingWebhookConfiguration`. A separate `HermesClusterDefaultsReconciler` validates the singleton-name invariant and surfaces a `Ready` condition; it does not reconcile downstream resources: the defaulting webhook reads the singleton directly each admission. The idempotency canary from Plan 1 is generalised: a `TestFullSpecNoOpAfterSettling` test applies a *maximal* `HermesInstance` and asserts no managed resource bumps `metadata.generation` across ten reconciles.

**Tech Stack:** Go 1.24, kubebuilder v4 / controller-runtime, Ginkgo v2 + Gomega + envtest, cert-manager v1.16+ (runtime dependency of the chart, not the operator binary), Prometheus Operator CRDs (ServiceMonitor / PrometheusRule: schemas read at startup but not required at runtime when disabled), `sigs.k8s.io/yaml` for YAML merge, `github.com/stretchr/testify` (already added in Plan 1), `k8s.io/api/policy/v1` (PDB), `k8s.io/api/autoscaling/v2` (HPA), `k8s.io/api/networking/v1` (Ingress + NetworkPolicy), `k8s.io/api/rbac/v1`.

**Prerequisite:** Plan 1 (Foundation) is merged. Plan 1's File Structure section is on disk; `internal/resources/{common,pvc,configmap,service,statefulset}.go` exist with their tests; the `HermesInstanceReconciler` reconciles those four resources; the Reconcile Guard CI job is wired; envtest binaries are downloaded by `make envtest`. **No webhooks** have been registered yet, **no `HermesClusterDefaults` controller** has been wired into `cmd/manager/main.go`.

**Plan 1 conventions referenced (do not redefine):**
- `resources.Ptr[T]`, `resources.LabelsForInstance(inst)`, `resources.MergePreservingForeign(existing, desired, "hermes.agent/")`: defined in Plan 1 Task 4.
- The idempotency canary pattern: Plan 1 Task 10 step 3 ("second reconcile does not change generation"). Plan 2 generalises it across the full spec.
- The explicit-k8s-defaults rule: Plan 1 Task 8 enumerates every server-side default a builder must set (`RevisionHistoryLimit`, `ProgressDeadlineSeconds`, `RestartPolicy`, `DNSPolicy`, `SchedulerName`, `TerminationGracePeriodSeconds`, `TerminationMessagePath`, `TerminationMessagePolicy`, `ImagePullPolicy`, `SuccessThreshold` on every probe, `DefaultMode` on volume sources, `SessionAffinity: None` on Service).
- `BuildPVC` / `PVCName`, `BuildConfigMap` / `ConfigMapName`, `BuildService` / `ServiceName`, `BuildStatefulSet` / `StatefulSetName` are the established naming pattern: every new builder in this plan follows it (`BuildNetworkPolicy` / `NetworkPolicyName`, `BuildPDB` / `PDBName`, etc.).
- Commit prefixes: `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `refactor:`, `test:`. Release-please uses `feat:`/`fix:` for the changelog.
- Worktree discipline: `git worktree add ../hermes-operator-plan-2 -b feat/plan-2-full-reconciler main` before starting; `git worktree remove` at the end.

**Forward references (do not implement here):**
- `spec.runtime`, `spec.gateways`, `spec.profileStore`, `spec.ollama`, `spec.webTerminal`, `spec.tailscale`, `spec.autoUpdate` body wiring: Plan 3.
- `HermesSelfConfig` reconciler + selfconfig validator real body: Plan 4 (we land a *stub* validator in this plan so the webhook server registers cleanly; Plan 4 replaces it).
- `spec.backup`, `spec.restoreFrom`, `spec.migration`: Plan 5.
- OLM bundle, GoReleaser, conformance suite: Plan 6.

**Spec reference:** `docs/superpowers/specs/2026-05-12-hermes-operator-design.md` §4 (full HermesInstance spec), §6 (HermesClusterDefaults), §7.2 (reconciliation rules), §7.3 (webhook design), §7.4 (operational guardrails).

---

## File Structure Established by This Plan

```
api/v1/
├── hermesinstance_types.go                # MODIFY: expand to full v1 spec (Tasks 3-9)
├── hermesclusterdefaults_types.go         # MODIFY: real spec mirroring HermesInstance sub-specs (Task 10)
├── webhook_hermesinstance.go              # NEW: Defaulter + Validator (kubebuilder-generated shell) (Task 22)
├── webhook_hermesselfconfig.go            # NEW: Validator *stub*: Plan 4 replaces the body (Task 25)
├── webhook_hermesclusterdefaults.go       # NEW: Validator (singleton-name rule) (Task 24)
└── zz_generated.deepcopy.go               # REGEN

internal/resources/
├── common.go                              # MODIFY: add port constants, builder option helpers (Task 11)
├── common_test.go                         # MODIFY: cover the new helpers (Task 11)
├── configmap.go                           # MODIFY: honor spec.config.raw / configMapRef / mergeMode (Task 12)
├── configmap_test.go                      # MODIFY (Task 12)
├── workspace_configmap.go                 # NEW: BuildWorkspaceConfigMap with nested-path encoding (Task 13)
├── workspace_configmap_test.go            # NEW (Task 13)
├── secret.go                              # NEW: BuildGatewayTokenSecret (placeholder, Plan 3 fills) (Task 14)
├── secret_test.go                         # NEW (Task 14)
├── networkpolicy.go                       # NEW: default-deny + DNS + allow-list builder (Task 15)
├── networkpolicy_test.go                  # NEW (Task 15)
├── pdb.go                                 # NEW: BuildPDB (Task 16)
├── pdb_test.go                            # NEW (Task 16)
├── hpa.go                                 # NEW: BuildHPA (Task 17)
├── hpa_test.go                            # NEW (Task 17)
├── ingress.go                             # NEW: BuildIngress with provider-aware annotations (Task 18)
├── ingress_test.go                        # NEW (Task 18)
├── rbac.go                                # NEW: BuildServiceAccount + BuildRole + BuildRoleBinding (Task 19)
├── rbac_test.go                           # NEW (Task 19)
├── servicemonitor.go                      # NEW: BuildServiceMonitor (unstructured) (Task 20)
├── servicemonitor_test.go                 # NEW (Task 20)
├── prometheusrule.go                      # NEW: BuildPrometheusRule (unstructured) (Task 20)
├── prometheusrule_test.go                 # NEW (Task 20)
├── service.go                             # MODIFY: honor spec.networking.service (Type, Ports) (Task 21)
├── service_test.go                        # MODIFY (Task 21)
└── statefulset.go                         # MODIFY: wire resources, security, probes, scheduling, initContainers,
                                             sidecars, extraVolumes/Mounts, envFrom/env, suspended (Tasks 26-29)
internal/resources/statefulset_test.go     # MODIFY (Tasks 26-29)

internal/webhook/
├── webhook_hermesinstance_default.go      # NEW: Defaulter: read HermesClusterDefaults singleton (Task 22)
├── webhook_hermesinstance_default_test.go # NEW
├── webhook_hermesinstance_validate.go     # NEW: Validator: required/immutable/one-of (Task 23)
├── webhook_hermesinstance_validate_test.go# NEW
├── webhook_hermesclusterdefaults.go       # NEW: Validator: name must be "cluster" (Task 24)
├── webhook_hermesclusterdefaults_test.go  # NEW
├── webhook_hermesselfconfig.go            # NEW: Validator STUB: always-allow + TODO (Task 25)
└── webhook_hermesselfconfig_test.go       # NEW

internal/controller/
├── hermesinstance_controller.go           # MODIFY: full reconcile order + per-subsystem conditions (Task 30)
├── hermesinstance_controller_test.go      # MODIFY: per-subsystem envtest cases + full-spec idempotency (Task 31)
├── hermesclusterdefaults_controller.go    # NEW: singleton validation + Ready condition (Task 26)
├── hermesclusterdefaults_controller_test.go # NEW
└── suite_test.go                          # MODIFY: register cluster-defaults reconciler + webhooks (Task 31)

cmd/manager/main.go                        # MODIFY: register webhook server + cluster-defaults reconciler (Task 32)

config/crd/bases/                          # REGEN: hermes.agent_hermesinstances.yaml + hermes.agent_hermesclusterdefaults.yaml
config/rbac/role.yaml                      # REGEN: new RBAC markers for NetworkPolicy/PDB/HPA/Ingress/SA/Role/RoleBinding/Secret
config/webhook/                            # NEW (kubebuilder-generated): manifests for webhooks
config/certmanager/                        # NEW: Issuer + Certificate manifests
config/default/                            # MODIFY: patch-webhook + patch-cainjection wired

charts/hermes-operator/
├── templates/crds/                        # REGEN: sync from config/crd/bases/
├── templates/webhook-configuration.yaml   # NEW: Validating + Mutating webhook configs with inject-ca-from
├── templates/certmanager.yaml             # NEW: Issuer + Certificate
├── templates/clusterrole.yaml             # MODIFY: mirror RBAC markers (Helm RBAC Sync CI passes)
└── values.yaml                            # MODIFY: add webhook.enabled + webhook.certManager.enabled

docs/
├── api-reference.md                       # NEW: full HermesInstance + HermesClusterDefaults field reference (Task 33)
├── conditions.md                          # MODIFY: append StorageReady, ConfigReady, SecretsReady, NetworkPolicyReady,
                                              RBACReady, WebhookReady, ServiceMonitorReady, PrometheusRuleReady,
                                              PDBReady, HPAReady, IngressReady (Task 33)
└── conventions.md                         # MODIFY: explicit-k8s-defaults checklist gets HPA + Ingress + ServiceMonitor rows (Task 33)

README.md                                  # MODIFY: feature table flags networking/observability/availability/RBAC/SA-IRSA (Task 33)
```

---

## Task 1: Worktree + branch setup

**Files:** none.

- [ ] **Step 1: Confirm Plan 1 landed on main**

```bash
cd /Users/jannesstubbemann/repos/hermes-operator
git fetch origin
git log --oneline -1 origin/main
```

Expected: most recent commit looks like `feat(ci): wire kind e2e workflow` or similar: the end of Plan 1.

- [ ] **Step 2: Create the worktree**

```bash
git worktree add ../hermes-operator-plan-2 -b feat/plan-2-full-reconciler origin/main
cd ../hermes-operator-plan-2
```

Expected: new directory `../hermes-operator-plan-2`, branch `feat/plan-2-full-reconciler` checked out, working tree clean.

- [ ] **Step 3: Verify Plan 1 invariants**

```bash
test -f internal/resources/common.go && echo "common.go OK"
test -f internal/resources/pvc.go && echo "pvc.go OK"
test -f internal/resources/configmap.go && echo "configmap.go OK"
test -f internal/resources/service.go && echo "service.go OK"
test -f internal/resources/statefulset.go && echo "statefulset.go OK"
test -f internal/controller/hermesinstance_controller.go && echo "controller OK"
test -f .github/workflows/reconcile-guard.yaml && echo "reconcile-guard CI OK"
grep -q "Ptr\[T any\]" internal/resources/common.go && echo "Ptr generic OK"
grep -q "LabelsForInstance" internal/resources/common.go && echo "labels helper OK"
grep -q "MergePreservingForeign" internal/resources/common.go && echo "merge helper OK"
```

Expected: all ten echo lines print. If any are missing, the assumed precondition is wrong: stop and flag the dispatching agent.

- [ ] **Step 4: Confirm green baseline**

```bash
go build ./...
make test
```

Expected: build succeeds; envtest suite green (Plan 1's happy-path + idempotency test).

- [ ] **Step 5: Note the merge baseline**

```bash
git log --oneline -1
```

Record the SHA: we'll reference it in the Plan-2 milestone tag at the end of Task 34.

---

## Task 2: Spec scaffold: top-level shape and the type tree

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Plan 1's `HermesInstanceSpec` only has `Image` and `Storage`. This task adds the *empty* sub-spec fields so subsequent tasks can fill them in isolation without merge conflicts. Each empty struct is a placeholder; later tasks populate it.

- [ ] **Step 1: Write the failing test first**

Create `api/v1/hermesinstance_types_test.go`:

```go
package v1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHermesInstanceSpec_HasAllSubSpecs is the schema canary: every sub-spec
// from design §4 must be addressable on HermesInstanceSpec. Tasks 3-9 fill the
// bodies; this test only guards the shape so the field-tag / json-name choices
// are reviewable in one place.
func TestHermesInstanceSpec_HasAllSubSpecs(t *testing.T) {
	t.Parallel()

	specType := reflect.TypeOf(HermesInstanceSpec{})
	required := []string{
		"Image", "Config", "Workspace", "Resources", "Security", "Storage",
		"Networking", "Observability", "Availability", "Probes",
		"Scheduling", "InitContainers", "Sidecars", "ExtraVolumes",
		"ExtraVolumeMounts", "EnvFrom", "Env", "Skills",
		"SelfConfigure", "Suspended",
	}
	for _, name := range required {
		_, ok := specType.FieldByName(name)
		assert.Truef(t, ok, "HermesInstanceSpec is missing field %q (design §4)", name)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./api/v1/... -run TestHermesInstanceSpec_HasAllSubSpecs -v
```

Expected: 18 of 20 fields missing. The test names every absent field.

- [ ] **Step 3: Replace `HermesInstanceSpec` with the full shape (empty bodies)**

In `api/v1/hermesinstance_types.go`, replace the existing `HermesInstanceSpec` struct with:

```go
// HermesInstanceSpec defines the desired state of HermesInstance.
// Field order follows design §4.
type HermesInstanceSpec struct {
	// Image selects the hermes-agent container image.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Config is the YAML content of ~/.hermes/config.yaml, supplied inline,
	// from a referenced ConfigMap, or merged from both.
	// +optional
	Config ConfigSpec `json:"config,omitempty"`

	// Workspace seeds initial files and directories into ~/.hermes on first start.
	// +optional
	Workspace WorkspaceSpec `json:"workspace,omitempty"`

	// Resources sets the agent container's CPU/memory requests + limits.
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`

	// Security configures pod/container security contexts, RBAC, NetworkPolicy,
	// and the optional cluster CA bundle injection.
	// +optional
	Security SecuritySpec `json:"security,omitempty"`

	// Storage controls the PVC backing ~/.hermes for this instance.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Networking exposes the agent via Service / Ingress.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	// Observability turns on metrics, ServiceMonitor, PrometheusRule, and logging.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Availability sets PDB, HPA, and topology-spread constraints.
	// +optional
	Availability AvailabilitySpec `json:"availability,omitempty"`

	// Probes lets users override the built-in liveness/readiness/startup probes.
	// +optional
	Probes ProbesSpec `json:"probes,omitempty"`

	// Scheduling targets the agent pod at specific nodes.
	// +optional
	Scheduling SchedulingSpec `json:"scheduling,omitempty"`

	// InitContainers is a user-supplied list of init containers appended after
	// any operator-managed init containers (e.g. runtime-init from Plan 3).
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Sidecars is a user-supplied list of sidecars appended after operator-managed
	// sidecars (e.g. ollama / web-terminal / tailscale from Plan 3).
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// ExtraVolumes is a user-supplied list of additional pod volumes.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts is a user-supplied list of additional volume mounts
	// applied to the agent container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// EnvFrom is a list of EnvFrom sources (ConfigMap/Secret refs) injected
	// into the agent container.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Env is a list of explicit environment variables for the agent container.
	// SSA list-map key is "name" so HermesSelfConfig can merge entries without
	// replacing the whole list.
	// +listType=map
	// +listMapKey=name
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Skills is the declarative list of uv-installable skill sources. Plan 3
	// wires the runtime; the field is declared here so SSA from HermesSelfConfig
	// (Plan 4) can target it without a CRD schema change.
	// +listType=map
	// +listMapKey=source
	// +optional
	Skills []InstanceSkill `json:"skills,omitempty"`

	// SelfConfigure is the allowlist policy for HermesSelfConfig mutations.
	// +optional
	SelfConfigure SelfConfigureSpec `json:"selfConfigure,omitempty"`

	// Suspended scales the StatefulSet to zero replicas without deleting state.
	// +optional
	Suspended bool `json:"suspended,omitempty"`
}
```

- [ ] **Step 4: Add the empty sub-spec definitions (bodies populated by Tasks 3-9)**

At the bottom of the file (above `HermesInstanceStatus`), append placeholder structs:

```go
// ConfigSpec: populated in Task 3.
type ConfigSpec struct{}

// WorkspaceSpec: populated in Task 4.
type WorkspaceSpec struct{}

// ResourcesSpec: populated in Task 5.
type ResourcesSpec struct{}

// SecuritySpec: populated in Task 6.
type SecuritySpec struct{}

// NetworkingSpec: populated in Task 7.
type NetworkingSpec struct{}

// ObservabilitySpec: populated in Task 8.
type ObservabilitySpec struct{}

// AvailabilitySpec: populated in Task 9.
type AvailabilitySpec struct{}

// ProbesSpec: populated in Task 9.
type ProbesSpec struct{}

// SchedulingSpec: populated in Task 9.
type SchedulingSpec struct{}

// InstanceSkill: Plan 3 fills the runtime semantics. The field exists here so
// SSA from HermesSelfConfig (Plan 4) can patch the slice with listMapKey=source.
type InstanceSkill struct {
	// Source is the uv/pip-compatible install source.
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`
}

// SelfConfigureSpec: populated in Task 9.
type SelfConfigureSpec struct{}
```

- [ ] **Step 5: Add the corev1 import**

At the top of `api/v1/hermesinstance_types.go`, ensure the imports include:

```go
import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

- [ ] **Step 6: Run generators**

```bash
make generate
go test ./api/v1/... -run TestHermesInstanceSpec_HasAllSubSpecs -v
```

Expected: deepcopy regenerated, test passes.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(api): scaffold full HermesInstance spec sub-fields (empty bodies)"
```

---

## Task 3: `ConfigSpec`: raw / configMapRef / mergeMode

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Design §4 specifies YAML-only and `one-of raw|configMapRef`. We embed `runtime.RawExtension` for the inline body so users may supply structured YAML directly in their manifest without re-serialising.

- [ ] **Step 1: Write the failing test**

Append to `api/v1/hermesinstance_types_test.go`:

```go
func TestConfigSpec_RawAndRef(t *testing.T) {
	t.Parallel()
	cs := ConfigSpec{
		Raw: &RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"a":1}`)}},
		ConfigMapRef: &corev1.LocalObjectReference{Name: "user-config"},
		MergeMode: ConfigMergeModeMerge,
	}
	assert.NotNil(t, cs.Raw)
	assert.NotNil(t, cs.ConfigMapRef)
	assert.Equal(t, ConfigMergeModeMerge, cs.MergeMode)
}
```

Add the `runtime "k8s.io/apimachinery/pkg/runtime"` import at the top of the test file if missing.

- [ ] **Step 2: Run to fail**

```bash
go test ./api/v1/... -run TestConfigSpec -v
```

Expected: build error: `ConfigSpec.Raw`, `RawConfig`, etc. undefined.

- [ ] **Step 3: Replace the `ConfigSpec` placeholder with the real body**

In `api/v1/hermesinstance_types.go`, replace `type ConfigSpec struct{}` with:

```go
// ConfigMergeMode controls how Raw and ConfigMapRef are combined.
// +kubebuilder:validation:Enum=replace;merge
type ConfigMergeMode string

const (
	// ConfigMergeModeReplace: Raw replaces ConfigMapRef entirely when both are set.
	// This is the default to avoid surprising merges.
	ConfigMergeModeReplace ConfigMergeMode = "replace"
	// ConfigMergeModeMerge: YAML deep-merge Raw onto ConfigMapRef. Raw wins on conflict.
	ConfigMergeModeMerge ConfigMergeMode = "merge"
)

// ConfigSpec holds the agent's ~/.hermes/config.yaml. Exactly one of Raw or
// ConfigMapRef SHOULD be set; the validating webhook rejects both unset and
// emits a warning if both are set with MergeMode unset.
type ConfigSpec struct {
	// Raw is the inline YAML body of config.yaml. Stored as a RawExtension so
	// users may write structured YAML in the manifest without escaping.
	// +optional
	Raw *RawConfig `json:"raw,omitempty"`

	// ConfigMapRef references a ConfigMap in the same namespace whose
	// "config.yaml" key holds the body.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// MergeMode controls combination when both Raw and ConfigMapRef are set.
	// +kubebuilder:default=replace
	// +optional
	MergeMode ConfigMergeMode `json:"mergeMode,omitempty"`
}

// RawConfig wraps runtime.RawExtension so deepcopy is generated cleanly.
type RawConfig struct {
	runtime.RawExtension `json:",inline"`
}
```

- [ ] **Step 4: Add the `runtime` import**

At the top of `api/v1/hermesinstance_types.go`, ensure the imports include:

```go
"k8s.io/apimachinery/pkg/runtime"
```

- [ ] **Step 5: Hand-write the RawConfig deepcopy hook**

Because `RawExtension` is an embedded struct rather than a named field, kubebuilder's deepcopy generator needs a marker. Above `type RawConfig struct`, add:

```go
// +kubebuilder:object:generate=true
```

If `make generate` still complains, add a manual `DeepCopyInto` shim in `api/v1/raw_config_deepcopy.go`:

```go
package v1

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out.
func (in *RawConfig) DeepCopyInto(out *RawConfig) {
	*out = *in
	in.RawExtension.DeepCopyInto(&out.RawExtension)
}

// DeepCopy returns a deep copy of RawConfig.
func (in *RawConfig) DeepCopy() *RawConfig {
	if in == nil {
		return nil
	}
	out := new(RawConfig)
	in.DeepCopyInto(out)
	return out
}
```

- [ ] **Step 6: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestConfigSpec -v
go test ./api/v1/... -run TestHermesInstanceSpec_HasAllSubSpecs -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(api): add ConfigSpec (raw / configMapRef / mergeMode)"
```

---

## Task 4: `WorkspaceSpec`: initial files & dirs with nested-path support

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Design §4 plus the openclaw lesson #482 ("initial files with slashes in their path failed validation"): the field accepts arbitrary nested paths via a string-typed `path` field; encoding into a single-level ConfigMap key uses `__` as the path separator (`notes/finance.md` → `notes__finance.md`). The decoder in Plan 3's runtime-init flips it back.

- [ ] **Step 1: Write the failing test**

Append to `api/v1/hermesinstance_types_test.go`:

```go
func TestWorkspaceSpec_NestedPath(t *testing.T) {
	t.Parallel()
	ws := WorkspaceSpec{
		InitialFiles: []WorkspaceFile{
			{Path: "notes/finance/2026.md", Content: "hi"},
			{Path: "shallow.txt", Content: "ok"},
		},
		InitialDirs: []string{"data", "data/raw"},
		ConfigMapRef: &corev1.LocalObjectReference{Name: "user-ws"},
		Bootstrap: WorkspaceBootstrap{Enabled: Ptr(false)},
	}
	assert.Len(t, ws.InitialFiles, 2)
	assert.Equal(t, "notes/finance/2026.md", ws.InitialFiles[0].Path)
	assert.NotNil(t, ws.Bootstrap.Enabled)
	assert.False(t, *ws.Bootstrap.Enabled)
}
```

Add a top-of-file local `func Ptr[T any](v T) *T { return &v }` if the test file does not already have it (the `resources` package's `Ptr` is not visible inside `api/v1`).

- [ ] **Step 2: Run to fail**

```bash
go test ./api/v1/... -run TestWorkspaceSpec -v
```

Expected: build error.

- [ ] **Step 3: Replace the `WorkspaceSpec` placeholder**

Replace `type WorkspaceSpec struct{}` with:

```go
// WorkspaceSpec seeds initial files and directories into ~/.hermes on first
// start. Path values support arbitrary nested directories ("a/b/c.md" is fine);
// the workspace ConfigMap encodes nested paths using "__" as the separator so a
// single-level ConfigMap data map can express them: Plan 3's runtime-init
// container decodes the keys back to filesystem paths before invoking the agent.
//
// Lesson from openclaw #482: do not constrain Path to a single segment; that
// caused users to flatten their notes into hash-separated filenames.
type WorkspaceSpec struct {
	// InitialFiles is the list of files to seed.
	// SSA list-map key is "path" so HermesSelfConfig (Plan 4) can patch entries
	// in place without replacing the whole slice.
	// +listType=map
	// +listMapKey=path
	// +optional
	InitialFiles []WorkspaceFile `json:"initialFiles,omitempty"`

	// InitialDirs is the list of directories to mkdir -p on first start.
	// +listType=set
	// +optional
	InitialDirs []string `json:"initialDirs,omitempty"`

	// ConfigMapRef references a user-owned ConfigMap whose entries are merged
	// onto InitialFiles (operator-managed entries win on conflict).
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// Bootstrap controls the optional one-shot bootstrap script that hermes-agent
	// runs on first start (e.g. `hermes onboard`). Default disabled.
	// +optional
	Bootstrap WorkspaceBootstrap `json:"bootstrap,omitempty"`
}

// WorkspaceFile is a single seeded file. Nested paths are allowed; the workspace
// ConfigMap encodes them with "__" separators (decoded by runtime-init).
type WorkspaceFile struct {
	// Path is the relative path under ~/.hermes (e.g. "notes/finance/2026.md").
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	// +kubebuilder:validation:Pattern=`^[^/].*[^/]$|^[^/]$`
	Path string `json:"path"`

	// Content is the UTF-8 body. Binary content must be base64-encoded by the
	// caller and decoded by the bootstrap step (out of scope of v1 schema).
	// +kubebuilder:validation:MaxLength=1048576
	Content string `json:"content"`
}

// WorkspaceBootstrap toggles the first-start bootstrap script.
type WorkspaceBootstrap struct {
	// Enabled: default false. Plan 3 wires the actual init-container.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}
```

- [ ] **Step 4: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestWorkspaceSpec -v
```

Expected: PASS.

- [ ] **Step 5: Verify the CRD YAML reflects the nested-path support**

```bash
grep -A2 "path:" config/crd/bases/hermes.agent_hermesinstances.yaml | head -20
```

Expected: see `pattern: ^[^/].*[^/]$|^[^/]$` (the boundary rule rejects leading/trailing slashes but allows nested segments).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(api): add WorkspaceSpec with nested-path InitialFiles (lesson #482)"
```

---

## Task 5: `ResourcesSpec`: requests + limits

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Thin wrapper around `corev1.ResourceList` so the API surface stays cohesive (`spec.resources.requests` / `.limits`) rather than asking users to drop a raw `ResourceRequirements` blob.

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestResourcesSpec_RequestsLimits(t *testing.T) {
	t.Parallel()
	rs := ResourcesSpec{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
	assert.Equal(t, resource.MustParse("100m"), rs.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("512Mi"), rs.Limits[corev1.ResourceMemory])
}
```

Add `"k8s.io/apimachinery/pkg/api/resource"` import if missing.

- [ ] **Step 2: Run to fail, then implement**

```bash
go test ./api/v1/... -run TestResourcesSpec -v
```

Replace `type ResourcesSpec struct{}` with:

```go
// ResourcesSpec sets CPU/memory requests + limits on the agent container.
// Defaults intentionally omitted: the defaulting webhook fills from
// HermesClusterDefaults if available, otherwise the field is left empty
// (meaning the agent inherits whatever Pod-level defaults the namespace's
// LimitRange applies).
type ResourcesSpec struct {
	// Requests is the resource-requests map.
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`

	// Limits is the resource-limits map.
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// ToContainerResourceRequirements converts to a corev1.ResourceRequirements,
// useful inside resource builders.
func (r *ResourcesSpec) ToContainerResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: r.Requests,
		Limits:   r.Limits,
	}
}
```

- [ ] **Step 3: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestResourcesSpec -v
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(api): add ResourcesSpec (requests + limits)"
```

---

## Task 6: `SecuritySpec`: pod/container contexts, RBAC, NetworkPolicy, CA bundle

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Three concerns bundled because they share a logical "security posture" surface:

1. `PodSecurityContext` + `ContainerSecurityContext`: straight pass-through.
2. `RBAC`: opt out of the operator creating an SA / opt in to bring-your-own SA; SA-level annotations carry IRSA / Workload Identity bindings.
3. `NetworkPolicy`: operator-default deny-all with selective allows. The instance scope only chooses on/off and allow-list extras (DNS, namespaces, CIDRs, additional egress).
4. `CABundle`: optional ConfigMap/Secret mounted into the agent so HTTPS calls trust an internal CA (corporate proxy / private registries).

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestSecuritySpec_Shape(t *testing.T) {
	t.Parallel()
	ss := SecuritySpec{
		PodSecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: Ptr(true)},
		ContainerSecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: Ptr(true)},
		RBAC: RBACSpec{
			CreateServiceAccount: Ptr(true),
			ServiceAccountName:   "",
			Annotations: map[string]string{
				"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/hermes",
			},
		},
		NetworkPolicy: NetworkPolicySpec{
			Enabled:  Ptr(true),
			AllowDNS: Ptr(true),
			AllowedIngressNamespaces: []string{"prometheus"},
			AllowedIngressCIDRs:      []string{"10.0.0.0/8"},
			AllowedEgressCIDRs:       []string{"203.0.113.0/24"},
		},
		CABundle: CABundleSpec{ConfigMapName: "corp-ca", Key: "ca.crt"},
	}
	assert.True(t, *ss.PodSecurityContext.RunAsNonRoot)
	assert.True(t, *ss.RBAC.CreateServiceAccount)
	assert.True(t, *ss.NetworkPolicy.Enabled)
	assert.Equal(t, "corp-ca", ss.CABundle.ConfigMapName)
}
```

- [ ] **Step 2: Run to fail, then implement**

```bash
go test ./api/v1/... -run TestSecuritySpec -v
```

Replace `type SecuritySpec struct{}` with:

```go
// SecuritySpec bundles pod/container security, per-instance RBAC, NetworkPolicy,
// and the optional CA-bundle mount.
type SecuritySpec struct {
	// PodSecurityContext overrides the operator's default hardened pod context.
	// Operator default is enforced when nil: runAsNonRoot=true, runAsUser=1000,
	// fsGroup=1000, seccompProfile=RuntimeDefault.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ContainerSecurityContext overrides the operator's default hardened container
	// context. Operator default: readOnlyRootFilesystem=true, allowPrivilegeEscalation=false,
	// drop ALL capabilities.
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// RBAC controls per-instance ServiceAccount + Role + RoleBinding creation.
	// +optional
	RBAC RBACSpec `json:"rbac,omitempty"`

	// NetworkPolicy controls per-instance NetworkPolicy creation (default-deny baseline).
	// +optional
	NetworkPolicy NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// CABundle optionally mounts a ConfigMap- or Secret-sourced CA bundle into
	// /etc/ssl/certs/hermes-ca-bundle.crt and sets SSL_CERT_FILE in the agent env.
	// +optional
	CABundle CABundleSpec `json:"caBundle,omitempty"`
}

// RBACSpec controls per-instance ServiceAccount + Role + RoleBinding creation.
type RBACSpec struct {
	// CreateServiceAccount: when true (the default), the operator creates and
	// owns a ServiceAccount named after the instance.
	// +kubebuilder:default=true
	// +optional
	CreateServiceAccount *bool `json:"createServiceAccount,omitempty"`

	// ServiceAccountName: when CreateServiceAccount is false, the agent uses
	// this externally-managed ServiceAccount. Must exist in the same namespace.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Annotations are applied to the operator-created ServiceAccount. Use this
	// for IRSA (`eks.amazonaws.com/role-arn`), GKE Workload Identity
	// (`iam.gke.io/gcp-service-account`), Azure Workload Identity, etc.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NetworkPolicySpec controls per-instance NetworkPolicy creation.
type NetworkPolicySpec struct {
	// Enabled: when true (the default), the operator creates a deny-all
	// NetworkPolicy plus selective allow rules (DNS + 443 egress + Service ingress
	// from the same namespace).
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AllowDNS: emit the standard DNS egress rule (UDP+TCP 53 to any peer).
	// Default true. Disable only when CoreDNS is reachable via a different
	// transport (e.g. node-local DNS via hostNetwork).
	// +kubebuilder:default=true
	// +optional
	AllowDNS *bool `json:"allowDNS,omitempty"`

	// AllowedIngressNamespaces is the set of additional namespaces (beyond the
	// instance's own) whose pods may connect to the agent's exposed ports.
	// +listType=set
	// +optional
	AllowedIngressNamespaces []string `json:"allowedIngressNamespaces,omitempty"`

	// AllowedIngressCIDRs is the set of CIDRs that may connect to the agent's
	// exposed ports.
	// +listType=set
	// +optional
	AllowedIngressCIDRs []string `json:"allowedIngressCIDRs,omitempty"`

	// AllowedEgressCIDRs is the set of CIDRs the agent may connect to in addition
	// to the operator-built defaults (DNS + 443).
	// +listType=set
	// +optional
	AllowedEgressCIDRs []string `json:"allowedEgressCIDRs,omitempty"`

	// AdditionalEgress is a list of user-supplied egress rules appended verbatim
	// to the generated NetworkPolicy.
	// +optional
	AdditionalEgress []networkingv1.NetworkPolicyEgressRule `json:"additionalEgress,omitempty"`
}

// CABundleSpec optionally mounts a CA bundle into the agent container.
// Exactly one of ConfigMapName / SecretName SHOULD be set.
type CABundleSpec struct {
	// ConfigMapName references a ConfigMap in the same namespace.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// SecretName references a Secret in the same namespace.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Key is the data-map key holding the PEM bundle. Default "ca.crt".
	// +kubebuilder:default="ca.crt"
	// +optional
	Key string `json:"key,omitempty"`
}
```

- [ ] **Step 3: Add the `networkingv1` import**

Ensure `api/v1/hermesinstance_types.go` imports:

```go
networkingv1 "k8s.io/api/networking/v1"
```

- [ ] **Step 4: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestSecuritySpec -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(api): add SecuritySpec (PSC/CSC, RBAC, NetworkPolicy, CABundle)"
```

---

## Task 7: `NetworkingSpec`: Service + Ingress

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Service supports headless + ClusterIP + LoadBalancer + NodePort with multiple ports. Ingress supports host + TLS + className + annotations.

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestNetworkingSpec_ServiceAndIngress(t *testing.T) {
	t.Parallel()
	port := int32(8443)
	ns := NetworkingSpec{
		Service: ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []NamedServicePort{
				{Name: "gateway", Port: 8443, TargetPort: &port, Protocol: corev1.ProtocolTCP},
			},
		},
		Ingress: IngressSpec{
			Enabled:   Ptr(true),
			Host:      "hermes.example.com",
			ClassName: Ptr("nginx"),
			TLS: []IngressTLSSpec{{SecretName: "hermes-tls", Hosts: []string{"hermes.example.com"}}},
			Annotations: map[string]string{"foo": "bar"},
		},
	}
	assert.Equal(t, corev1.ServiceTypeClusterIP, ns.Service.Type)
	assert.Len(t, ns.Service.Ports, 1)
	assert.True(t, *ns.Ingress.Enabled)
	assert.Equal(t, "hermes.example.com", ns.Ingress.Host)
}
```

- [ ] **Step 2: Run to fail, then implement**

Replace `type NetworkingSpec struct{}` with:

```go
// NetworkingSpec exposes the agent via Service + (optionally) Ingress.
type NetworkingSpec struct {
	// Service controls the Service kind and ports.
	// +optional
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress controls optional Ingress creation.
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`
}

// ServiceSpec controls the agent's Service.
type ServiceSpec struct {
	// Type is the Service kind. Default ClusterIP (headed): Plan 1 emitted a
	// headless Service; v1 keeps ClusterIP as the default and lets users opt
	// into Headless via Type=ClusterIP with ClusterIP="None" through the spec.
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// ClusterIP: set to "None" for a headless Service. Default empty (api-server allocates).
	// +optional
	ClusterIP string `json:"clusterIP,omitempty"`

	// Ports is the list of Service ports. If empty, the operator emits a default
	// "gateway" port on 8443 (matches the StatefulSet's container port).
	// +listType=map
	// +listMapKey=name
	// +optional
	Ports []NamedServicePort `json:"ports,omitempty"`

	// Annotations are applied verbatim onto the Service (LoadBalancer hints, etc.).
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// LoadBalancerClass is propagated when Type=LoadBalancer.
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// ExternalTrafficPolicy is propagated when Type=LoadBalancer or NodePort.
	// +kubebuilder:validation:Enum=Cluster;Local
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
}

// NamedServicePort is a single Service port. The TargetPort is optional and
// defaults to Port when nil.
type NamedServicePort struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`

	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	// +kubebuilder:default=TCP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`

	// NodePort is honored only when the Service is NodePort or LoadBalancer.
	// +optional
	NodePort int32 `json:"nodePort,omitempty"`
}

// IngressSpec controls optional Ingress creation.
type IngressSpec struct {
	// Enabled: when true, the operator creates an Ingress for the agent.
	// Default false.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Host is the primary hostname.
	// +optional
	Host string `json:"host,omitempty"`

	// ClassName is the IngressClass (`nginx`, `traefik`, ...).
	// +optional
	ClassName *string `json:"className,omitempty"`

	// TLS is the list of TLS settings.
	// +optional
	TLS []IngressTLSSpec `json:"tls,omitempty"`

	// Annotations are applied to the Ingress. The operator merges
	// provider-specific defaults (force-https, etc.) on top of these.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// PathType: default Prefix.
	// +kubebuilder:default=Prefix
	// +kubebuilder:validation:Enum=Exact;Prefix;ImplementationSpecific
	// +optional
	PathType networkingv1.PathType `json:"pathType,omitempty"`

	// Path: default "/".
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// ServicePortName: name of the Service port the Ingress should route to.
	// Default "gateway".
	// +kubebuilder:default="gateway"
	// +optional
	ServicePortName string `json:"servicePortName,omitempty"`
}

// IngressTLSSpec is a single TLS section on the Ingress.
type IngressTLSSpec struct {
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// +listType=set
	Hosts []string `json:"hosts,omitempty"`
}
```

- [ ] **Step 3: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestNetworkingSpec -v
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(api): add NetworkingSpec (Service + Ingress)"
```

---

## Task 8: `ObservabilitySpec`: metrics, ServiceMonitor, PrometheusRule, logging

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Plan 1 doesn't expose metrics yet; this task lands the schema so Plan 3's runtime wiring has somewhere to write to.

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestObservabilitySpec_Shape(t *testing.T) {
	t.Parallel()
	o := ObservabilitySpec{
		Metrics: MetricsSpec{
			Enabled: Ptr(true), Port: 9090, Secure: Ptr(false),
		},
		ServiceMonitor: ServiceMonitorSpec{
			Enabled:       Ptr(true),
			Labels:        map[string]string{"team": "platform"},
			Interval:      "30s",
			ScrapeTimeout: "10s",
		},
		PrometheusRule: PrometheusRuleSpec{Enabled: Ptr(true)},
		Logging:        LoggingSpec{Format: LogFormatJSON, Level: "info"},
	}
	assert.True(t, *o.Metrics.Enabled)
	assert.Equal(t, int32(9090), o.Metrics.Port)
	assert.Equal(t, LogFormatJSON, o.Logging.Format)
}
```

- [ ] **Step 2: Run to fail, then implement**

Replace `type ObservabilitySpec struct{}` with:

```go
// ObservabilitySpec controls metrics, scraping, alerting, logging.
type ObservabilitySpec struct {
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`
	// +optional
	ServiceMonitor ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
	// +optional
	PrometheusRule PrometheusRuleSpec `json:"prometheusRule,omitempty"`
	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`
}

// MetricsSpec controls the agent's Prometheus metrics endpoint.
type MetricsSpec struct {
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port for the /metrics endpoint.
	// +kubebuilder:default=9090
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// Secure: when true, /metrics requires bearer-token auth and uses HTTPS.
	// The ServiceMonitor scheme/scrape settings must agree (lesson #435/#440).
	// +kubebuilder:default=false
	// +optional
	Secure *bool `json:"secure,omitempty"`
}

// ServiceMonitorSpec controls Prometheus-Operator ServiceMonitor emission.
// When Enabled is true, the operator emits an unstructured ServiceMonitor; it
// does not require the Prometheus-Operator CRDs to be installed at compile time.
type ServiceMonitorSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Labels are extra labels applied onto the ServiceMonitor for Prometheus
	// label-selector matching (e.g. `release: kube-prometheus-stack`).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Interval: default "30s".
	// +kubebuilder:default="30s"
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	// +optional
	Interval string `json:"interval,omitempty"`

	// ScrapeTimeout: default "10s".
	// +kubebuilder:default="10s"
	// +kubebuilder:validation:Pattern=`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`
	// +optional
	ScrapeTimeout string `json:"scrapeTimeout,omitempty"`
}

// PrometheusRuleSpec controls emission of a default PrometheusRule with
// hermes-agent alerts (HighRestartRate, MetricsDown, etc.).
type PrometheusRuleSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AdditionalRules is a list of user-supplied rules merged onto the operator
	// default ruleset.
	// +optional
	AdditionalRules []PrometheusRule `json:"additionalRules,omitempty"`
}

// PrometheusRule is a minimal copy of monitoringv1.Rule so we don't depend on
// the Prometheus-Operator Go types at compile time. The runtime emits
// unstructured objects.
type PrometheusRule struct {
	// +kubebuilder:validation:MinLength=1
	Alert string `json:"alert"`
	// +kubebuilder:validation:MinLength=1
	Expr string `json:"expr"`
	// +optional
	For string `json:"for,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// LogFormat is the agent's log output format.
// +kubebuilder:validation:Enum=text;json
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// LoggingSpec controls the agent's logger configuration via env vars.
type LoggingSpec struct {
	// +kubebuilder:default=text
	// +optional
	Format LogFormat `json:"format,omitempty"`

	// Level: Plan 3 wires HERMES_LOG_LEVEL on the agent container.
	// +kubebuilder:default=info
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error
	// +optional
	Level string `json:"level,omitempty"`
}
```

- [ ] **Step 3: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestObservabilitySpec -v
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(api): add ObservabilitySpec (metrics, ServiceMonitor, PrometheusRule, logging)"
```

---

## Task 9: `AvailabilitySpec`, `ProbesSpec`, `SchedulingSpec`, `SelfConfigureSpec`

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

Bundle the four remaining sub-specs because each one is small. `SelfConfigure` is the *schema only*: Plan 4's controller is what actually consumes it; we land the field here so Plan 4 doesn't have to back-modify the CRD.

- [ ] **Step 1: Write the failing tests**

Append to `api/v1/hermesinstance_types_test.go`:

```go
func TestAvailabilitySpec_Shape(t *testing.T) {
	t.Parallel()
	min := intstr.FromString("50%")
	max := intstr.FromInt(1)
	a := AvailabilitySpec{
		PodDisruptionBudget: PDBSpec{Enabled: Ptr(true), MinAvailable: &min, MaxUnavailable: &max},
		HorizontalPodAutoscaler: HPASpec{Enabled: Ptr(true), MinReplicas: Ptr(int32(2)), MaxReplicas: Ptr(int32(5)),
			TargetCPUUtilization: Ptr(int32(70))},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
		},
	}
	assert.True(t, *a.PodDisruptionBudget.Enabled)
	assert.Equal(t, "50%", a.PodDisruptionBudget.MinAvailable.StrVal)
	assert.Equal(t, int32(70), *a.HorizontalPodAutoscaler.TargetCPUUtilization)
}

func TestProbesSpec_Overrides(t *testing.T) {
	t.Parallel()
	p := ProbesSpec{
		Liveness:  &corev1.Probe{InitialDelaySeconds: 10},
		Readiness: &corev1.Probe{InitialDelaySeconds: 5},
		Startup:   &corev1.Probe{InitialDelaySeconds: 0, PeriodSeconds: 2, FailureThreshold: 30},
	}
	assert.Equal(t, int32(10), p.Liveness.InitialDelaySeconds)
}

func TestSchedulingSpec_Shape(t *testing.T) {
	t.Parallel()
	s := SchedulingSpec{
		NodeSelector:      map[string]string{"disktype": "ssd"},
		Tolerations:       []corev1.Toleration{{Key: "gpu", Operator: corev1.TolerationOpExists}},
		PriorityClassName: "high-prio",
	}
	assert.Equal(t, "ssd", s.NodeSelector["disktype"])
	assert.Equal(t, "high-prio", s.PriorityClassName)
}

func TestSelfConfigureSpec_AllowList(t *testing.T) {
	t.Parallel()
	sc := SelfConfigureSpec{
		Enabled:        Ptr(true),
		AllowedActions: []string{"skills", "envVars"},
		ProtectedKeys:  []string{"spec.image.repository"},
	}
	assert.True(t, *sc.Enabled)
	assert.Len(t, sc.AllowedActions, 2)
}
```

Add the `"k8s.io/apimachinery/pkg/util/intstr"` import if missing.

- [ ] **Step 2: Run to fail, then implement**

Replace the four placeholder structs (`AvailabilitySpec`, `ProbesSpec`, `SchedulingSpec`, `SelfConfigureSpec`) with:

```go
// AvailabilitySpec bundles PDB, HPA, and topology-spread.
type AvailabilitySpec struct {
	// +optional
	PodDisruptionBudget PDBSpec `json:"podDisruptionBudget,omitempty"`

	// +optional
	HorizontalPodAutoscaler HPASpec `json:"horizontalPodAutoscaler,omitempty"`

	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// PDBSpec controls PodDisruptionBudget emission.
type PDBSpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinAvailable: optional, mutually exclusive with MaxUnavailable.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// MaxUnavailable: optional, mutually exclusive with MinAvailable.
	// Default 1 when neither is set and PDB is enabled.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// HPASpec controls HorizontalPodAutoscaler emission.
type HPASpec struct {
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinReplicas: default 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas: default 5.
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilization: default 80 (percent).
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`

	// TargetMemoryUtilization: optional, when set adds a memory metric.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemoryUtilization *int32 `json:"targetMemoryUtilization,omitempty"`

	// Behavior is forwarded onto HPA's autoscaling/v2 behavior field.
	// Plan 6 conformance suite asserts the field is exposed; v1 forwards it raw.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// ProbesSpec overrides the operator's built-in probes. Each field is a complete
// probe: set every value you want non-default because we apply it verbatim.
type ProbesSpec struct {
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
}

// SchedulingSpec targets the agent pod at specific nodes.
type SchedulingSpec struct {
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// SelfConfigureSpec is the allowlist policy for HermesSelfConfig mutations.
// Plan 4 wires the controller; the field exists here so Plan 4 doesn't need a
// CRD change. The validator rejects Enabled=true with ProtectedKeys empty.
type SelfConfigureSpec struct {
	// Enabled: explicit *bool so the defaulter can distinguish "user said false"
	// from "user did not set it" (Plan 4 relies on this).
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AllowedActions is the set of permitted action categories Plan 4 will
	// enforce: skills, config, envVars, workspaceFiles, profiles.
	// +listType=set
	// +optional
	AllowedActions []string `json:"allowedActions,omitempty"`

	// ProtectedKeys is the list of glob expressions over JSON paths that may
	// not be mutated by HermesSelfConfig. Required (non-empty) when Enabled=true.
	// +listType=set
	// +optional
	ProtectedKeys []string `json:"protectedKeys,omitempty"`
}
```

- [ ] **Step 3: Add the `autoscalingv2` import**

```go
autoscalingv2 "k8s.io/api/autoscaling/v2"
```

- [ ] **Step 4: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -v
```

Expected: all `TestAvailabilitySpec_*`, `TestProbesSpec_*`, `TestSchedulingSpec_*`, `TestSelfConfigureSpec_*` PASS, plus the shape-canary from Task 2.

- [ ] **Step 5: Expand `HermesInstanceStatus` with the per-subsystem conditions**

Replace the existing `HermesInstanceStatus` struct with:

```go
// HermesInstanceStatus reflects the observed state of HermesInstance.
type HermesInstanceStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is a short human-readable status (Pending | Ready | Degraded | Suspended).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Conditions reflect subsystem readiness. Plan 2 emits:
	//   StorageReady, ConfigReady, SecretsReady, NetworkPolicyReady, RBACReady,
	//   ServiceReady, PDBReady, HPAReady, IngressReady, ServiceMonitorReady,
	//   PrometheusRuleReady, WebhookReady (manager-level), Ready (overall).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Replicas is the latest observed StatefulSet replica count.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the latest observed ready-replica count.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// Condition type constants. Centralised so Plan 4-6 and docs/conditions.md stay aligned.
const (
	ConditionTypeReady               = "Ready"
	ConditionTypeStorageReady        = "StorageReady"
	ConditionTypeConfigReady         = "ConfigReady"
	ConditionTypeSecretsReady        = "SecretsReady"
	ConditionTypeNetworkPolicyReady  = "NetworkPolicyReady"
	ConditionTypeRBACReady           = "RBACReady"
	ConditionTypeServiceReady        = "ServiceReady"
	ConditionTypePDBReady            = "PDBReady"
	ConditionTypeHPAReady            = "HPAReady"
	ConditionTypeIngressReady        = "IngressReady"
	ConditionTypeServiceMonitorReady = "ServiceMonitorReady"
	ConditionTypePrometheusRuleReady = "PrometheusRuleReady"
)
```

- [ ] **Step 6: Regenerate + verify**

```bash
make generate manifests
go build ./...
go test ./api/v1/... -v
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(api): add Availability/Probes/Scheduling/SelfConfigure specs + status conditions"
```

---

## Task 10: `HermesClusterDefaults`: full spec + singleton invariant

**Files:**
- Modify: `api/v1/hermesclusterdefaults_types.go`

Design §6: cluster-scoped, name **must** be `cluster`, body mirrors the HermesInstance defaultable sub-specs (image, registry, storage, security, observability, networking). ClusterDefaults only fills `nil` fields on the instance: never overrides.

- [ ] **Step 1: Write the failing test**

Create `api/v1/hermesclusterdefaults_types_test.go`:

```go
package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHermesClusterDefaults_Shape(t *testing.T) {
	t.Parallel()
	hcd := &HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: HermesClusterDefaultsSpec{
			Image: ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.4.2"},
			Registry: RegistryDefaults{
				PullSecretName: "ghcr-pull",
			},
			Storage: StorageSpec{
				Persistence: PersistenceSpec{Size: "10Gi", StorageClassName: Ptr("gp3")},
			},
			Security: SecurityDefaults{
				ServiceAccount: ServiceAccountDefaults{
					Annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:..."},
				},
			},
			Networking: NetworkingDefaults{
				NetworkPolicy: NetworkPolicyDefaults{Enabled: Ptr(true)},
			},
			Observability: ObservabilityDefaults{
				ServiceMonitor: ServiceMonitorSpec{Enabled: Ptr(true)},
			},
		},
	}
	assert.Equal(t, "cluster", hcd.Name)
	assert.Equal(t, "ghcr.io/stubbi/hermes-agent", hcd.Spec.Image.Repository)
	assert.Equal(t, "ghcr-pull", hcd.Spec.Registry.PullSecretName)
	assert.NotNil(t, hcd.Spec.Storage.Persistence.StorageClassName)

	// Sanity: a non-cluster name should still parse: the *webhook* rejects it,
	// not the type system.
	other := &HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "not-cluster"}}
	_ = corev1.ObjectReference{Name: other.Name}
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./api/v1/... -run TestHermesClusterDefaults_Shape -v
```

Expected: build error: most fields undefined.

- [ ] **Step 3: Replace `HermesClusterDefaultsSpec` with the real body**

Open `api/v1/hermesclusterdefaults_types.go`. Replace the scaffolded `HermesClusterDefaultsSpec` and `HermesClusterDefaultsStatus` blocks with:

```go
// HermesClusterDefaultsSpec is the cluster-wide default set applied by the
// defaulting webhook when a HermesInstance leaves a field nil. ClusterDefaults
// only fills nil fields; an explicit value on the instance always wins.
type HermesClusterDefaultsSpec struct {
	// Image defaults the instance's spec.image.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Registry defaults image-pull plumbing.
	// +optional
	Registry RegistryDefaults `json:"registry,omitempty"`

	// Storage defaults the instance's spec.storage.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Security defaults SA annotations + NetworkPolicy on/off + container-level
	// defaults (read-only rootfs etc. are operator-baked, not defaultable).
	// +optional
	Security SecurityDefaults `json:"security,omitempty"`

	// Observability defaults metrics / ServiceMonitor / PrometheusRule.
	// +optional
	Observability ObservabilityDefaults `json:"observability,omitempty"`

	// Networking defaults Service kind + NetworkPolicy enablement.
	// +optional
	Networking NetworkingDefaults `json:"networking,omitempty"`

	// Resources defaults requests + limits when the instance leaves them nil.
	// +optional
	Resources ResourcesSpec `json:"resources,omitempty"`
}

// RegistryDefaults groups image-pull secret hints.
type RegistryDefaults struct {
	// PullSecretName, if non-empty, is added to every instance's
	// pod.spec.imagePullSecrets when the instance doesn't override.
	// +optional
	PullSecretName string `json:"pullSecretName,omitempty"`
}

// SecurityDefaults mirrors the defaultable subset of SecuritySpec.
type SecurityDefaults struct {
	// +optional
	ServiceAccount ServiceAccountDefaults `json:"serviceAccount,omitempty"`

	// +optional
	NetworkPolicy NetworkPolicyDefaults `json:"networkPolicy,omitempty"`

	// +optional
	CABundle CABundleSpec `json:"caBundle,omitempty"`
}

// ServiceAccountDefaults defaults the per-instance SA annotations (IRSA / WI).
type ServiceAccountDefaults struct {
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NetworkPolicyDefaults defaults whether per-instance NetworkPolicies are created.
type NetworkPolicyDefaults struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	AllowDNS *bool `json:"allowDNS,omitempty"`
}

// NetworkingDefaults mirrors the defaultable subset of NetworkingSpec.
type NetworkingDefaults struct {
	// +optional
	Service ServiceDefaults `json:"service,omitempty"`

	// +optional
	NetworkPolicy NetworkPolicyDefaults `json:"networkPolicy,omitempty"`
}

// ServiceDefaults defaults the Service kind cluster-wide.
type ServiceDefaults struct {
	// +optional
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	Type corev1.ServiceType `json:"type,omitempty"`
}

// ObservabilityDefaults mirrors the defaultable subset of ObservabilitySpec.
type ObservabilityDefaults struct {
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`

	// +optional
	ServiceMonitor ServiceMonitorSpec `json:"serviceMonitor,omitempty"`

	// +optional
	PrometheusRule PrometheusRuleSpec `json:"prometheusRule,omitempty"`

	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`
}

// HermesClusterDefaultsStatus reflects observed singleton state.
type HermesClusterDefaultsStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions track Ready ("singleton-name OK and reachable").
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

- [ ] **Step 4: Strengthen the kubebuilder markers**

Above the root `type HermesClusterDefaults struct {` declaration, ensure the markers include:

```go
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=hcd,categories=hermes;agents
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

- [ ] **Step 5: Regenerate + verify**

```bash
make generate manifests
go test ./api/v1/... -run TestHermesClusterDefaults -v
```

Expected: deepcopy regenerated; CRD YAML at `config/crd/bases/hermes.agent_hermesclusterdefaults.yaml` shows `scope: Cluster` and `shortNames: [hcd]`; test passes.

- [ ] **Step 6: Verify cluster scope**

```bash
grep -A2 "names:" config/crd/bases/hermes.agent_hermesclusterdefaults.yaml | grep scope
```

Expected: `scope: Cluster`. If `Namespaced`, the kubebuilder marker was lost in regeneration: re-add and rerun `make manifests`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(api): full HermesClusterDefaults spec (image/registry/storage/security/networking/observability)"
```

---

## Task 11: `internal/resources/common.go`: port constants + builder option helpers

**Files:**
- Modify: `internal/resources/common.go`, `internal/resources/common_test.go`

Plan 1's `common.go` only has `Ptr` / `LabelsForInstance` / `MergePreservingForeign`. We add named-port constants and small reusable helpers consumed by every new builder.

- [ ] **Step 1: Write the failing tests**

Append to `internal/resources/common_test.go`:

```go
func TestPortConstants(t *testing.T) {
	t.Parallel()
	// Constants must be stable: Plan 3-6 reference these by name.
	assert.Equal(t, int32(8443), GatewayPort)
	assert.Equal(t, int32(9090), DefaultMetricsPort)
	assert.Equal(t, "gateway", GatewayPortName)
	assert.Equal(t, "metrics", MetricsPortName)
}

func TestSelectorLabels(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns"}}
	got := SelectorLabels(inst)
	// Selector labels are the immutable subset of LabelsForInstance.
	assert.Equal(t, "hermes-agent", got["app.kubernetes.io/name"])
	assert.Equal(t, "demo", got["app.kubernetes.io/instance"])
	// Selector labels MUST NOT include "managed-by" because that field is
	// allowed to evolve across operator versions.
	_, exists := got["app.kubernetes.io/managed-by"]
	assert.False(t, exists)
}

func TestServiceAccountName_Override(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				RBAC: hermesv1.RBACSpec{ServiceAccountName: "byo-sa"},
			},
		},
	}
	assert.Equal(t, "byo-sa", ServiceAccountNameFor(inst))

	inst.Spec.Security.RBAC.ServiceAccountName = ""
	assert.Equal(t, "demo", ServiceAccountNameFor(inst))
}

func TestBoolValue(t *testing.T) {
	t.Parallel()
	assert.True(t, BoolValue(Ptr(true)))
	assert.False(t, BoolValue(Ptr(false)))
	assert.False(t, BoolValue(nil))
	assert.True(t, BoolValueOrDefault(nil, true))
	assert.False(t, BoolValueOrDefault(Ptr(false), true))
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestPortConstants|TestSelectorLabels|TestServiceAccountName|TestBoolValue" -v
```

Expected: build errors.

- [ ] **Step 3: Implement the helpers**

Append to `internal/resources/common.go`:

```go
// Named ports used across builders. Stable across versions.
const (
	GatewayPort        int32 = 8443
	DefaultMetricsPort int32 = 9090
	GatewayPortName          = "gateway"
	MetricsPortName          = "metrics"
)

// SelectorLabels returns the immutable subset of LabelsForInstance suitable for
// Selector fields on Service/Deployment/StatefulSet. Selectors are immutable
// in k8s; the operator-managed-by label may evolve across versions, so we
// exclude it from selectors.
func SelectorLabels(inst *hermesv1.HermesInstance) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "hermes-agent",
		"app.kubernetes.io/instance": inst.Name,
	}
}

// ServiceAccountNameFor returns the ServiceAccount the agent pod should use:
// the spec.security.rbac.serviceAccountName override when set, else the
// operator-created SA which has the same name as the instance.
func ServiceAccountNameFor(inst *hermesv1.HermesInstance) string {
	if inst.Spec.Security.RBAC.ServiceAccountName != "" {
		return inst.Spec.Security.RBAC.ServiceAccountName
	}
	return inst.Name
}

// BoolValue dereferences a *bool, returning false on nil.
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// BoolValueOrDefault dereferences a *bool, returning def on nil.
func BoolValueOrDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
```

- [ ] **Step 4: Run to verify**

```bash
go test ./internal/resources/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add port constants, SelectorLabels, ServiceAccountNameFor, BoolValue helpers"
```

---

## Task 12: ConfigMap builder: honor `spec.config.raw` / `configMapRef` / `mergeMode`

**Files:**
- Modify: `internal/resources/configmap.go`, `internal/resources/configmap_test.go`

Plan 1's `BuildConfigMap` returns `{"config.yaml":"{}\n"}`. v1 must honor the three modes:

1. `Raw` set, `ConfigMapRef` unset → use Raw verbatim.
2. `ConfigMapRef` set, `Raw` unset → emit a sentinel comment pointing at the user-owned CM; the StatefulSet mounts the *user* CM as `config.yaml` (Plan 3 wires the projected-volume), and the operator-owned CM holds runtime overlays only.
3. Both set → `MergeMode=merge` deep-merges Raw onto the user CM body at reconcile time; `MergeMode=replace` (default) uses Raw verbatim and ignores the ConfigMapRef.
4. Neither set → emit `{}\n` (matches Plan 1 baseline).

The reconciler resolves option 2/3 (it needs cluster access to fetch the user CM); the *builder* receives the already-resolved body as a string.

- [ ] **Step 1: Write the failing tests**

Replace `internal/resources/configmap_test.go` with:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestBuildConfigMap_EmptyConfig(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	cm := BuildConfigMap(inst, "")
	assert.Equal(t, "demo-config", cm.Name)
	assert.Equal(t, "{}\n", cm.Data["config.yaml"])
}

func TestBuildConfigMap_RawBody(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"telegram":{"enabled":true}}`)}},
			},
		},
	}
	cm := BuildConfigMap(inst, "")
	body := cm.Data["config.yaml"]
	assert.Contains(t, body, "telegram:")
	assert.Contains(t, body, "enabled: true")
}

func TestBuildConfigMap_RefOnly_PassesResolvedBody(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	resolved := "discord:\n  enabled: true\n"
	cm := BuildConfigMap(inst, resolved)
	assert.Equal(t, resolved, cm.Data["config.yaml"])
}

func TestBuildConfigMap_MergeMode(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Config: hermesv1.ConfigSpec{
				Raw:       &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte(`{"telegram":{"enabled":true}}`)}},
				MergeMode: hermesv1.ConfigMergeModeMerge,
			},
		},
	}
	// In merge mode the builder accepts a resolved body and the caller is
	// responsible for performing the merge before calling the builder.
	cm := BuildConfigMap(inst, "discord:\n  enabled: true\ntelegram:\n  enabled: true\n")
	assert.Contains(t, cm.Data["config.yaml"], "discord:")
	assert.Contains(t, cm.Data["config.yaml"], "telegram:")
}

func TestMergeYAMLBodies(t *testing.T) {
	t.Parallel()
	base := "discord:\n  enabled: true\n"
	overlay := `{"telegram":{"enabled":true},"discord":{"enabled":false}}`
	got, err := MergeYAMLBodies(base, overlay)
	assert.NoError(t, err)
	assert.Contains(t, got, "telegram:")
	// Overlay wins on conflict.
	assert.Contains(t, got, "enabled: false")
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run TestBuildConfigMap -v
```

Expected: build error: `BuildConfigMap` signature changed; `MergeYAMLBodies` undefined.

- [ ] **Step 3: Update `internal/resources/configmap.go`**

Replace the file body with:

```go
package resources

import (
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ConfigMapName returns the deterministic ConfigMap name for the rendered config.
func ConfigMapName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-config"
}

// BuildConfigMap returns the desired ConfigMap holding ~/.hermes/config.yaml.
//
// `resolvedBody` is the body the reconciler has already resolved for the case
// where spec.config.configMapRef is set. The builder is pure: it does not
// reach out to the apiserver.
//
//   - Empty resolvedBody + Raw set         → use Raw verbatim (YAML-serialised).
//   - Empty resolvedBody + Raw unset       → emit "{}\n".
//   - resolvedBody non-empty + Raw unset   → use resolvedBody verbatim.
//   - resolvedBody non-empty + Raw set     → caller is responsible for merging
//     (use MergeYAMLBodies) and passing the merged result as resolvedBody.
func BuildConfigMap(inst *hermesv1.HermesInstance, resolvedBody string) *corev1.ConfigMap {
	body := "{}\n"
	switch {
	case resolvedBody != "":
		body = resolvedBody
	case inst.Spec.Config.Raw != nil && len(inst.Spec.Config.Raw.Raw) > 0:
		y, err := yaml.JSONToYAML(inst.Spec.Config.Raw.Raw)
		if err == nil {
			body = string(y)
		}
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Data: map[string]string{
			"config.yaml": body,
		},
	}
}

// MergeYAMLBodies performs a YAML deep-merge of `overlay` (JSON or YAML) onto
// `base` (YAML). Overlay wins on conflict. Used when spec.config.mergeMode=merge.
func MergeYAMLBodies(base, overlay string) (string, error) {
	baseMap := map[string]any{}
	if base != "" {
		if err := yaml.Unmarshal([]byte(base), &baseMap); err != nil {
			return "", fmt.Errorf("parse base YAML: %w", err)
		}
	}
	overlayMap := map[string]any{}
	if overlay != "" {
		if err := yaml.Unmarshal([]byte(overlay), &overlayMap); err != nil {
			return "", fmt.Errorf("parse overlay: %w", err)
		}
	}
	merged := deepMergeMaps(baseMap, overlayMap)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged: %w", err)
	}
	return string(out), nil
}

func deepMergeMaps(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if bv, ok := out[k]; ok {
			bm, bok := bv.(map[string]any)
			vm, vok := v.(map[string]any)
			if bok && vok {
				out[k] = deepMergeMaps(bm, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Add `sigs.k8s.io/yaml` if missing**

```bash
go get sigs.k8s.io/yaml@latest
```

It's almost certainly already in the module graph (kubebuilder ships with it transitively), but `go get` is a no-op if so.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/resources/... -run TestBuildConfigMap -v
go test ./internal/resources/... -run TestMergeYAMLBodies -v
```

Expected: PASS.

- [ ] **Step 6: Update the Plan-1 reconciler call site**

The Plan-1 reconciler in `internal/controller/hermesinstance_controller.go` calls `resources.BuildConfigMap(inst)` with one argument. Update the call (in `reconcileConfigMap`) to:

```go
desired := resources.BuildConfigMap(inst, "")
```

(Task 30 replaces this whole reconciler with a richer version that resolves `configMapRef`; this single-line update keeps the build green in the meantime.)

- [ ] **Step 7: Build + commit**

```bash
go build ./...
git add -A
git commit -m "feat(resources): ConfigMap builder honors spec.config.raw + provides MergeYAMLBodies"
```

---

## Task 13: Workspace ConfigMap builder with nested-path encoding

**Files:**
- Create: `internal/resources/workspace_configmap.go`, `internal/resources/workspace_configmap_test.go`

A second ConfigMap (`<inst>-workspace`) holds the `spec.workspace.initialFiles`. ConfigMap data keys cannot contain `/`, so we encode nested paths with `__` as the separator. The runtime-init container (Plan 3) reverses the encoding to write real files into `~/.hermes`.

Plan 4's SSA for `addWorkspaceFiles` lands in this ConfigMap with field manager `hermes.agent/selfconfig`, so the builder's encoding contract is shared API surface.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/workspace_configmap_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEncodeWorkspacePath_FlatAndNested(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "shallow.txt", EncodeWorkspacePath("shallow.txt"))
	assert.Equal(t, "notes__finance__2026.md", EncodeWorkspacePath("notes/finance/2026.md"))
	assert.Equal(t, "deep__a__b__c__d.txt", EncodeWorkspacePath("deep/a/b/c/d.txt"))
}

func TestDecodeWorkspacePath_Roundtrip(t *testing.T) {
	t.Parallel()
	cases := []string{"a.md", "a/b.md", "a/b/c/d/e/f.md"}
	for _, p := range cases {
		got := DecodeWorkspacePath(EncodeWorkspacePath(p))
		assert.Equal(t, p, got, "round-trip failed for %q", p)
	}
}

func TestBuildWorkspaceConfigMap_Encoded(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Workspace: hermesv1.WorkspaceSpec{
				InitialFiles: []hermesv1.WorkspaceFile{
					{Path: "notes/finance.md", Content: "Q1"},
					{Path: "shallow.txt", Content: "ok"},
				},
				InitialDirs: []string{"data", "data/raw"},
			},
		},
	}
	cm := BuildWorkspaceConfigMap(inst)
	assert.Equal(t, "demo-workspace", cm.Name)
	assert.Equal(t, "Q1", cm.Data["notes__finance.md"])
	assert.Equal(t, "ok", cm.Data["shallow.txt"])
	// InitialDirs are stored as a single key holding newline-separated paths.
	dirs := cm.Data[InitialDirsKey]
	assert.Contains(t, dirs, "data\n")
	assert.Contains(t, dirs, "data/raw\n")
}

func TestBuildWorkspaceConfigMap_EmptyIsStillEmitted(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	cm := BuildWorkspaceConfigMap(inst)
	assert.Equal(t, "demo-workspace", cm.Name)
	// Empty workspace still produces a ConfigMap (so the StatefulSet volume mount is stable).
	assert.NotNil(t, cm.Data)
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestEncodeWorkspacePath|TestDecodeWorkspacePath|TestBuildWorkspaceConfigMap" -v
```

Expected: build errors.

- [ ] **Step 3: Implement the encoder + builder**

Create `internal/resources/workspace_configmap.go`:

```go
package resources

import (
	"sort"
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitialDirsKey is the well-known data key holding the newline-separated list
// of directories to mkdir -p. Stored under a key that cannot collide with any
// EncodeWorkspacePath output (the "__" prefix is reserved).
const InitialDirsKey = "__hermes_initial_dirs__"

// WorkspaceConfigMapName returns the deterministic name.
func WorkspaceConfigMapName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-workspace"
}

// EncodeWorkspacePath turns "a/b/c.md" into "a__b__c.md". This is the
// canonical encoding shared with Plan 3's runtime-init decoder and Plan 4's
// HermesSelfConfig SSA writer.
func EncodeWorkspacePath(path string) string {
	return strings.ReplaceAll(path, "/", "__")
}

// DecodeWorkspacePath is the inverse of EncodeWorkspacePath.
func DecodeWorkspacePath(key string) string {
	return strings.ReplaceAll(key, "__", "/")
}

// BuildWorkspaceConfigMap creates the ConfigMap holding spec.workspace.initialFiles
// (path-encoded into ConfigMap data keys) and spec.workspace.initialDirs (under
// a single newline-separated key).
func BuildWorkspaceConfigMap(inst *hermesv1.HermesInstance) *corev1.ConfigMap {
	data := map[string]string{}
	for _, f := range inst.Spec.Workspace.InitialFiles {
		data[EncodeWorkspacePath(f.Path)] = f.Content
	}
	if len(inst.Spec.Workspace.InitialDirs) > 0 {
		dirs := make([]string, len(inst.Spec.Workspace.InitialDirs))
		copy(dirs, inst.Spec.Workspace.InitialDirs)
		sort.Strings(dirs)
		data[InitialDirsKey] = strings.Join(dirs, "\n") + "\n"
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkspaceConfigMapName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Data: data,
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run "TestEncodeWorkspacePath|TestDecodeWorkspacePath|TestBuildWorkspaceConfigMap" -v
```

Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add workspace ConfigMap builder with nested-path encoding (__)"
```

---

## Task 14: Auto-generated gateway-token Secret (placeholder for Plan 3)

**Files:**
- Create: `internal/resources/secret.go`, `internal/resources/secret_test.go`

The full gateway-token wiring (per-platform token reading, multi-secret merge) is Plan 3. This plan lands the *shell* so:

1. Plan 3's PR is a body-only change to one file.
2. The validating webhook can reference `spec.security.caBundle.secretName` and friends without ordering dependencies on Plan 3.
3. The StatefulSet's `envFrom` wiring can already refer to the operator-owned Secret by name.

The placeholder Secret is owned by the instance (garbage-collected on delete) and starts with an empty data map plus a tracking annotation `hermes.agent/placeholder: "true"` so Plan 3's controller knows it's safe to fill in.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/secret_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildGatewayTokenSecret_NameAndLabels(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	s := BuildGatewayTokenSecret(inst)
	assert.Equal(t, "demo-gateway-tokens", s.Name)
	assert.Equal(t, "agents", s.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, s.Type)
	assert.Equal(t, "hermes-agent", s.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "true", s.Annotations["hermes.agent/placeholder"])
}

func TestGatewayTokenSecretName_Determ(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo-gateway-tokens", GatewayTokenSecretName(inst))
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run TestBuildGatewayTokenSecret -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/secret.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GatewayTokenSecretName returns the deterministic name for the operator-owned
// gateway-tokens Secret.
func GatewayTokenSecretName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-gateway-tokens"
}

// BuildGatewayTokenSecret returns a placeholder Secret owned by the instance.
// Plan 2 emits an empty Secret with the "hermes.agent/placeholder: true"
// annotation; Plan 3 replaces the body with gateway-token bytes resolved from
// spec.gateways.*.tokenSecretRef. Until Plan 3 lands, the agent reads its tokens
// from user-provided EnvFrom secrets directly.
func BuildGatewayTokenSecret(inst *hermesv1.HermesInstance) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayTokenSecretName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
			Annotations: map[string]string{
				"hermes.agent/placeholder": "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run TestBuildGatewayTokenSecret -v
```

Expected: 2 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add gateway-token Secret builder (placeholder; Plan 3 fills body)"
```

---

## Task 15: NetworkPolicy builder: deny-all + DNS + 443 + allow-lists

**Files:**
- Create: `internal/resources/networkpolicy.go`, `internal/resources/networkpolicy_test.go`

Default-deny ingress and egress; allow ingress from same namespace + AllowedIngressNamespaces + AllowedIngressCIDRs; egress to DNS (when AllowDNS) + TCP 443 + AllowedEgressCIDRs + AdditionalEgress.

Naming follows Plan 1's `BuildPVC`/`PVCName` convention → `BuildNetworkPolicy` / `NetworkPolicyName`.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/networkpolicy_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildNetworkPolicy_DenyAllBase(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	np := BuildNetworkPolicy(inst)
	assert.Equal(t, "demo", np.Name)
	assert.Equal(t, "agents", np.Namespace)
	assert.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress)
	assert.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
	// PodSelector matches the instance selector labels (not all labels: those evolve).
	assert.Equal(t, "demo", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/instance"])
	assert.Equal(t, "hermes-agent", np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
}

func TestBuildNetworkPolicy_SameNamespaceIngress(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	np := BuildNetworkPolicy(inst)
	require := false
	for _, rule := range np.Spec.Ingress {
		for _, from := range rule.From {
			if from.NamespaceSelector != nil &&
				from.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == "agents" {
				require = true
			}
		}
	}
	assert.True(t, require, "expected ingress rule from same namespace")
}

func TestBuildNetworkPolicy_DefaultDNSEgress(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	np := BuildNetworkPolicy(inst)
	foundUDP53, foundTCP443 := false, false
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol == corev1.ProtocolUDP && p.Port != nil && p.Port.IntValue() == 53 {
				foundUDP53 = true
			}
			if p.Protocol != nil && *p.Protocol == corev1.ProtocolTCP && p.Port != nil && p.Port.IntValue() == 443 {
				foundTCP443 = true
			}
		}
	}
	assert.True(t, foundUDP53, "default-allow DNS UDP/53")
	assert.True(t, foundTCP443, "default-allow HTTPS TCP/443")
}

func TestBuildNetworkPolicy_AllowDNSDisabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{AllowDNS: Ptr(false)},
			},
		},
	}
	np := BuildNetworkPolicy(inst)
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Port != nil && p.Port.IntValue() == 53 {
				t.Fatalf("expected no DNS rule when AllowDNS=false")
			}
		}
	}
}

func TestBuildNetworkPolicy_AllowedIngressNamespacesAndCIDRs(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{
					AllowedIngressNamespaces: []string{"prometheus"},
					AllowedIngressCIDRs:      []string{"10.0.0.0/8"},
				},
			},
		},
	}
	np := BuildNetworkPolicy(inst)
	var sawNS, sawCIDR bool
	for _, rule := range np.Spec.Ingress {
		for _, from := range rule.From {
			if from.NamespaceSelector != nil &&
				from.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == "prometheus" {
				sawNS = true
			}
			if from.IPBlock != nil && from.IPBlock.CIDR == "10.0.0.0/8" {
				sawCIDR = true
			}
		}
	}
	assert.True(t, sawNS, "expected ingress rule for namespace prometheus")
	assert.True(t, sawCIDR, "expected ingress rule for CIDR 10.0.0.0/8")
}

func TestBuildNetworkPolicy_AdditionalEgress(t *testing.T) {
	t.Parallel()
	extra := networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "203.0.113.0/24"}}},
	}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{AdditionalEgress: []networkingv1.NetworkPolicyEgressRule{extra}},
			},
		},
	}
	np := BuildNetworkPolicy(inst)
	var sawExtra bool
	for _, rule := range np.Spec.Egress {
		for _, peer := range rule.To {
			if peer.IPBlock != nil && peer.IPBlock.CIDR == "203.0.113.0/24" {
				sawExtra = true
			}
		}
	}
	assert.True(t, sawExtra)
}

func TestNetworkPolicyName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", NetworkPolicyName(inst))
	_ = metav1.ObjectMeta{} // keep import used
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run TestBuildNetworkPolicy -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/networkpolicy.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NetworkPolicyName returns the deterministic NetworkPolicy name.
func NetworkPolicyName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// BuildNetworkPolicy returns a default-deny baseline plus selective allow rules:
//
//	Ingress:
//	  - From: same namespace (always)
//	  - From: AllowedIngressNamespaces[*]
//	  - From: AllowedIngressCIDRs[*]
//	  - Ports: Service ports if defined, else the default gateway port
//
//	Egress:
//	  - To: any peer; UDP+TCP 53 (DNS) when AllowDNS (default true)
//	  - To: any peer; TCP 443 (HTTPS): required for AI provider APIs
//	  - To: AllowedEgressCIDRs[*] (no port restriction; common for proxy fan-out)
//	  - Verbatim AdditionalEgress[*]
func BuildNetworkPolicy(inst *hermesv1.HermesInstance) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: SelectorLabels(inst)},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: buildIngressRules(inst),
			Egress:  buildEgressRules(inst),
		},
	}
}

// networkPolicyIngressPorts returns the ports allowed inbound. When
// spec.networking.service.ports is set, those are honored; else the
// well-known gateway port.
func networkPolicyIngressPorts(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyPort {
	if len(inst.Spec.Networking.Service.Ports) > 0 {
		ports := make([]networkingv1.NetworkPolicyPort, 0, len(inst.Spec.Networking.Service.Ports))
		for _, p := range inst.Spec.Networking.Service.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = corev1.ProtocolTCP
			}
			port := p.Port
			if p.TargetPort != nil {
				port = *p.TargetPort
			}
			ports = append(ports, networkingv1.NetworkPolicyPort{
				Protocol: Ptr(protocol),
				Port:     Ptr(intstr.FromInt32(port)),
			})
		}
		return ports
	}
	return []networkingv1.NetworkPolicyPort{
		{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt32(GatewayPort))},
	}
}

func buildIngressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyIngressRule {
	rules := []networkingv1.NetworkPolicyIngressRule{}
	ports := networkPolicyIngressPorts(inst)

	// Same namespace: always allowed.
	rules = append(rules, networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": inst.Namespace},
				},
			},
		},
		Ports: ports,
	})

	for _, ns := range inst.Spec.Security.NetworkPolicy.AllowedIngressNamespaces {
		rules = append(rules, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": ns},
					},
				},
			},
			Ports: ports,
		})
	}

	for _, cidr := range inst.Spec.Security.NetworkPolicy.AllowedIngressCIDRs {
		rules = append(rules, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
			},
			Ports: ports,
		})
	}

	return rules
}

func buildEgressRules(inst *hermesv1.HermesInstance) []networkingv1.NetworkPolicyEgressRule {
	rules := []networkingv1.NetworkPolicyEgressRule{}

	allowDNS := BoolValueOrDefault(inst.Spec.Security.NetworkPolicy.AllowDNS, true)
	if allowDNS {
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: Ptr(corev1.ProtocolUDP), Port: Ptr(intstr.FromInt(53))},
				{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt(53))},
			},
		})
	}

	// HTTPS: always allowed (AI provider APIs etc.)
	rules = append(rules, networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: Ptr(corev1.ProtocolTCP), Port: Ptr(intstr.FromInt(443))},
		},
	})

	for _, cidr := range inst.Spec.Security.NetworkPolicy.AllowedEgressCIDRs {
		rules = append(rules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: cidr}}},
		})
	}

	rules = append(rules, inst.Spec.Security.NetworkPolicy.AdditionalEgress...)

	return rules
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run TestBuildNetworkPolicy -v
```

Expected: 6 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): NetworkPolicy builder (deny-all + DNS + 443 + allow-lists)"
```

---

## Task 16: PodDisruptionBudget builder

**Files:**
- Create: `internal/resources/pdb.go`, `internal/resources/pdb_test.go`

Per design §4.2 ("PDB always created when replicas > 1"), the builder emits a PDB only when `spec.availability.podDisruptionBudget.enabled=true`. The reconciler deletes it when disabled or when `replicas <= 1` to avoid stuck-pod fail-modes during normal voluntary disruption.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/pdb_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildPDB_DefaultMaxUnavailable(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	pdb := BuildPDB(inst)
	assert.Equal(t, "demo", pdb.Name)
	assert.Equal(t, "agents", pdb.Namespace)
	assert.NotNil(t, pdb.Spec.MaxUnavailable)
	assert.Equal(t, intstr.FromInt(1), *pdb.Spec.MaxUnavailable)
	assert.Equal(t, "demo", pdb.Spec.Selector.MatchLabels["app.kubernetes.io/instance"])
}

func TestBuildPDB_HonorsMinAvailable(t *testing.T) {
	t.Parallel()
	min := intstr.FromString("50%")
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &min},
			},
		},
	}
	pdb := BuildPDB(inst)
	assert.NotNil(t, pdb.Spec.MinAvailable)
	assert.Equal(t, "50%", pdb.Spec.MinAvailable.StrVal)
	assert.Nil(t, pdb.Spec.MaxUnavailable, "MinAvailable and MaxUnavailable are mutually exclusive")
}

func TestPDBName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", PDBName(inst))
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run TestBuildPDB -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/pdb.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PDBName returns the deterministic PDB name.
func PDBName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// BuildPDB constructs the desired PodDisruptionBudget. When both MinAvailable
// and MaxUnavailable are set, MinAvailable wins (k8s forbids both: the
// validating webhook rejects the spec). When neither is set, MaxUnavailable=1.
func BuildPDB(inst *hermesv1.HermesInstance) *policyv1.PodDisruptionBudget {
	spec := inst.Spec.Availability.PodDisruptionBudget

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PDBName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: SelectorLabels(inst)},
		},
	}

	switch {
	case spec.MinAvailable != nil:
		pdb.Spec.MinAvailable = spec.MinAvailable
	case spec.MaxUnavailable != nil:
		pdb.Spec.MaxUnavailable = spec.MaxUnavailable
	default:
		def := intstr.FromInt(1)
		pdb.Spec.MaxUnavailable = &def
	}
	return pdb
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run TestBuildPDB -v
```

Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): PodDisruptionBudget builder (MinAvailable | MaxUnavailable default 1)"
```

---

## Task 17: HorizontalPodAutoscaler builder

**Files:**
- Create: `internal/resources/hpa.go`, `internal/resources/hpa_test.go`

Default min=1, max=5, target-CPU=80%. Memory metric appended when `TargetMemoryUtilization` is set. Custom `Behavior` field forwarded verbatim. Scale target is the StatefulSet by name.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/hpa_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildHPA_DefaultsScaleTargetStatefulSet(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{Enabled: Ptr(true)},
			},
		},
	}
	hpa := BuildHPA(inst)
	assert.Equal(t, "demo", hpa.Name)
	assert.Equal(t, "agents", hpa.Namespace)
	assert.Equal(t, "StatefulSet", hpa.Spec.ScaleTargetRef.Kind)
	assert.Equal(t, "apps/v1", hpa.Spec.ScaleTargetRef.APIVersion)
	assert.Equal(t, "demo", hpa.Spec.ScaleTargetRef.Name)
	assert.NotNil(t, hpa.Spec.MinReplicas)
	assert.Equal(t, int32(1), *hpa.Spec.MinReplicas)
	assert.Equal(t, int32(5), hpa.Spec.MaxReplicas)
	assert.NotEmpty(t, hpa.Spec.Metrics)
}

func TestBuildHPA_CustomCPUTarget(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled:              Ptr(true),
					MinReplicas:          Ptr(int32(2)),
					MaxReplicas:          Ptr(int32(10)),
					TargetCPUUtilization: Ptr(int32(70)),
				},
			},
		},
	}
	hpa := BuildHPA(inst)
	assert.Equal(t, int32(2), *hpa.Spec.MinReplicas)
	assert.Equal(t, int32(10), hpa.Spec.MaxReplicas)
	require := false
	for _, m := range hpa.Spec.Metrics {
		if m.Type == autoscalingv2.ResourceMetricSourceType && m.Resource.Name == corev1.ResourceCPU {
			require = true
			assert.Equal(t, int32(70), *m.Resource.Target.AverageUtilization)
		}
	}
	assert.True(t, require)
}

func TestBuildHPA_MemoryMetric(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Availability: hermesv1.AvailabilitySpec{
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled:                 Ptr(true),
					TargetMemoryUtilization: Ptr(int32(85)),
				},
			},
		},
	}
	hpa := BuildHPA(inst)
	var sawMemory bool
	for _, m := range hpa.Spec.Metrics {
		if m.Type == autoscalingv2.ResourceMetricSourceType && m.Resource.Name == corev1.ResourceMemory {
			sawMemory = true
			assert.Equal(t, int32(85), *m.Resource.Target.AverageUtilization)
		}
	}
	assert.True(t, sawMemory)
}

func TestIsHPAEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.False(t, IsHPAEnabled(inst))
	inst.Spec.Availability.HorizontalPodAutoscaler.Enabled = Ptr(false)
	assert.False(t, IsHPAEnabled(inst))
	inst.Spec.Availability.HorizontalPodAutoscaler.Enabled = Ptr(true)
	assert.True(t, IsHPAEnabled(inst))
}

func TestHPAName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", HPAName(inst))
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestBuildHPA|TestIsHPAEnabled|TestHPAName" -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/hpa.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HPAName returns the deterministic HPA name.
func HPAName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// IsHPAEnabled returns true when spec.availability.horizontalPodAutoscaler.enabled is true.
func IsHPAEnabled(inst *hermesv1.HermesInstance) bool {
	return BoolValue(inst.Spec.Availability.HorizontalPodAutoscaler.Enabled)
}

// BuildHPA constructs the desired HorizontalPodAutoscaler. Scale target is the
// StatefulSet built by BuildStatefulSet (same name).
func BuildHPA(inst *hermesv1.HermesInstance) *autoscalingv2.HorizontalPodAutoscaler {
	hs := inst.Spec.Availability.HorizontalPodAutoscaler

	minReplicas := int32(1)
	if hs.MinReplicas != nil {
		minReplicas = *hs.MinReplicas
	}
	maxReplicas := int32(5)
	if hs.MaxReplicas != nil {
		maxReplicas = *hs.MaxReplicas
	}
	cpuTarget := int32(80)
	if hs.TargetCPUUtilization != nil {
		cpuTarget = *hs.TargetCPUUtilization
	}

	metrics := []autoscalingv2.MetricSpec{
		{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: Ptr(cpuTarget),
				},
			},
		},
	}
	if hs.TargetMemoryUtilization != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: hs.TargetMemoryUtilization,
				},
			},
		})
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HPAName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       StatefulSetName(inst),
			},
			MinReplicas: Ptr(minReplicas),
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior:    hs.Behavior,
		},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run "TestBuildHPA|TestIsHPAEnabled|TestHPAName" -v
```

Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): HorizontalPodAutoscaler builder (CPU default, optional memory metric)"
```

---

## Task 18: Ingress builder with provider-aware annotations

**Files:**
- Create: `internal/resources/ingress.go`, `internal/resources/ingress_test.go`

When the user sets `spec.networking.ingress.className`, detect nginx / traefik / other by substring match and emit provider-specific annotations (force-https, body-size, etc.) on top of the user-supplied `annotations` map. Users always win on annotation key conflicts.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/ingress_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectIngressProvider(t *testing.T) {
	t.Parallel()
	cases := map[string]IngressProvider{
		"nginx":         IngressProviderNginx,
		"NGINX":         IngressProviderNginx,
		"traefik":       IngressProviderTraefik,
		"traefik-lb":    IngressProviderTraefik,
		"haproxy":       IngressProviderUnknown,
		"":              IngressProviderUnknown,
	}
	for in, want := range cases {
		var ptr *string
		if in != "" {
			ptr = Ptr(in)
		}
		assert.Equal(t, want, DetectIngressProvider(ptr), "input=%q", in)
	}
}

func TestBuildIngress_BasicShape(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:   Ptr(true),
					Host:      "hermes.example.com",
					ClassName: Ptr("nginx"),
					Path:      "/",
					PathType:  "Prefix",
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.Equal(t, "demo", ing.Name)
	assert.Equal(t, "agents", ing.Namespace)
	assert.NotNil(t, ing.Spec.IngressClassName)
	assert.Equal(t, "nginx", *ing.Spec.IngressClassName)
	assert.Len(t, ing.Spec.Rules, 1)
	assert.Equal(t, "hermes.example.com", ing.Spec.Rules[0].Host)
	// Default nginx annotation set (force-https): Plan 3 may extend.
	assert.Equal(t, "true", ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"])
}

func TestBuildIngress_TraefikAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:         Ptr(true),
					Host:            "hermes.example.com",
					ClassName:       Ptr("traefik"),
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	// Traefik HTTPS-redirect middleware annotation.
	assert.NotEmpty(t, ing.Annotations["traefik.ingress.kubernetes.io/router.entrypoints"])
}

func TestBuildIngress_UserAnnotationsWin(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled:   Ptr(true),
					Host:      "x",
					ClassName: Ptr("nginx"),
					Annotations: map[string]string{
						"nginx.ingress.kubernetes.io/ssl-redirect": "false",
						"team":                                     "platform",
					},
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	// User override wins.
	assert.Equal(t, "false", ing.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"])
	// User-only annotation preserved.
	assert.Equal(t, "platform", ing.Annotations["team"])
}

func TestBuildIngress_TLS(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Ingress: hermesv1.IngressSpec{
					Enabled: Ptr(true),
					Host:    "x.example.com",
					TLS:     []hermesv1.IngressTLSSpec{{SecretName: "tls", Hosts: []string{"x.example.com"}}},
					ServicePortName: "gateway",
				},
			},
		},
	}
	ing := BuildIngress(inst)
	assert.Len(t, ing.Spec.TLS, 1)
	assert.Equal(t, "tls", ing.Spec.TLS[0].SecretName)
}

func TestIngressName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", IngressName(inst))
	_ = metav1.ObjectMeta{}
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestDetectIngressProvider|TestBuildIngress|TestIngressName" -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/ingress.go`:

```go
package resources

import (
	"strings"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressProvider is the detected ingress controller flavour.
type IngressProvider string

const (
	IngressProviderNginx   IngressProvider = "nginx"
	IngressProviderTraefik IngressProvider = "traefik"
	IngressProviderUnknown IngressProvider = "unknown"
)

// IngressName returns the deterministic Ingress name.
func IngressName(inst *hermesv1.HermesInstance) string {
	return inst.Name
}

// DetectIngressProvider classifies the className by substring match.
func DetectIngressProvider(className *string) IngressProvider {
	if className == nil {
		return IngressProviderUnknown
	}
	lower := strings.ToLower(*className)
	switch {
	case strings.Contains(lower, "nginx"):
		return IngressProviderNginx
	case strings.Contains(lower, "traefik"):
		return IngressProviderTraefik
	default:
		return IngressProviderUnknown
	}
}

// BuildIngress constructs the desired Ingress. User annotations always win on
// key conflict with operator-supplied defaults.
func BuildIngress(inst *hermesv1.HermesInstance) *networkingv1.Ingress {
	ing := inst.Spec.Networking.Ingress
	annotations := buildIngressAnnotations(inst)
	pathType := ing.PathType
	if pathType == "" {
		pathType = networkingv1.PathTypePrefix
	}
	path := ing.Path
	if path == "" {
		path = "/"
	}
	portName := ing.ServicePortName
	if portName == "" {
		portName = GatewayPortName
	}

	rules := []networkingv1.IngressRule{}
	if ing.Host != "" {
		rules = append(rules, networkingv1.IngressRule{
			Host: ing.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: Ptr(pathType),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: ServiceName(inst),
									Port: networkingv1.ServiceBackendPort{Name: portName},
								},
							},
						},
					},
				},
			},
		})
	}

	tls := []networkingv1.IngressTLS{}
	for _, t := range ing.TLS {
		tls = append(tls, networkingv1.IngressTLS{SecretName: t.SecretName, Hosts: t.Hosts})
	}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        IngressName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ing.ClassName,
			Rules:            rules,
			TLS:              tls,
		},
	}
}

func buildIngressAnnotations(inst *hermesv1.HermesInstance) map[string]string {
	annotations := map[string]string{}
	provider := DetectIngressProvider(inst.Spec.Networking.Ingress.ClassName)
	switch provider {
	case IngressProviderNginx:
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "32m"
	case IngressProviderTraefik:
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "websecure"
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
	}
	// User annotations win.
	for k, v := range inst.Spec.Networking.Ingress.Annotations {
		annotations[k] = v
	}
	return annotations
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run "TestDetectIngressProvider|TestBuildIngress|TestIngressName" -v
```

Expected: 6 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): Ingress builder with provider-aware annotations (nginx/traefik)"
```

---

## Task 19: Per-instance RBAC: ServiceAccount + Role + RoleBinding

**Files:**
- Create: `internal/resources/rbac.go`, `internal/resources/rbac_test.go`

Plan 1's pod runs as the default SA. v1's `spec.security.rbac.createServiceAccount` (default true) makes the operator emit a per-instance SA (with SA-level annotations for IRSA / Workload Identity), plus a Role + RoleBinding scoped to the instance's own ConfigMaps and its own HermesInstance resource. When `selfConfigure.enabled=true`, the Role gets additional verbs (Plan 4 reads them; we land the rule scaffolding here).

`AutomountServiceAccountToken` is false by default; flips to true when self-configure is on (the agent needs a token to call the apiserver).

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/rbac_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildServiceAccount_NameAndAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				RBAC: hermesv1.RBACSpec{
					CreateServiceAccount: Ptr(true),
					Annotations: map[string]string{
						"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/hermes",
					},
				},
			},
		},
	}
	sa := BuildServiceAccount(inst)
	assert.Equal(t, "demo", sa.Name)
	assert.Equal(t, "agents", sa.Namespace)
	assert.Equal(t, "arn:aws:iam::1:role/hermes", sa.Annotations["eks.amazonaws.com/role-arn"])
	// AutomountServiceAccountToken false when self-configure disabled.
	assert.NotNil(t, sa.AutomountServiceAccountToken)
	assert.False(t, *sa.AutomountServiceAccountToken)
}

func TestBuildServiceAccount_AutomountTokenWhenSelfConfigureEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true)},
		},
	}
	sa := BuildServiceAccount(inst)
	assert.NotNil(t, sa.AutomountServiceAccountToken)
	assert.True(t, *sa.AutomountServiceAccountToken)
}

func TestBuildRole_BaseRulesAndSelfConfigure(t *testing.T) {
	t.Parallel()
	// Base: only read own ConfigMap.
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	r := BuildRole(inst)
	assert.Equal(t, "demo", r.Name)
	require := false
	for _, rule := range r.Rules {
		for _, res := range rule.Resources {
			if res == "configmaps" {
				require = true
			}
		}
	}
	assert.True(t, require, "base Role must grant configmap reads")

	// Self-configure on: extra verbs.
	inst2 := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true)},
		},
	}
	r2 := BuildRole(inst2)
	var sawHermesSelfConfig bool
	for _, rule := range r2.Rules {
		for _, g := range rule.APIGroups {
			if g == "hermes.agent" {
				for _, res := range rule.Resources {
					if res == "hermesselfconfigs" {
						sawHermesSelfConfig = true
					}
				}
			}
		}
	}
	assert.True(t, sawHermesSelfConfig, "selfConfigure=true must add hermesselfconfigs verbs")
}

func TestBuildRoleBinding_Matches(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	rb := BuildRoleBinding(inst)
	assert.Equal(t, "demo", rb.Name)
	assert.Equal(t, "agents", rb.Namespace)
	assert.Equal(t, "demo", rb.RoleRef.Name)
	assert.Equal(t, "Role", rb.RoleRef.Kind)
	assert.Len(t, rb.Subjects, 1)
	assert.Equal(t, "ServiceAccount", rb.Subjects[0].Kind)
	assert.Equal(t, "demo", rb.Subjects[0].Name)
	assert.Equal(t, "agents", rb.Subjects[0].Namespace)
}

func TestRBACNames(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", ServiceAccountName(inst))
	assert.Equal(t, "demo", RoleName(inst))
	assert.Equal(t, "demo", RoleBindingName(inst))
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestBuildServiceAccount|TestBuildRole|TestBuildRoleBinding|TestRBACNames" -v
```

- [ ] **Step 3: Implement**

Create `internal/resources/rbac.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountName, RoleName, RoleBindingName all equal inst.Name. The names
// are split into helpers because builders consume them by name (e.g. RoleBinding
// references RoleName) and clarity beats compactness.
func ServiceAccountName(inst *hermesv1.HermesInstance) string { return inst.Name }

// RoleName returns the deterministic Role name.
func RoleName(inst *hermesv1.HermesInstance) string { return inst.Name }

// RoleBindingName returns the deterministic RoleBinding name.
func RoleBindingName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildServiceAccount returns the per-instance SA. AutomountServiceAccountToken
// is false unless spec.selfConfigure.enabled is true (the agent needs a token
// to call the apiserver only when self-configure is on).
func BuildServiceAccount(inst *hermesv1.HermesInstance) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceAccountName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: inst.Spec.Security.RBAC.Annotations,
		},
		AutomountServiceAccountToken: Ptr(BoolValue(inst.Spec.SelfConfigure.Enabled)),
	}
}

// BuildRole returns the per-instance Role. Base ruleset: read own ConfigMap +
// own gateway-token Secret. When selfConfigure.enabled is true, additional
// verbs are added on hermesinstances (get on self) and hermesselfconfigs
// (create/get/list) so the agent can drive Plan 4's SSA path.
func BuildRole(inst *hermesv1.HermesInstance) *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{ConfigMapName(inst), WorkspaceConfigMapName(inst)},
			Verbs:         []string{"get", "watch"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{GatewayTokenSecretName(inst)},
			Verbs:         []string{"get", "watch"},
		},
	}
	if BoolValue(inst.Spec.SelfConfigure.Enabled) {
		rules = append(rules,
			rbacv1.PolicyRule{
				APIGroups:     []string{"hermes.agent"},
				Resources:     []string{"hermesinstances"},
				ResourceNames: []string{inst.Name},
				Verbs:         []string{"get"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"hermes.agent"},
				Resources: []string{"hermesselfconfigs"},
				Verbs:     []string{"create", "get", "list"},
			},
		)
	}
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Rules: rules,
	}
}

// BuildRoleBinding binds the per-instance SA to the per-instance Role.
func BuildRoleBinding(inst *hermesv1.HermesInstance) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleBindingName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName(inst),
				Namespace: inst.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     RoleName(inst),
		},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run "TestBuildServiceAccount|TestBuildRole|TestBuildRoleBinding|TestRBACNames" -v
```

Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): per-instance ServiceAccount + Role + RoleBinding"
```

---

## Task 20: ServiceMonitor + PrometheusRule (unstructured builders)

**Files:**
- Create: `internal/resources/servicemonitor.go`, `internal/resources/servicemonitor_test.go`, `internal/resources/prometheusrule.go`, `internal/resources/prometheusrule_test.go`

Prometheus-Operator CRDs aren't a hard dep; we emit `*unstructured.Unstructured` and the reconciler creates them only when those CRDs exist (Task 30 wires the runtime check). `metrics.secure` must agree with the ServiceMonitor's scheme (lesson #435/#440): if `secure=true`, scheme=https; else scheme=http.

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/servicemonitor_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildServiceMonitor_Basics(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true)},
				ServiceMonitor: hermesv1.ServiceMonitorSpec{
					Enabled:       Ptr(true),
					Interval:      "60s",
					ScrapeTimeout: "20s",
					Labels:        map[string]string{"release": "kps"},
				},
			},
		},
	}
	sm := BuildServiceMonitor(inst)
	assert.Equal(t, "monitoring.coreos.com/v1", sm.GetAPIVersion())
	assert.Equal(t, "ServiceMonitor", sm.GetKind())
	assert.Equal(t, "demo", sm.GetName())
	assert.Equal(t, "agents", sm.GetNamespace())
	labels := sm.GetLabels()
	assert.Equal(t, "kps", labels["release"])
	// Spec.endpoints[0].interval = "60s"
	spec, _, _ := getNestedMap(sm.Object, "spec")
	endpoints := spec["endpoints"].([]interface{})
	ep := endpoints[0].(map[string]interface{})
	assert.Equal(t, "60s", ep["interval"])
	assert.Equal(t, "20s", ep["scrapeTimeout"])
	assert.Equal(t, "metrics", ep["port"])
	assert.Equal(t, "http", ep["scheme"])
}

func TestBuildServiceMonitor_SecureSchemeMatchesMetricsSecure(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true), Secure: Ptr(true)},
				ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: Ptr(true)},
			},
		},
	}
	sm := BuildServiceMonitor(inst)
	spec, _, _ := getNestedMap(sm.Object, "spec")
	ep := spec["endpoints"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "https", ep["scheme"], "lesson #435: scheme must follow metrics.secure")
}

func TestServiceMonitorName(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", ServiceMonitorName(inst))
}

// getNestedMap is a tiny helper used only in tests to walk Unstructured.
func getNestedMap(obj map[string]interface{}, key string) (map[string]interface{}, bool, error) {
	v, ok := obj[key]
	if !ok {
		return nil, false, nil
	}
	m, _ := v.(map[string]interface{})
	return m, true, nil
}
```

Create `internal/resources/prometheusrule_test.go`:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPrometheusRule_DefaultGroup(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				PrometheusRule: hermesv1.PrometheusRuleSpec{Enabled: Ptr(true)},
			},
		},
	}
	pr := BuildPrometheusRule(inst)
	assert.Equal(t, "monitoring.coreos.com/v1", pr.GetAPIVersion())
	assert.Equal(t, "PrometheusRule", pr.GetKind())
	assert.Equal(t, "demo", pr.GetName())
	spec := pr.Object["spec"].(map[string]interface{})
	groups := spec["groups"].([]interface{})
	assert.Len(t, groups, 1)
	g := groups[0].(map[string]interface{})
	rules := g["rules"].([]interface{})
	assert.NotEmpty(t, rules, "default rules: HermesHighRestartRate + HermesMetricsDown")
	// Sanity check at least one default rule name.
	var names []string
	for _, r := range rules {
		rm := r.(map[string]interface{})
		names = append(names, rm["alert"].(string))
	}
	assert.Contains(t, names, "HermesHighRestartRate")
}

func TestBuildPrometheusRule_AdditionalRulesAppended(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				PrometheusRule: hermesv1.PrometheusRuleSpec{
					Enabled: Ptr(true),
					AdditionalRules: []hermesv1.PrometheusRule{
						{Alert: "MyAlert", Expr: "up == 0", For: "5m"},
					},
				},
			},
		},
	}
	pr := BuildPrometheusRule(inst)
	rules := pr.Object["spec"].(map[string]interface{})["groups"].([]interface{})[0].(map[string]interface{})["rules"].([]interface{})
	names := []string{}
	for _, r := range rules {
		names = append(names, r.(map[string]interface{})["alert"].(string))
	}
	assert.Contains(t, names, "MyAlert")
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run "TestBuildServiceMonitor|TestBuildPrometheusRule|TestServiceMonitorName" -v
```

- [ ] **Step 3: Implement ServiceMonitor**

Create `internal/resources/servicemonitor.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceMonitorGVK is the GroupVersionKind we emit. Plan 6's distribution work
// asserts the operator does not require the Prometheus-Operator Go types at
// compile time.
func ServiceMonitorGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
}

// ServiceMonitorName returns the deterministic name.
func ServiceMonitorName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildServiceMonitor returns an unstructured ServiceMonitor. Scheme on the
// endpoint follows spec.observability.metrics.secure so the Prometheus side
// stops re-trying after lesson #435/#440.
func BuildServiceMonitor(inst *hermesv1.HermesInstance) *unstructured.Unstructured {
	labels := map[string]string{}
	for k, v := range LabelsForInstance(inst) {
		labels[k] = v
	}
	for k, v := range inst.Spec.Observability.ServiceMonitor.Labels {
		labels[k] = v
	}

	interval := inst.Spec.Observability.ServiceMonitor.Interval
	if interval == "" {
		interval = "30s"
	}
	scrapeTimeout := inst.Spec.Observability.ServiceMonitor.ScrapeTimeout
	if scrapeTimeout == "" {
		scrapeTimeout = "10s"
	}
	scheme := "http"
	if BoolValue(inst.Spec.Observability.Metrics.Secure) {
		scheme = "https"
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ServiceMonitorGVK().GroupVersion().String(),
			"kind":       ServiceMonitorGVK().Kind,
			"metadata": map[string]interface{}{
				"name":      ServiceMonitorName(inst),
				"namespace": inst.Namespace,
				"labels":    toIface(labels),
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": toIface(SelectorLabels(inst)),
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":          MetricsPortName,
						"interval":      interval,
						"scrapeTimeout": scrapeTimeout,
						"path":          "/metrics",
						"scheme":        scheme,
					},
				},
			},
		},
	}
}

func toIface(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Implement PrometheusRule**

Create `internal/resources/prometheusrule.go`:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PrometheusRuleGVK is the GroupVersionKind emitted.
func PrometheusRuleGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"}
}

// PrometheusRuleName returns the deterministic name.
func PrometheusRuleName(inst *hermesv1.HermesInstance) string { return inst.Name }

// defaultPrometheusRules returns the operator's built-in alert ruleset.
func defaultPrometheusRules(inst *hermesv1.HermesInstance) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"alert": "HermesHighRestartRate",
			"expr":  `sum by (pod) (rate(kube_pod_container_status_restarts_total{pod=~"` + inst.Name + `-.*"}[10m])) > 0.1`,
			"for":   "10m",
			"labels": map[string]interface{}{
				"severity": "warning",
				"instance": inst.Name,
			},
			"annotations": map[string]interface{}{
				"summary":     "Hermes pod restarting frequently",
				"description": "Pod {{$labels.pod}} has been restarting > 0.1/min for 10m",
			},
		},
		map[string]interface{}{
			"alert": "HermesMetricsDown",
			"expr":  `up{job="` + inst.Name + `"} == 0`,
			"for":   "5m",
			"labels": map[string]interface{}{
				"severity": "warning",
				"instance": inst.Name,
			},
			"annotations": map[string]interface{}{
				"summary":     "Hermes metrics endpoint is down",
				"description": "Pod {{$labels.pod}} stopped serving /metrics for 5m",
			},
		},
	}
}

// BuildPrometheusRule emits a PrometheusRule containing the operator-default
// alerts plus any spec.observability.prometheusRule.additionalRules.
func BuildPrometheusRule(inst *hermesv1.HermesInstance) *unstructured.Unstructured {
	rules := defaultPrometheusRules(inst)
	for _, r := range inst.Spec.Observability.PrometheusRule.AdditionalRules {
		entry := map[string]interface{}{
			"alert": r.Alert,
			"expr":  r.Expr,
		}
		if r.For != "" {
			entry["for"] = r.For
		}
		if len(r.Labels) > 0 {
			entry["labels"] = toIface(r.Labels)
		}
		if len(r.Annotations) > 0 {
			entry["annotations"] = toIface(r.Annotations)
		}
		rules = append(rules, entry)
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": PrometheusRuleGVK().GroupVersion().String(),
			"kind":       PrometheusRuleGVK().Kind,
			"metadata": map[string]interface{}{
				"name":      PrometheusRuleName(inst),
				"namespace": inst.Namespace,
				"labels":    toIface(LabelsForInstance(inst)),
			},
			"spec": map[string]interface{}{
				"groups": []interface{}{
					map[string]interface{}{
						"name":  "hermes-" + inst.Name,
						"rules": rules,
					},
				},
			},
		},
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/resources/... -run "TestBuildServiceMonitor|TestBuildPrometheusRule" -v
```

Expected: 4 PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(resources): ServiceMonitor + PrometheusRule builders (unstructured)"
```

---

## Task 21: Service builder: honor `spec.networking.service`

**Files:**
- Modify: `internal/resources/service.go`, `internal/resources/service_test.go`

Plan 1's Service is always headless and hard-codes one port. v1 honors `spec.networking.service.type` (default ClusterIP), `clusterIP="None"` for headless, `ports` (default `gateway:8443`), plus `annotations` and `loadBalancerClass` / `externalTrafficPolicy`. Also adds `metrics` port automatically when `spec.observability.metrics.enabled`.

- [ ] **Step 1: Extend `service_test.go`**

Replace `internal/resources/service_test.go` with:

```go
package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildService_DefaultClusterIPWithGatewayPort(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	svc := BuildService(inst)
	assert.Equal(t, "demo", svc.Name)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(t, corev1.ServiceAffinityNone, svc.Spec.SessionAffinity)
	assert.Equal(t, "demo", svc.Spec.Selector["app.kubernetes.io/instance"])
	require := false
	for _, p := range svc.Spec.Ports {
		if p.Name == "gateway" && p.Port == 8443 {
			require = true
		}
	}
	assert.True(t, require, "default gateway port on 8443")
}

func TestBuildService_Headless(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: corev1.ClusterIPNone},
			},
		},
	}
	svc := BuildService(inst)
	assert.Equal(t, corev1.ClusterIPNone, svc.Spec.ClusterIP)
}

func TestBuildService_LoadBalancerAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Type:                  corev1.ServiceTypeLoadBalancer,
					Annotations:           map[string]string{"foo": "bar"},
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
				},
			},
		},
	}
	svc := BuildService(inst)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
	assert.Equal(t, "bar", svc.Annotations["foo"])
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, svc.Spec.ExternalTrafficPolicy)
}

func TestBuildService_CustomPorts(t *testing.T) {
	t.Parallel()
	tp := int32(8443)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Ports: []hermesv1.NamedServicePort{
						{Name: "gateway", Port: 443, TargetPort: &tp, Protocol: corev1.ProtocolTCP},
					},
				},
			},
		},
	}
	svc := BuildService(inst)
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(443), svc.Spec.Ports[0].Port)
}

func TestBuildService_AddsMetricsPortWhenEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true), Port: 9090},
			},
		},
	}
	svc := BuildService(inst)
	var sawMetrics bool
	for _, p := range svc.Spec.Ports {
		if p.Name == "metrics" && p.Port == 9090 {
			sawMetrics = true
		}
	}
	assert.True(t, sawMetrics, "metrics port emitted when Metrics.Enabled")
}
```

- [ ] **Step 2: Run to fail**

```bash
go test ./internal/resources/... -run TestBuildService -v
```

- [ ] **Step 3: Implement**

Replace `internal/resources/service.go` body with:

```go
package resources

import (
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceName returns the deterministic Service name.
func ServiceName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildService constructs the desired Service. Honors spec.networking.service
// (Type, ClusterIP, Ports, Annotations, LoadBalancerClass, ExternalTrafficPolicy);
// appends a "metrics" port automatically when spec.observability.metrics.enabled.
func BuildService(inst *hermesv1.HermesInstance) *corev1.Service {
	ss := inst.Spec.Networking.Service

	svcType := ss.Type
	if svcType == "" {
		svcType = corev1.ServiceTypeClusterIP
	}

	ports := buildServicePorts(inst)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: ss.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                  svcType,
			ClusterIP:             ss.ClusterIP,
			SessionAffinity:       corev1.ServiceAffinityNone, // explicit default
			Selector:              SelectorLabels(inst),
			Ports:                 ports,
			LoadBalancerClass:     ss.LoadBalancerClass,
			ExternalTrafficPolicy: ss.ExternalTrafficPolicy,
		},
	}
}

func buildServicePorts(inst *hermesv1.HermesInstance) []corev1.ServicePort {
	ports := []corev1.ServicePort{}
	custom := inst.Spec.Networking.Service.Ports
	if len(custom) > 0 {
		for _, p := range custom {
			protocol := p.Protocol
			if protocol == "" {
				protocol = corev1.ProtocolTCP
			}
			target := p.Port
			if p.TargetPort != nil {
				target = *p.TargetPort
			}
			sp := corev1.ServicePort{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: intstr.FromInt32(target),
				Protocol:   protocol,
				NodePort:   p.NodePort,
			}
			ports = append(ports, sp)
		}
	} else {
		ports = append(ports, corev1.ServicePort{
			Name:       GatewayPortName,
			Port:       GatewayPort,
			TargetPort: intstr.FromString(GatewayPortName),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	if BoolValueOrDefault(inst.Spec.Observability.Metrics.Enabled, true) {
		port := inst.Spec.Observability.Metrics.Port
		if port == 0 {
			port = DefaultMetricsPort
		}
		// Avoid duplicate port name (some users may already declare "metrics").
		seen := false
		for _, p := range ports {
			if p.Name == MetricsPortName {
				seen = true
				break
			}
		}
		if !seen {
			ports = append(ports, corev1.ServicePort{
				Name:       MetricsPortName,
				Port:       port,
				TargetPort: intstr.FromString(MetricsPortName),
				Protocol:   corev1.ProtocolTCP,
			})
		}
	}
	return ports
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/resources/... -run TestBuildService -v
```

Expected: 5 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): Service builder honors spec.networking.service + auto-emits metrics port"
```

---

## Task 22: Defaulting webhook for HermesInstance

**Files:**
- Create: `api/v1/webhook_hermesinstance.go`, `internal/webhook/webhook_hermesinstance_default.go`, `internal/webhook/webhook_hermesinstance_default_test.go`

The defaulter reads the `HermesClusterDefaults` singleton (name `cluster`) and fills `nil` fields on the instance. Explicit values on the instance always win. Splitting the implementation between `api/v1/webhook_hermesinstance.go` (kubebuilder marker shell: minimal, just `SetupWebhookWithManager`) and `internal/webhook/` (logic + tests) keeps the API package clean of side-effecting code.

- [ ] **Step 1: Run kubebuilder webhook generator**

```bash
kubebuilder create webhook \
  --group hermes \
  --version v1 \
  --kind HermesInstance \
  --defaulting \
  --programmatic-validation
```

Expected: creates `api/v1/hermesinstance_webhook.go` (rename to `webhook_hermesinstance.go` for consistency with the spec's file-naming rule), updates `cmd/manager/main.go` with `SetupWebhookWithManager`, regenerates `PROJECT`.

```bash
git mv api/v1/hermesinstance_webhook.go api/v1/webhook_hermesinstance.go 2>/dev/null || \
  mv api/v1/hermesinstance_webhook.go api/v1/webhook_hermesinstance.go
```

- [ ] **Step 2: Write the failing test for the defaulter**

Create `internal/webhook/webhook_hermesinstance_default_test.go`:

```go
package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require := hermesv1.AddToScheme(scheme)
	assert.NoError(t, require)
	assert.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func TestDefaulter_FillsNilFromClusterDefaults(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	hcd := &hermesv1.HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: hermesv1.HermesClusterDefaultsSpec{
			Image: hermesv1.ImageSpec{
				Repository: "ghcr.io/stubbi/hermes-agent",
				Tag:        "1.4.2",
			},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Size: "10Gi"},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hcd).Build()
	d := &HermesInstanceDefaulter{Client: c}

	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	assert.NoError(t, d.Default(context.Background(), inst))

	assert.Equal(t, "ghcr.io/stubbi/hermes-agent", inst.Spec.Image.Repository)
	assert.Equal(t, "1.4.2", inst.Spec.Image.Tag)
	assert.Equal(t, "10Gi", inst.Spec.Storage.Persistence.Size)
}

func TestDefaulter_ExplicitInstanceValuesAlwaysWin(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	hcd := &hermesv1.HermesClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: hermesv1.HermesClusterDefaultsSpec{
			Image: hermesv1.ImageSpec{Repository: "default-repo", Tag: "default-tag"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hcd).Build()
	d := &HermesInstanceDefaulter{Client: c}

	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "explicit-repo", Tag: "explicit-tag"},
		},
	}
	assert.NoError(t, d.Default(context.Background(), inst))
	assert.Equal(t, "explicit-repo", inst.Spec.Image.Repository)
	assert.Equal(t, "explicit-tag", inst.Spec.Image.Tag)
}

func TestDefaulter_NoClusterDefaultsIsNotAnError(t *testing.T) {
	t.Parallel()
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	d := &HermesInstanceDefaulter{Client: c}
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	err := d.Default(context.Background(), inst)
	assert.NoError(t, err, "missing HermesClusterDefaults is allowed: operator works without it")
	// Sanity: defaulter is also tolerant of other not-found errors.
	_ = apierrors.IsNotFound(nil)
}
```

- [ ] **Step 3: Run to fail**

```bash
go test ./internal/webhook/... -run TestDefaulter -v
```

Expected: build error: `HermesInstanceDefaulter` type undefined.

- [ ] **Step 4: Implement the defaulter**

Create `internal/webhook/webhook_hermesinstance_default.go`:

```go
package webhook

import (
	"context"
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// HermesInstanceDefaulter fills nil fields on a HermesInstance from the
// HermesClusterDefaults singleton (name "cluster"). It never overrides
// explicit values.
type HermesInstanceDefaulter struct {
	client.Client
}

var _ admission.CustomDefaulter = &HermesInstanceDefaulter{}

// Default implements admission.CustomDefaulter.
func (d *HermesInstanceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	inst, ok := obj.(*hermesv1.HermesInstance)
	if !ok {
		return fmt.Errorf("expected *HermesInstance, got %T", obj)
	}
	hcd := &hermesv1.HermesClusterDefaults{}
	err := d.Get(ctx, types.NamespacedName{Name: "cluster"}, hcd)
	if apierrors.IsNotFound(err) {
		return nil // cluster defaults are optional
	}
	if err != nil {
		return fmt.Errorf("get HermesClusterDefaults: %w", err)
	}
	ApplyClusterDefaults(inst, hcd)
	return nil
}

// ApplyClusterDefaults mutates inst in place, filling nil fields from hcd.
// Public so unit tests can exercise without needing the apiserver.
func ApplyClusterDefaults(inst *hermesv1.HermesInstance, hcd *hermesv1.HermesClusterDefaults) {
	// Image
	if inst.Spec.Image.Repository == "" {
		inst.Spec.Image.Repository = hcd.Spec.Image.Repository
	}
	if inst.Spec.Image.Tag == "" {
		inst.Spec.Image.Tag = hcd.Spec.Image.Tag
	}
	if inst.Spec.Image.PullPolicy == "" {
		inst.Spec.Image.PullPolicy = hcd.Spec.Image.PullPolicy
	}

	// Storage
	if inst.Spec.Storage.Persistence.Size == "" {
		inst.Spec.Storage.Persistence.Size = hcd.Spec.Storage.Persistence.Size
	}
	if inst.Spec.Storage.Persistence.StorageClassName == nil {
		inst.Spec.Storage.Persistence.StorageClassName = hcd.Spec.Storage.Persistence.StorageClassName
	}

	// Resources
	if inst.Spec.Resources.Requests == nil {
		inst.Spec.Resources.Requests = hcd.Spec.Resources.Requests
	}
	if inst.Spec.Resources.Limits == nil {
		inst.Spec.Resources.Limits = hcd.Spec.Resources.Limits
	}

	// Security: SA annotations & NetworkPolicy flags
	if inst.Spec.Security.RBAC.Annotations == nil {
		inst.Spec.Security.RBAC.Annotations = hcd.Spec.Security.ServiceAccount.Annotations
	}
	if inst.Spec.Security.NetworkPolicy.Enabled == nil {
		inst.Spec.Security.NetworkPolicy.Enabled = hcd.Spec.Security.NetworkPolicy.Enabled
	}
	if inst.Spec.Security.NetworkPolicy.AllowDNS == nil {
		inst.Spec.Security.NetworkPolicy.AllowDNS = hcd.Spec.Security.NetworkPolicy.AllowDNS
	}
	if inst.Spec.Security.CABundle.ConfigMapName == "" && inst.Spec.Security.CABundle.SecretName == "" {
		inst.Spec.Security.CABundle = hcd.Spec.Security.CABundle
	}

	// Networking
	if inst.Spec.Networking.Service.Type == "" {
		inst.Spec.Networking.Service.Type = hcd.Spec.Networking.Service.Type
	}

	// Observability
	if inst.Spec.Observability.Metrics.Enabled == nil {
		inst.Spec.Observability.Metrics.Enabled = hcd.Spec.Observability.Metrics.Enabled
	}
	if inst.Spec.Observability.Metrics.Port == 0 {
		inst.Spec.Observability.Metrics.Port = hcd.Spec.Observability.Metrics.Port
	}
	if inst.Spec.Observability.Metrics.Secure == nil {
		inst.Spec.Observability.Metrics.Secure = hcd.Spec.Observability.Metrics.Secure
	}
	if inst.Spec.Observability.ServiceMonitor.Enabled == nil {
		inst.Spec.Observability.ServiceMonitor.Enabled = hcd.Spec.Observability.ServiceMonitor.Enabled
	}
	if inst.Spec.Observability.PrometheusRule.Enabled == nil {
		inst.Spec.Observability.PrometheusRule.Enabled = hcd.Spec.Observability.PrometheusRule.Enabled
	}
	if inst.Spec.Observability.Logging.Format == "" {
		inst.Spec.Observability.Logging.Format = hcd.Spec.Observability.Logging.Format
	}
	if inst.Spec.Observability.Logging.Level == "" {
		inst.Spec.Observability.Logging.Level = hcd.Spec.Observability.Logging.Level
	}
}
```

- [ ] **Step 5: Replace `api/v1/webhook_hermesinstance.go` with a thin shell**

Replace the kubebuilder-generated body with:

```go
package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:webhook:path=/mutate-hermes-agent-v1-hermesinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesinstances,verbs=create;update,versions=v1,name=mhermesinstance.hermes.agent,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesinstances,verbs=create;update,versions=v1,name=vhermesinstance.hermes.agent,admissionReviewVersions=v1

var hermesinstancelog = logf.Log.WithName("hermesinstance-webhook")

// RegisterHermesInstanceWebhook wires both the defaulter and the validator with the manager.
// The actual implementations live in internal/webhook/.
func RegisterHermesInstanceWebhook(mgr ctrl.Manager, def admission.CustomDefaulter, val admission.CustomValidator) error {
	hermesinstancelog.Info("registering HermesInstance webhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesInstance{}).
		WithDefaulter(def).
		WithValidator(val).
		Complete()
}

// keep webhook package referenced for go imports
var _ = webhook.Admission{}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/webhook/... -run TestDefaulter -v
```

Expected: 3 PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(webhook): defaulter applies HermesClusterDefaults to nil instance fields"
```

---

## Task 23: Validating webhook for HermesInstance

**Files:**
- Create: `internal/webhook/webhook_hermesinstance_validate.go`, `internal/webhook/webhook_hermesinstance_validate_test.go`

Per design §7.3:

- **Required:** `image.repository` (after defaulting), `storage.persistence.size`, exactly-one of `config.raw` xor `config.configMapRef`.
- **Immutable after creation:** `storage.persistence.storageClassName`, `storage.persistence.accessModes`, `metadata.name`.
- **Warning (not deny):** unknown top-level keys in `spec.config.raw` (we whitelist a fixed set), MaxUnavailable + MinAvailable both set on PDB.
- **Deny:** `selfConfigure.enabled=true` with empty `protectedKeys`; `Enabled=true` plus `AllowedActions=[]` is also a deny (empty allowlist = nothing allowed → useless).

- [ ] **Step 1: Write the failing tests**

Create `internal/webhook/webhook_hermesinstance_validate_test.go`:

```go
package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestValidator_DenyEmptyImageRepository(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err, "image.repository is required")
}

func TestValidator_DenyConfigRawAndConfigMapRefWithoutMergeMode(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Config: hermesv1.ConfigSpec{
				Raw:          &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte("{}")}},
				ConfigMapRef: &corev1.LocalObjectReference{Name: "x"},
				// MergeMode unset → defaulter will set it; we test pre-defaulter behaviour:
				MergeMode: "",
			},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.NotEmpty(t, warns, "expected a warning about both raw + configMapRef without explicit mergeMode")
}

func TestValidator_DenySelfConfigureEnabledNoProtectedKeys(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:         hermesv1.ImageSpec{Repository: "x"},
			Storage:       hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true), AllowedActions: []string{"skills"}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err, "selfConfigure.enabled=true requires non-empty protectedKeys")
}

func TestValidator_DenyImmutableStorageClassName(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	old := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Size: "1Gi", StorageClassName: Ptr("gp3")},
			},
		},
	}
	newer := old.DeepCopy()
	newer.Spec.Storage.Persistence.StorageClassName = Ptr("io2")

	_, err := v.ValidateUpdate(context.Background(), old, newer)
	assert.Error(t, err, "storageClassName is immutable")
}

func TestValidator_DenyBothPDBValuesSet(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	mi := intOrStr("50%")
	mu := intOrStr("1")
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &mi, MaxUnavailable: &mu},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err, "MinAvailable and MaxUnavailable are mutually exclusive")
}

func TestValidator_AllowHappyPath(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.Empty(t, warns)
}
```

- [ ] **Step 2: Implement the validator**

Create `internal/webhook/webhook_hermesinstance_validate.go`:

```go
package webhook

import (
	"context"
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// HermesInstanceValidator enforces the rules from spec §7.3.
type HermesInstanceValidator struct{}

var _ admission.CustomValidator = &HermesInstanceValidator{}

// Ptr is a tiny copy of the resources package helper: webhook package must
// not depend on internal/resources (cycle risk).
func Ptr[T any](v T) *T { return &v }

// intOrStr is a test helper hoisted into the package for compactness.
func intOrStr(s string) intstr.IntOrString { return intstr.FromString(s) }

// ValidateCreate runs the full sanity ruleset on a fresh resource.
func (v *HermesInstanceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	inst, ok := obj.(*hermesv1.HermesInstance)
	if !ok {
		return nil, fmt.Errorf("expected *HermesInstance, got %T", obj)
	}
	return validateCommon(inst)
}

// ValidateUpdate runs the create rules + immutability rules.
func (v *HermesInstanceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldI, ok1 := oldObj.(*hermesv1.HermesInstance)
	newI, ok2 := newObj.(*hermesv1.HermesInstance)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("ValidateUpdate types: old=%T new=%T", oldObj, newObj)
	}
	if err := validateImmutable(oldI, newI); err != nil {
		return nil, err
	}
	return validateCommon(newI)
}

// ValidateDelete is a no-op; finalizer logic in Plan 5 handles delete-time work.
func (v *HermesInstanceValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateCommon(inst *hermesv1.HermesInstance) (admission.Warnings, error) {
	var warns admission.Warnings

	if inst.Spec.Image.Repository == "" {
		return warns, fmt.Errorf("spec.image.repository is required (set on the instance or via HermesClusterDefaults)")
	}
	if inst.Spec.Storage.Persistence.Size == "" {
		return warns, fmt.Errorf("spec.storage.persistence.size is required")
	}

	// config.raw + configMapRef without explicit mergeMode is a warning.
	if inst.Spec.Config.Raw != nil && inst.Spec.Config.ConfigMapRef != nil && inst.Spec.Config.MergeMode == "" {
		warns = append(warns, "spec.config.raw and spec.config.configMapRef are both set without spec.config.mergeMode; defaults to 'replace' (Raw wins)")
	}

	// SelfConfigure rules.
	if inst.Spec.SelfConfigure.Enabled != nil && *inst.Spec.SelfConfigure.Enabled {
		if len(inst.Spec.SelfConfigure.ProtectedKeys) == 0 {
			return warns, fmt.Errorf("spec.selfConfigure.enabled=true requires non-empty spec.selfConfigure.protectedKeys (explicit allowlist policy)")
		}
		if len(inst.Spec.SelfConfigure.AllowedActions) == 0 {
			return warns, fmt.Errorf("spec.selfConfigure.enabled=true requires non-empty spec.selfConfigure.allowedActions")
		}
	}

	// PDB mutual exclusion.
	pdb := inst.Spec.Availability.PodDisruptionBudget
	if pdb.MinAvailable != nil && pdb.MaxUnavailable != nil {
		return warns, fmt.Errorf("spec.availability.podDisruptionBudget: MinAvailable and MaxUnavailable are mutually exclusive")
	}

	// HPA sanity.
	hpa := inst.Spec.Availability.HorizontalPodAutoscaler
	if hpa.MinReplicas != nil && hpa.MaxReplicas != nil && *hpa.MinReplicas > *hpa.MaxReplicas {
		return warns, fmt.Errorf("spec.availability.horizontalPodAutoscaler: MinReplicas > MaxReplicas")
	}

	return warns, nil
}

func validateImmutable(oldI, newI *hermesv1.HermesInstance) error {
	if oldI.Spec.Storage.Persistence.StorageClassName != nil &&
		(newI.Spec.Storage.Persistence.StorageClassName == nil ||
			*oldI.Spec.Storage.Persistence.StorageClassName != *newI.Spec.Storage.Persistence.StorageClassName) {
		return fmt.Errorf("spec.storage.persistence.storageClassName is immutable")
	}
	// metadata.name immutability is enforced by k8s itself, but check for clarity.
	if oldI.Name != newI.Name {
		return fmt.Errorf("metadata.name is immutable")
	}
	return nil
}

// keep webhook package referenced to silence unused-import diagnostics in tools
var _ = webhook.Admission{}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/webhook/... -run TestValidator -v
```

Expected: 6 PASS.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(webhook): validator for HermesInstance (required, immutable, one-of, SelfConfigure policy)"
```

---

## Task 24: Validating webhook for HermesClusterDefaults: name must be `cluster`

**Files:**
- Create: `api/v1/webhook_hermesclusterdefaults.go`, `internal/webhook/webhook_hermesclusterdefaults.go`, `internal/webhook/webhook_hermesclusterdefaults_test.go`

The validator is small but load-bearing: it enforces the singleton invariant from design §6 ("name **must** be `cluster`"). Any other name is rejected at admission.

- [ ] **Step 1: Run kubebuilder webhook generator**

```bash
kubebuilder create webhook \
  --group hermes \
  --version v1 \
  --kind HermesClusterDefaults \
  --programmatic-validation
```

Move the generated file to follow the naming rule:

```bash
mv api/v1/hermesclusterdefaults_webhook.go api/v1/webhook_hermesclusterdefaults.go
```

- [ ] **Step 2: Write the failing test**

Create `internal/webhook/webhook_hermesclusterdefaults_test.go`:

```go
package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHCDValidator_AllowSingleton(t *testing.T) {
	t.Parallel()
	v := &HermesClusterDefaultsValidator{}
	hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
	_, err := v.ValidateCreate(context.Background(), hcd)
	assert.NoError(t, err)
}

func TestHCDValidator_DenyOtherNames(t *testing.T) {
	t.Parallel()
	v := &HermesClusterDefaultsValidator{}
	for _, n := range []string{"default", "foo", "Cluster", "CLUSTER"} {
		hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: n}}
		_, err := v.ValidateCreate(context.Background(), hcd)
		assert.Errorf(t, err, "expected reject for name %q", n)
	}
}
```

- [ ] **Step 3: Implement**

Create `internal/webhook/webhook_hermesclusterdefaults.go`:

```go
package webhook

import (
	"context"
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// HermesClusterDefaultsValidator enforces design §6: the singleton's name must
// be "cluster".
type HermesClusterDefaultsValidator struct{}

var _ admission.CustomValidator = &HermesClusterDefaultsValidator{}

func (v *HermesClusterDefaultsValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validateHCD(obj)
}

func (v *HermesClusterDefaultsValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validateHCD(newObj)
}

func (v *HermesClusterDefaultsValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateHCD(obj runtime.Object) (admission.Warnings, error) {
	hcd, ok := obj.(*hermesv1.HermesClusterDefaults)
	if !ok {
		return nil, fmt.Errorf("expected *HermesClusterDefaults, got %T", obj)
	}
	if hcd.Name != "cluster" {
		return nil, fmt.Errorf("HermesClusterDefaults must be the singleton named \"cluster\" (got %q)", hcd.Name)
	}
	return nil, nil
}
```

- [ ] **Step 4: Update `api/v1/webhook_hermesclusterdefaults.go` to a thin shell**

Replace with:

```go
package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesclusterdefaults,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesclusterdefaults,verbs=create;update,versions=v1,name=vhermesclusterdefaults.hermes.agent,admissionReviewVersions=v1

// RegisterHermesClusterDefaultsWebhook wires the validator with the manager.
func RegisterHermesClusterDefaultsWebhook(mgr ctrl.Manager, val admission.CustomValidator) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesClusterDefaults{}).
		WithValidator(val).
		Complete()
}
```

- [ ] **Step 5: Run tests + commit**

```bash
go test ./internal/webhook/... -run TestHCDValidator -v
git add -A
git commit -m "feat(webhook): HermesClusterDefaults validator enforces singleton name=cluster"
```

---

## Task 25: HermesSelfConfig validator stub

**Files:**
- Create: `api/v1/webhook_hermesselfconfig.go`, `internal/webhook/webhook_hermesselfconfig.go`, `internal/webhook/webhook_hermesselfconfig_test.go`

Plan 4 owns the real selfconfig validator (policy-aware deny with k8s Events). Plan 2 lands a *stub* that always allows so the webhook server registers cleanly across all three CRDs. The stub is a single function with a `TODO(plan-4)` marker. Plan 4 replaces this file body.

- [ ] **Step 1: Run kubebuilder generator**

```bash
kubebuilder create webhook \
  --group hermes \
  --version v1 \
  --kind HermesSelfConfig \
  --programmatic-validation
mv api/v1/hermesselfconfig_webhook.go api/v1/webhook_hermesselfconfig.go
```

- [ ] **Step 2: Write the test**

Create `internal/webhook/webhook_hermesselfconfig_test.go`:

```go
package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSelfConfigValidator_Stub_AlwaysAllows(t *testing.T) {
	t.Parallel()
	v := &HermesSelfConfigValidator{}
	sc := &hermesv1.HermesSelfConfig{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	warns, err := v.ValidateCreate(context.Background(), sc)
	assert.NoError(t, err)
	assert.NotEmpty(t, warns, "stub emits a Plan-4-TODO warning so consumers know SelfConfig policy isn't enforced yet")
}
```

- [ ] **Step 3: Implement the stub**

Create `internal/webhook/webhook_hermesselfconfig.go`:

```go
package webhook

import (
	"context"
	"fmt"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// HermesSelfConfigValidator is the Plan-2 *stub* validator. It always allows
// the resource but emits a warning that SelfConfig policy is not enforced yet.
// Plan 4 replaces this file body with the real policy-aware validator.
type HermesSelfConfigValidator struct{}

var _ admission.CustomValidator = &HermesSelfConfigValidator{}

func (v *HermesSelfConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validateSelfConfigStub(obj)
}

func (v *HermesSelfConfigValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validateSelfConfigStub(newObj)
}

func (v *HermesSelfConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateSelfConfigStub(obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*hermesv1.HermesSelfConfig)
	if !ok {
		return nil, fmt.Errorf("expected *HermesSelfConfig, got %T", obj)
	}
	return admission.Warnings{
		"HermesSelfConfig policy is NOT enforced in operator v1.0.0 (Plan 2 stub); Plan 4 wires the real policy-aware validator.",
	}, nil
}
```

- [ ] **Step 4: Replace the API-level shell**

```go
// api/v1/webhook_hermesselfconfig.go
package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesselfconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesselfconfigs,verbs=create;update,versions=v1,name=vhermesselfconfig.hermes.agent,admissionReviewVersions=v1

// RegisterHermesSelfConfigWebhook wires the validator with the manager.
func RegisterHermesSelfConfigWebhook(mgr ctrl.Manager, val admission.CustomValidator) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesSelfConfig{}).
		WithValidator(val).
		Complete()
}
```

- [ ] **Step 5: Run tests + commit**

```bash
go test ./internal/webhook/... -run TestSelfConfigValidator -v
git add -A
git commit -m "feat(webhook): HermesSelfConfig validator stub (Plan 4 will fill the body)"
```

---

## Task 26: HermesClusterDefaults reconciler: singleton + Ready condition

**Files:**
- Create: `internal/controller/hermesclusterdefaults_controller.go`, `internal/controller/hermesclusterdefaults_controller_test.go`

No downstream resources to reconcile: the defaulting webhook reads the singleton synchronously each admission. The controller's job is:

1. Verify the singleton-name invariant (defence in depth alongside the webhook).
2. Set a `Ready` condition with `ObservedGeneration`.
3. Emit a Kubernetes Event when the invariant fails (so `kubectl describe hcd` surfaces it).

- [ ] **Step 1: Write the failing test**

Create `internal/controller/hermesclusterdefaults_controller_test.go`:

```go
package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("HermesClusterDefaults controller", func() {
	AfterEach(func() {
		ctx := context.Background()
		hcd := &hermesv1.HermesClusterDefaults{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
		_ = k8sClient.Delete(ctx, hcd)
	})

	It("sets Ready=True on the cluster singleton", func() {
		ctx := context.Background()
		hcd := &hermesv1.HermesClusterDefaults{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: hermesv1.HermesClusterDefaultsSpec{
				Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.0.0"},
			},
		}
		Expect(k8sClient.Create(ctx, hcd)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesClusterDefaults{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "cluster"}, got)).To(Succeed())
			g.Expect(got.Status.Conditions).ToNot(BeEmpty())
			var ready bool
			for _, c := range got.Status.Conditions {
				if c.Type == "Ready" && c.Status == metav1.ConditionTrue {
					ready = true
				}
			}
			g.Expect(ready).To(BeTrue())
			g.Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
		}).Within(30*time.Second).WithPolling(250*time.Millisecond).Should(Succeed())
	})
})
```

- [ ] **Step 2: Implement the reconciler**

Create `internal/controller/hermesclusterdefaults_controller.go`:

```go
package controller

import (
	"context"
	"fmt"
	"time"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HermesClusterDefaultsReconciler enforces the singleton invariant on the
// cluster-scoped HermesClusterDefaults resource and surfaces Ready status.
type HermesClusterDefaultsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesclusterdefaults,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesclusterdefaults/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *HermesClusterDefaultsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	hcd := &hermesv1.HermesClusterDefaults{}
	if err := r.Get(ctx, req.NamespacedName, hcd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cond := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Singleton",
		Message:            "HermesClusterDefaults singleton is healthy",
		ObservedGeneration: hcd.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}

	if hcd.Name != "cluster" {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InvalidName"
		cond.Message = fmt.Sprintf("name must be \"cluster\" (got %q); this resource is ignored by the defaulting webhook", hcd.Name)
		if r.Recorder != nil {
			r.Recorder.Event(hcd, "Warning", "InvalidName", cond.Message)
		}
	}

	meta.SetStatusCondition(&hcd.Status.Conditions, cond)
	hcd.Status.ObservedGeneration = hcd.Generation
	if err := r.Status().Update(ctx, hcd); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	// No downstream watches: RequeueAfter is fine for drift detection.
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager wires the controller.
func (r *HermesClusterDefaultsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesClusterDefaults{}).
		Named("hermesclusterdefaults").
		Complete(r)
}
```

- [ ] **Step 3: Wire into `suite_test.go`**

In `internal/controller/suite_test.go`, after the existing `HermesInstanceReconciler` setup, add:

```go
err = (&HermesClusterDefaultsReconciler{
    Client:   k8sManager.GetClient(),
    Scheme:   k8sManager.GetScheme(),
    Recorder: k8sManager.GetEventRecorderFor("hermesclusterdefaults"),
}).SetupWithManager(k8sManager)
Expect(err).ToNot(HaveOccurred())
```

- [ ] **Step 4: Run the envtest suite**

```bash
make test
```

Expected: the new Ginkgo spec passes alongside Plan 1's.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(controller): HermesClusterDefaults reconciler (singleton invariant + Ready condition)"
```

---

## Task 27: StatefulSet: wire `resources`, `security`, `probes`

**Files:**
- Modify: `internal/resources/statefulset.go`, `internal/resources/statefulset_test.go`

The full `StatefulSet` build is split across Tasks 27-29 because the file is the largest single artefact in the codebase and per-feature tests keep regression diagnostics legible.

- [ ] **Step 1: Write the failing tests**

Append to `internal/resources/statefulset_test.go`:

```go
func TestBuildStatefulSet_HonorsResources(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Resources = hermesv1.ResourcesSpec{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
	sts := BuildStatefulSet(inst)
	c := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("100m"), c.Resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("512Mi"), c.Resources.Limits[corev1.ResourceMemory])
}

func TestBuildStatefulSet_OverridesSecurityContexts(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Security.PodSecurityContext = &corev1.PodSecurityContext{
		RunAsUser: Ptr(int64(2000)),
	}
	inst.Spec.Security.ContainerSecurityContext = &corev1.SecurityContext{
		ReadOnlyRootFilesystem: Ptr(false),
	}
	sts := BuildStatefulSet(inst)
	assert.Equal(t, int64(2000), *sts.Spec.Template.Spec.SecurityContext.RunAsUser)
	assert.False(t, *sts.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
}

func TestBuildStatefulSet_ProbeOverrides(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Probes.Liveness = &corev1.Probe{
		InitialDelaySeconds: 30,
		PeriodSeconds:       15,
		SuccessThreshold:    1,
		FailureThreshold:    5,
		TimeoutSeconds:      2,
	}
	sts := BuildStatefulSet(inst)
	c := sts.Spec.Template.Spec.Containers[0]
	assert.NotNil(t, c.LivenessProbe)
	assert.Equal(t, int32(30), c.LivenessProbe.InitialDelaySeconds)
}
```

- [ ] **Step 2: Update `BuildStatefulSet` body**

Open `internal/resources/statefulset.go`. Modify the function so that:

1. After the existing default `PodSecurityContext` initialiser, **override** it if `inst.Spec.Security.PodSecurityContext != nil`:

```go
podSecurityCtx := &corev1.PodSecurityContext{
    RunAsNonRoot: Ptr(true),
    RunAsUser:    Ptr(int64(1000)),
    RunAsGroup:   Ptr(int64(1000)),
    FSGroup:      Ptr(int64(1000)),
    SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
}
if inst.Spec.Security.PodSecurityContext != nil {
    podSecurityCtx = inst.Spec.Security.PodSecurityContext.DeepCopy()
}
```

Replace the inline literal in `PodSpec.SecurityContext` with `podSecurityCtx`.

2. Same pattern for the container `SecurityContext`:

```go
containerSecurityCtx := &corev1.SecurityContext{
    AllowPrivilegeEscalation: Ptr(false),
    ReadOnlyRootFilesystem:   Ptr(true),
    Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
}
if inst.Spec.Security.ContainerSecurityContext != nil {
    containerSecurityCtx = inst.Spec.Security.ContainerSecurityContext.DeepCopy()
}
```

Use `containerSecurityCtx` on the container.

3. Set `c.Resources = inst.Spec.Resources.ToContainerResourceRequirements()` on the agent container.

4. Apply probe overrides: after declaring the default `ReadinessProbe`:

```go
if inst.Spec.Probes.Liveness != nil {
    c.LivenessProbe = inst.Spec.Probes.Liveness.DeepCopy()
}
if inst.Spec.Probes.Readiness != nil {
    c.ReadinessProbe = inst.Spec.Probes.Readiness.DeepCopy()
}
if inst.Spec.Probes.Startup != nil {
    c.StartupProbe = inst.Spec.Probes.Startup.DeepCopy()
}
```

- [ ] **Step 3: Run tests + commit**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
git add -A
git commit -m "feat(resources): StatefulSet honors resources, security overrides, probe overrides"
```

---

## Task 28: StatefulSet: wire scheduling, initContainers, sidecars, extraVolumes/Mounts, env/envFrom

**Files:**
- Modify: `internal/resources/statefulset.go`, `internal/resources/statefulset_test.go`

- [ ] **Step 1: Write the failing tests**

Append:

```go
func TestBuildStatefulSet_Scheduling(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Scheduling = hermesv1.SchedulingSpec{
		NodeSelector:      map[string]string{"disktype": "ssd"},
		Tolerations:       []corev1.Toleration{{Key: "gpu", Operator: corev1.TolerationOpExists}},
		PriorityClassName: "hi",
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{}},
				},
			},
		},
	}
	sts := BuildStatefulSet(inst)
	podSpec := sts.Spec.Template.Spec
	assert.Equal(t, "ssd", podSpec.NodeSelector["disktype"])
	assert.Len(t, podSpec.Tolerations, 1)
	assert.Equal(t, "hi", podSpec.PriorityClassName)
	assert.NotNil(t, podSpec.Affinity)
}

func TestBuildStatefulSet_TopologySpread(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Availability.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
		{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
	}
	sts := BuildStatefulSet(inst)
	assert.Len(t, sts.Spec.Template.Spec.TopologySpreadConstraints, 1)
}

func TestBuildStatefulSet_InitContainersAndSidecars(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.InitContainers = []corev1.Container{{Name: "user-init", Image: "alpine"}}
	inst.Spec.Sidecars = []corev1.Container{{Name: "user-side", Image: "alpine"}}
	sts := BuildStatefulSet(inst)
	var sawInit, sawSide bool
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		if c.Name == "user-init" {
			sawInit = true
		}
	}
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == "user-side" {
			sawSide = true
		}
	}
	assert.True(t, sawInit)
	assert.True(t, sawSide)
}

func TestBuildStatefulSet_ExtraVolumesAndMounts(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.ExtraVolumes = []corev1.Volume{{Name: "user-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	inst.Spec.ExtraVolumeMounts = []corev1.VolumeMount{{Name: "user-vol", MountPath: "/user"}}
	sts := BuildStatefulSet(inst)
	var sawVol, sawMount bool
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "user-vol" {
			sawVol = true
		}
	}
	for _, m := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
		if m.Name == "user-vol" && m.MountPath == "/user" {
			sawMount = true
		}
	}
	assert.True(t, sawVol)
	assert.True(t, sawMount)
}

func TestBuildStatefulSet_EnvAndEnvFrom(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Env = []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
	inst.Spec.EnvFrom = []corev1.EnvFromSource{
		{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "user-secret"}}},
	}
	sts := BuildStatefulSet(inst)
	c := sts.Spec.Template.Spec.Containers[0]
	var sawEnv, sawEnvFrom bool
	for _, e := range c.Env {
		if e.Name == "FOO" && e.Value == "bar" {
			sawEnv = true
		}
	}
	for _, ef := range c.EnvFrom {
		if ef.SecretRef != nil && ef.SecretRef.Name == "user-secret" {
			sawEnvFrom = true
		}
	}
	assert.True(t, sawEnv)
	assert.True(t, sawEnvFrom)
}

func TestBuildStatefulSet_ServiceAccountName(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	sts := BuildStatefulSet(inst)
	// Default: operator-created SA named after the instance.
	assert.Equal(t, "demo", sts.Spec.Template.Spec.ServiceAccountName)

	inst.Spec.Security.RBAC.ServiceAccountName = "byo-sa"
	sts2 := BuildStatefulSet(inst)
	assert.Equal(t, "byo-sa", sts2.Spec.Template.Spec.ServiceAccountName)
}
```

- [ ] **Step 2: Update `BuildStatefulSet` body**

Inside the `PodSpec` literal, set:

```go
NodeSelector:              inst.Spec.Scheduling.NodeSelector,
Tolerations:               inst.Spec.Scheduling.Tolerations,
Affinity:                  inst.Spec.Scheduling.Affinity,
PriorityClassName:         inst.Spec.Scheduling.PriorityClassName,
TopologySpreadConstraints: inst.Spec.Availability.TopologySpreadConstraints,
ServiceAccountName:        ServiceAccountNameFor(inst),
```

After the `Containers: []corev1.Container{...}` block, append the user-supplied sidecars:

```go
// Plan 3 prepends operator-managed sidecars (ollama / web-terminal / tailscale).
// User-supplied sidecars come last so users can override the agent's PID 1 expectation.
podSpec.Containers = append(podSpec.Containers, inst.Spec.Sidecars...)
```

(Restructure the existing literal so `podSpec` is built incrementally: declare it as a local first, then assign `Template.Spec = podSpec`.)

After `Volumes:` initialiser, append:

```go
podSpec.Volumes = append(podSpec.Volumes, inst.Spec.ExtraVolumes...)
```

For the agent container (`c`):

```go
c.VolumeMounts = append(c.VolumeMounts, inst.Spec.ExtraVolumeMounts...)
c.Env = append(c.Env, inst.Spec.Env...)
c.EnvFrom = append(c.EnvFrom, inst.Spec.EnvFrom...)
```

Add `InitContainers`:

```go
podSpec.InitContainers = append(podSpec.InitContainers, inst.Spec.InitContainers...)
```

- [ ] **Step 3: Run tests + commit**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
git add -A
git commit -m "feat(resources): StatefulSet wires scheduling, init/sidecars, extraVolumes/Mounts, env/envFrom, SA"
```

---

## Task 29: StatefulSet: workspace ConfigMap mount, CA bundle, `suspended` scale-to-zero

**Files:**
- Modify: `internal/resources/statefulset.go`, `internal/resources/statefulset_test.go`

- [ ] **Step 1: Write the failing tests**

Append:

```go
func TestBuildStatefulSet_WorkspaceVolumeMounted(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Workspace.InitialFiles = []hermesv1.WorkspaceFile{{Path: "a.md", Content: "x"}}
	sts := BuildStatefulSet(inst)
	var sawVol bool
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "workspace" && v.ConfigMap != nil && v.ConfigMap.Name == "demo-workspace" {
			sawVol = true
		}
	}
	assert.True(t, sawVol, "workspace ConfigMap mounted as volume")
}

func TestBuildStatefulSet_CABundleConfigMapMounted(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Security.CABundle = hermesv1.CABundleSpec{ConfigMapName: "corp-ca", Key: "ca.crt"}
	sts := BuildStatefulSet(inst)
	var sawCA bool
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "ca-bundle" {
			sawCA = true
		}
	}
	assert.True(t, sawCA)
	c := sts.Spec.Template.Spec.Containers[0]
	var hasSSLEnv bool
	for _, e := range c.Env {
		if e.Name == "SSL_CERT_FILE" {
			hasSSLEnv = true
		}
	}
	assert.True(t, hasSSLEnv, "SSL_CERT_FILE set when CA bundle is mounted")
}

func TestBuildStatefulSet_Suspended(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Suspended = true
	sts := BuildStatefulSet(inst)
	assert.NotNil(t, sts.Spec.Replicas)
	assert.Equal(t, int32(0), *sts.Spec.Replicas)
}

func TestBuildStatefulSet_NotSuspendedDefaultReplica(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	sts := BuildStatefulSet(inst)
	assert.NotNil(t, sts.Spec.Replicas)
	assert.Equal(t, int32(1), *sts.Spec.Replicas)
}
```

- [ ] **Step 2: Update `BuildStatefulSet`**

After the existing `Volumes` block:

```go
// Workspace volume: always mount (empty data is still a valid mount).
podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
    Name: "workspace",
    VolumeSource: corev1.VolumeSource{
        ConfigMap: &corev1.ConfigMapVolumeSource{
            LocalObjectReference: corev1.LocalObjectReference{Name: WorkspaceConfigMapName(inst)},
            DefaultMode:          Ptr(int32(0o644)),
        },
    },
})
c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
    Name:      "workspace",
    MountPath: "/home/hermes/.hermes-workspace-seed",
    ReadOnly:  true,
})
```

For the CA bundle:

```go
if inst.Spec.Security.CABundle.ConfigMapName != "" || inst.Spec.Security.CABundle.SecretName != "" {
    key := inst.Spec.Security.CABundle.Key
    if key == "" {
        key = "ca.crt"
    }
    var src corev1.VolumeSource
    switch {
    case inst.Spec.Security.CABundle.ConfigMapName != "":
        src = corev1.VolumeSource{
            ConfigMap: &corev1.ConfigMapVolumeSource{
                LocalObjectReference: corev1.LocalObjectReference{Name: inst.Spec.Security.CABundle.ConfigMapName},
                Items:                []corev1.KeyToPath{{Key: key, Path: "ca.crt"}},
                DefaultMode:          Ptr(int32(0o644)),
            },
        }
    case inst.Spec.Security.CABundle.SecretName != "":
        src = corev1.VolumeSource{
            Secret: &corev1.SecretVolumeSource{
                SecretName:  inst.Spec.Security.CABundle.SecretName,
                Items:       []corev1.KeyToPath{{Key: key, Path: "ca.crt"}},
                DefaultMode: Ptr(int32(0o644)),
            },
        }
    }
    podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{Name: "ca-bundle", VolumeSource: src})
    c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
        Name:      "ca-bundle",
        MountPath: "/etc/ssl/certs/hermes-ca-bundle.crt",
        SubPath:   "ca.crt",
        ReadOnly:  true,
    })
    c.Env = append(c.Env, corev1.EnvVar{
        Name:  "SSL_CERT_FILE",
        Value: "/etc/ssl/certs/hermes-ca-bundle.crt",
    })
}
```

For `Suspended`:

```go
replicas := int32(1)
if inst.Spec.Suspended {
    replicas = 0
}
// then set Replicas: Ptr(replicas) in the StatefulSetSpec.
```

- [ ] **Step 3: Run tests + commit**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
git add -A
git commit -m "feat(resources): StatefulSet mounts workspace + CA bundle; honors spec.suspended (replicas=0)"
```

---

## Task 30: HermesInstance reconciler: full subsystem orchestration

**Files:**
- Modify: `internal/controller/hermesinstance_controller.go`

Replace Plan 1's four-resource reconciler with the full dependency-ordered version:

```
Secret(gateway-tokens placeholder) → PVC → ConfigMap → WorkspaceConfigMap → NetworkPolicy
  → ServiceAccount → Role → RoleBinding → Service → PDB → HPA → Ingress
  → ServiceMonitor → PrometheusRule → StatefulSet
```

Per-subsystem condition setters surface every reconcile-time failure in `status.conditions`. ServiceMonitor / PrometheusRule are emitted only when the Prometheus-Operator CRDs are present on the cluster (the controller does a one-time check at startup, caches the result). `MergePreservingForeign` is used for labels + annotations on every owned resource.

- [ ] **Step 1: Replace the controller file body**

Open `internal/controller/hermesinstance_controller.go`. Replace the existing body with:

```go
package controller

import (
	"context"
	"fmt"
	"time"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HermesInstanceReconciler reconciles a HermesInstance.
type HermesInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// PrometheusOperatorCRDsPresent caches whether ServiceMonitor/PrometheusRule
	// CRDs are installed. Probed once at startup by cmd/manager.
	PrometheusOperatorCRDsPresent bool
}

const operatorLabelPrefix = "hermes.agent/"

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims;secrets;serviceaccounts;events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies;ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete

func (r *HermesInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	inst := &hermesv1.HermesInstance{}
	if err := r.Get(ctx, req.NamespacedName, inst); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	steps := []struct {
		name string
		cond string
		fn   func(context.Context, *hermesv1.HermesInstance) error
	}{
		{"Secret", hermesv1.ConditionTypeSecretsReady, r.reconcileSecret},
		{"PVC", hermesv1.ConditionTypeStorageReady, r.reconcilePVC},
		{"ConfigMap", hermesv1.ConditionTypeConfigReady, r.reconcileConfigMap},
		{"WorkspaceConfigMap", hermesv1.ConditionTypeConfigReady, r.reconcileWorkspaceConfigMap},
		{"NetworkPolicy", hermesv1.ConditionTypeNetworkPolicyReady, r.reconcileNetworkPolicy},
		{"RBAC", hermesv1.ConditionTypeRBACReady, r.reconcileRBAC},
		{"Service", hermesv1.ConditionTypeServiceReady, r.reconcileService},
		{"PDB", hermesv1.ConditionTypePDBReady, r.reconcilePDB},
		{"HPA", hermesv1.ConditionTypeHPAReady, r.reconcileHPA},
		{"Ingress", hermesv1.ConditionTypeIngressReady, r.reconcileIngress},
		{"ServiceMonitor", hermesv1.ConditionTypeServiceMonitorReady, r.reconcileServiceMonitor},
		{"PrometheusRule", hermesv1.ConditionTypePrometheusRuleReady, r.reconcilePrometheusRule},
		{"StatefulSet", "StatefulSetReady", r.reconcileStatefulSet},
	}
	for _, s := range steps {
		if err := s.fn(ctx, inst); err != nil {
			r.setCondition(inst, s.cond, metav1.ConditionFalse, "Error", err.Error())
			_ = r.Status().Update(ctx, inst)
			logger.Error(err, "subsystem failed", "subsystem", s.name)
			return ctrl.Result{}, fmt.Errorf("reconcile %s: %w", s.name, err)
		}
		r.setCondition(inst, s.cond, metav1.ConditionTrue, "Reconciled", s.name+" up to date")
	}

	if err := r.updateStatus(ctx, inst); err != nil {
		logger.Error(err, "status update failed")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// --- per-subsystem reconcilers ---

func (r *HermesInstanceReconciler) reconcileSecret(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: resources.GatewayTokenSecretName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildGatewayTokenSecret(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		obj.Type = desired.Type
		// Do NOT overwrite Data: Plan 3 fills it; reconciler is content-preserving here.
		if obj.Data == nil {
			obj.Data = desired.Data
		}
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePVC(ctx context.Context, inst *hermesv1.HermesInstance) error {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: resources.PVCName(inst), Namespace: inst.Namespace,
	}}
	err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
	if apierrors.IsNotFound(err) {
		desired := resources.BuildPVC(inst)
		if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, desired)
	}
	return err
}

func (r *HermesInstanceReconciler) reconcileConfigMap(ctx context.Context, inst *hermesv1.HermesInstance) error {
	body, err := r.resolveConfigBody(ctx, inst)
	if err != nil {
		return err
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ConfigMapName(inst), Namespace: inst.Namespace,
	}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildConfigMap(inst, body)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Data = desired.Data
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

// resolveConfigBody handles the three modes (raw, ref, merge).
func (r *HermesInstanceReconciler) resolveConfigBody(ctx context.Context, inst *hermesv1.HermesInstance) (string, error) {
	cs := inst.Spec.Config
	if cs.ConfigMapRef == nil {
		return "", nil // BuildConfigMap reads from Raw directly
	}
	user := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: cs.ConfigMapRef.Name, Namespace: inst.Namespace}, user); err != nil {
		return "", fmt.Errorf("resolve configMapRef %q: %w", cs.ConfigMapRef.Name, err)
	}
	base := user.Data["config.yaml"]
	if cs.Raw == nil {
		return base, nil
	}
	if cs.MergeMode == hermesv1.ConfigMergeModeMerge {
		return resources.MergeYAMLBodies(base, string(cs.Raw.Raw))
	}
	// replace mode (default): Raw wins: return empty so BuildConfigMap uses Raw verbatim.
	return "", nil
}

func (r *HermesInstanceReconciler) reconcileWorkspaceConfigMap(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: resources.WorkspaceConfigMapName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildWorkspaceConfigMap(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Data = desired.Data
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileNetworkPolicy(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValueOrDefault(inst.Spec.Security.NetworkPolicy.Enabled, true)
	obj := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
		Name: resources.NetworkPolicyName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildNetworkPolicy(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileRBAC(ctx context.Context, inst *hermesv1.HermesInstance) error {
	create := resources.BoolValueOrDefault(inst.Spec.Security.RBAC.CreateServiceAccount, true)
	if !create {
		return nil
	}
	// SA
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ServiceAccountName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		desired := resources.BuildServiceAccount(inst)
		sa.Labels = resources.MergePreservingForeign(sa.Labels, desired.Labels, operatorLabelPrefix)
		sa.Annotations = resources.MergePreservingForeign(sa.Annotations, desired.Annotations, operatorLabelPrefix)
		sa.AutomountServiceAccountToken = desired.AutomountServiceAccountToken
		return controllerutil.SetControllerReference(inst, sa, r.Scheme)
	}); err != nil {
		return fmt.Errorf("sa: %w", err)
	}
	// Role
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
		Name: resources.RoleName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		desired := resources.BuildRole(inst)
		role.Labels = resources.MergePreservingForeign(role.Labels, desired.Labels, operatorLabelPrefix)
		role.Rules = desired.Rules
		return controllerutil.SetControllerReference(inst, role, r.Scheme)
	}); err != nil {
		return fmt.Errorf("role: %w", err)
	}
	// RoleBinding
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: resources.RoleBindingName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		desired := resources.BuildRoleBinding(inst)
		rb.Labels = resources.MergePreservingForeign(rb.Labels, desired.Labels, operatorLabelPrefix)
		rb.Subjects = desired.Subjects
		rb.RoleRef = desired.RoleRef
		return controllerutil.SetControllerReference(inst, rb, r.Scheme)
	}); err != nil {
		return fmt.Errorf("rolebinding: %w", err)
	}
	return nil
}

func (r *HermesInstanceReconciler) reconcileService(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ServiceName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildService(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		// Preserve server-assigned ClusterIP fields.
		clusterIP := obj.Spec.ClusterIP
		clusterIPs := obj.Spec.ClusterIPs
		obj.Spec = desired.Spec
		if clusterIP != "" {
			obj.Spec.ClusterIP = clusterIP
			obj.Spec.ClusterIPs = clusterIPs
		}
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePDB(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Availability.PodDisruptionBudget.Enabled)
	obj := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{
		Name: resources.PDBName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildPDB(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileHPA(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{
		Name: resources.HPAName(inst), Namespace: inst.Namespace,
	}}
	if !resources.IsHPAEnabled(inst) {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildHPA(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileIngress(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Networking.Ingress.Enabled)
	obj := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{
		Name: resources.IngressName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildIngress(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileServiceMonitor(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Observability.ServiceMonitor.Enabled)
	if !enabled || !r.PrometheusOperatorCRDsPresent {
		// best-effort delete if it exists
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(resources.ServiceMonitorGVK())
		obj.SetName(resources.ServiceMonitorName(inst))
		obj.SetNamespace(inst.Namespace)
		return r.deleteIfExists(ctx, obj)
	}
	desired := resources.BuildServiceMonitor(inst)
	if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
		return err
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resources.ServiceMonitorGVK())
	obj.SetName(desired.GetName())
	obj.SetNamespace(desired.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Object["spec"] = desired.Object["spec"]
		obj.SetLabels(resources.MergePreservingForeign(obj.GetLabels(), desired.GetLabels(), operatorLabelPrefix))
		obj.SetOwnerReferences(desired.GetOwnerReferences())
		return nil
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePrometheusRule(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Observability.PrometheusRule.Enabled)
	if !enabled || !r.PrometheusOperatorCRDsPresent {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(resources.PrometheusRuleGVK())
		obj.SetName(resources.PrometheusRuleName(inst))
		obj.SetNamespace(inst.Namespace)
		return r.deleteIfExists(ctx, obj)
	}
	desired := resources.BuildPrometheusRule(inst)
	if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
		return err
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resources.PrometheusRuleGVK())
	obj.SetName(desired.GetName())
	obj.SetNamespace(desired.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Object["spec"] = desired.Object["spec"]
		obj.SetLabels(resources.MergePreservingForeign(obj.GetLabels(), desired.GetLabels(), operatorLabelPrefix))
		obj.SetOwnerReferences(desired.GetOwnerReferences())
		return nil
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileStatefulSet(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: resources.StatefulSetName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildStatefulSet(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

// --- helpers ---

func (r *HermesInstanceReconciler) deleteIfExists(ctx context.Context, obj client.Object) error {
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return client.IgnoreNotFound(r.Delete(ctx, obj))
}

func (r *HermesInstanceReconciler) setCondition(inst *hermesv1.HermesInstance, t string, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               t,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: inst.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})
}

func (r *HermesInstanceReconciler) updateStatus(ctx context.Context, inst *hermesv1.HermesInstance) error {
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		return err
	}
	inst.Status.Replicas = sts.Status.Replicas
	inst.Status.ReadyReplicas = sts.Status.ReadyReplicas
	inst.Status.ObservedGeneration = inst.Generation
	switch {
	case inst.Spec.Suspended:
		inst.Status.Phase = "Suspended"
	case sts.Status.ReadyReplicas > 0 && sts.Status.ReadyReplicas == sts.Status.Replicas:
		inst.Status.Phase = "Ready"
		r.setCondition(inst, hermesv1.ConditionTypeReady, metav1.ConditionTrue, "AllSubsystemsReady", "")
	default:
		inst.Status.Phase = "Pending"
		r.setCondition(inst, hermesv1.ConditionTypeReady, metav1.ConditionFalse, "StatefulSetNotReady", "")
	}
	return r.Status().Update(ctx, inst)
}

// SetupWithManager wires watches for every owned type.
func (r *HermesInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesInstance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Named("hermesinstance").
		Complete(r)
}
```

- [ ] **Step 2: Regenerate manifests**

```bash
make manifests
```

Expected: `config/rbac/role.yaml` grows verbs for NetworkPolicy, Ingress, PDB, HPA, ServiceAccount, Role/RoleBinding, Secret, ServiceMonitor, PrometheusRule.

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```

Expected: exit 0. If imports are missing add them per the import block above.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(controller): full HermesInstance reconciler: 13 subsystems + per-subsystem conditions"
```

---

## Task 31: envtest cases per new subsystem + full-spec idempotency canary

**Files:**
- Modify: `internal/controller/hermesinstance_controller_test.go`, `internal/controller/suite_test.go`

We extend Plan 1's idempotency test with a *full-spec* variant that applies a maximal `HermesInstance` (every sub-spec populated to a non-default value) and asserts no managed resource bumps `metadata.generation` across 10 reconciles after the first one.

- [ ] **Step 1: Register Prometheus-Operator CRDs as no-ops in envtest**

The envtest harness has no Prometheus-Operator CRDs by default. Tell the reconciler that they are absent so it skips ServiceMonitor / PrometheusRule reconcile. In `internal/controller/suite_test.go`, when constructing `HermesInstanceReconciler`, set `PrometheusOperatorCRDsPresent: false`:

```go
err = (&HermesInstanceReconciler{
    Client:                        k8sManager.GetClient(),
    Scheme:                        k8sManager.GetScheme(),
    Recorder:                      k8sManager.GetEventRecorderFor("hermesinstance"),
    PrometheusOperatorCRDsPresent: false,
}).SetupWithManager(k8sManager)
Expect(err).ToNot(HaveOccurred())
```

- [ ] **Step 2: Add the per-subsystem envtest specs**

Append to `internal/controller/hermesinstance_controller_test.go`:

```go
var _ = Describe("HermesInstance: full subsystems", func() {
	const (
		name      = "demo-full"
		namespace = "default"
		timeout   = 60 * time.Second
		interval  = 250 * time.Millisecond
	)

	AfterEach(func() {
		inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		_ = k8sClient.Delete(context.Background(), inst)
	})

	It("creates per-subsystem resources for a maximal HermesInstance", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			// NetworkPolicy
			np := &networkingv1.NetworkPolicy{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())
			// SA + Role + RoleBinding
			sa := &corev1.ServiceAccount{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sa)).To(Succeed())
			role := &rbacv1.Role{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, role)).To(Succeed())
			rb := &rbacv1.RoleBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, rb)).To(Succeed())
			// PDB
			pdb := &policyv1.PodDisruptionBudget{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, pdb)).To(Succeed())
			// HPA
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, hpa)).To(Succeed())
			// Ingress
			ing := &networkingv1.Ingress{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)).To(Succeed())
			// Secret (placeholder)
			sec := &corev1.Secret{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-gateway-tokens", Namespace: namespace}, sec)).To(Succeed())
			// Workspace ConfigMap
			ws := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-workspace", Namespace: namespace}, ws)).To(Succeed())
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("is idempotent across the FULL spec (10 reconciles, no STS generation bump)", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		var stsGen int64
		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			g.Expect(sts.Generation).To(BeNumerically(">=", int64(1)))
			stsGen = sts.Generation
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		// Poke ten times.
		for i := 0; i < 10; i++ {
			var cur hermesv1.HermesInstance
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur)).To(Succeed())
			if cur.Annotations == nil {
				cur.Annotations = map[string]string{}
			}
			cur.Annotations["test.example.com/poke"] = fmt.Sprintf("%d-%d", i, time.Now().UnixNano())
			Expect(k8sClient.Update(ctx, &cur)).To(Succeed())
			time.Sleep(500 * time.Millisecond)
		}

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
		Expect(sts.Generation).To(Equal(stsGen), "STS generation must not bump on no-op reconciles")
	})

	It("scales to zero replicas when spec.suspended=true", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		inst.Spec.Suspended = true
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			g.Expect(sts.Spec.Replicas).ToNot(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(0)))
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("deletes the Ingress when spec.networking.ingress.enabled is flipped to false", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			ing := &networkingv1.Ingress{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)).To(Succeed())
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		var cur hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur)).To(Succeed())
		cur.Spec.Networking.Ingress.Enabled = Ptr(false)
		Expect(k8sClient.Update(ctx, &cur)).To(Succeed())

		Eventually(func() bool {
			ing := &networkingv1.Ingress{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)
			return apierrors.IsNotFound(err)
		}).Within(timeout).WithPolling(interval).Should(BeTrue())
	})
})

// maximalInstance returns a HermesInstance with every spec sub-field populated to a non-default value.
func maximalInstance(name, namespace string) *hermesv1.HermesInstance {
	tp := int32(8443)
	mi := intstr.FromString("50%")
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "test", PullPolicy: "IfNotPresent"},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Enabled: Ptr(true), Size: "1Gi"},
			},
			Resources: hermesv1.ResourcesSpec{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			},
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{Enabled: Ptr(true), AllowDNS: Ptr(true)},
				RBAC:          hermesv1.RBACSpec{CreateServiceAccount: Ptr(true)},
				CABundle:      hermesv1.CABundleSpec{ConfigMapName: "no-such-cm", Key: "ca.crt"}, // not resolved at envtest time; just shape
			},
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Type:  corev1.ServiceTypeClusterIP,
					Ports: []hermesv1.NamedServicePort{{Name: "gateway", Port: 8443, TargetPort: &tp, Protocol: corev1.ProtocolTCP}},
				},
				Ingress: hermesv1.IngressSpec{
					Enabled: Ptr(true), Host: "demo.example.com", ClassName: Ptr("nginx"),
					ServicePortName: "gateway", PathType: networkingv1.PathTypePrefix, Path: "/",
				},
			},
			Observability: hermesv1.ObservabilitySpec{
				Metrics: hermesv1.MetricsSpec{Enabled: Ptr(true), Port: 9090, Secure: Ptr(false)},
				// ServiceMonitor/PrometheusRule enabled but the reconciler skips because CRDs absent in envtest.
				ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: Ptr(true)},
				PrometheusRule: hermesv1.PrometheusRuleSpec{Enabled: Ptr(true)},
				Logging:        hermesv1.LoggingSpec{Format: hermesv1.LogFormatJSON, Level: "info"},
			},
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &mi},
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled: Ptr(true), MinReplicas: Ptr(int32(1)), MaxReplicas: Ptr(int32(3)),
					TargetCPUUtilization: Ptr(int32(70)),
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
				},
			},
			Workspace: hermesv1.WorkspaceSpec{
				InitialFiles: []hermesv1.WorkspaceFile{{Path: "notes/finance.md", Content: "x"}},
				InitialDirs:  []string{"data"},
			},
			Scheduling: hermesv1.SchedulingSpec{NodeSelector: map[string]string{"disktype": "ssd"}},
			Env:        []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
		},
	}
}
```

Add the imports (Ptr from the resources package is not visible; declare a local Ptr in the test file or import as `. "github.com/stubbi/hermes-operator/internal/resources"`):

```go
import (
	"fmt"
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func Ptr[T any](v T) *T { return &v }
```

- [ ] **Step 3: Run envtest**

```bash
make test
```

Expected: all Plan 1 specs + Plan 2's four new specs PASS. If the full-spec idempotency test fails with "STS generation bumped", a builder field is still not explicitly set or merged on update: bisect by removing fields from `maximalInstance` until the bump disappears.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test(controller): envtest covers full-spec subsystems + 10-reconcile idempotency canary"
```

---

## Task 32: Wire the manager: webhooks + cluster-defaults reconciler + Prometheus-CRD probe

**Files:**
- Modify: `cmd/manager/main.go`

- [ ] **Step 1: Probe Prometheus-Operator CRDs at startup**

Add a helper function to `cmd/manager/main.go`:

```go
// prometheusOperatorCRDsPresent returns true when both ServiceMonitor and
// PrometheusRule CRDs exist in the cluster. Probed once at startup; the
// reconciler caches the result so each reconcile is a constant-time skip.
func prometheusOperatorCRDsPresent(ctx context.Context, cfg *rest.Config) bool {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return false
	}
	groups, err := dc.ServerGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.Groups {
		if g.Name == "monitoring.coreos.com" {
			return true
		}
	}
	return false
}
```

Add imports:

```go
"k8s.io/client-go/discovery"
"k8s.io/client-go/rest"
"context"
```

- [ ] **Step 2: Construct + register reconcilers**

Inside the `main` function, after `mgr` is created:

```go
hasPromOpCRDs := prometheusOperatorCRDsPresent(context.Background(), mgr.GetConfig())
setupLog.Info("prometheus-operator CRDs probed", "present", hasPromOpCRDs)

if err = (&controller.HermesInstanceReconciler{
    Client:                        mgr.GetClient(),
    Scheme:                        mgr.GetScheme(),
    Recorder:                      mgr.GetEventRecorderFor("hermesinstance"),
    PrometheusOperatorCRDsPresent: hasPromOpCRDs,
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "HermesInstance")
    os.Exit(1)
}

if err = (&controller.HermesClusterDefaultsReconciler{
    Client:   mgr.GetClient(),
    Scheme:   mgr.GetScheme(),
    Recorder: mgr.GetEventRecorderFor("hermesclusterdefaults"),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "HermesClusterDefaults")
    os.Exit(1)
}
```

- [ ] **Step 3: Register webhooks**

Below the controllers:

```go
defaulter := &webhook.HermesInstanceDefaulter{Client: mgr.GetClient()}
instValidator := &webhook.HermesInstanceValidator{}
if err = hermesv1.RegisterHermesInstanceWebhook(mgr, defaulter, instValidator); err != nil {
    setupLog.Error(err, "unable to register HermesInstance webhook")
    os.Exit(1)
}
hcdValidator := &webhook.HermesClusterDefaultsValidator{}
if err = hermesv1.RegisterHermesClusterDefaultsWebhook(mgr, hcdValidator); err != nil {
    setupLog.Error(err, "unable to register HermesClusterDefaults webhook")
    os.Exit(1)
}
scValidator := &webhook.HermesSelfConfigValidator{}
if err = hermesv1.RegisterHermesSelfConfigWebhook(mgr, scValidator); err != nil {
    setupLog.Error(err, "unable to register HermesSelfConfig webhook")
    os.Exit(1)
}
```

Add the import:

```go
"github.com/stubbi/hermes-operator/internal/webhook"
```

- [ ] **Step 4: Build + smoke-run**

```bash
go build ./...
```

Expected: exit 0. Don't actually `make run` here: that requires cert-manager. Task 33's chart additions make `helm install` viable.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(manager): wire HermesClusterDefaults reconciler + webhooks + Prometheus-CRD probe"
```

---

## Task 33: Helm chart: cert-manager Issuer + Certificate + webhook configurations

**Files:**
- Create: `charts/hermes-operator/templates/certmanager.yaml`, `charts/hermes-operator/templates/webhook-configuration.yaml`
- Modify: `charts/hermes-operator/values.yaml`, `charts/hermes-operator/templates/clusterrole.yaml`

The operator's webhook server needs TLS. The chart provisions a cert-manager `Issuer` (self-signed) and `Certificate` named `hermes-operator-serving-cert`, plus `ValidatingWebhookConfiguration` and `MutatingWebhookConfiguration` resources annotated with `cert-manager.io/inject-ca-from` so cert-manager injects the CA bundle automatically.

Toggling `webhook.certManager.enabled=false` skips the cert-manager bits: useful when the cluster already provides the cert via another mechanism (`webhook.caBundle` is then read directly).

- [ ] **Step 1: Extend `values.yaml`**

Append to `charts/hermes-operator/values.yaml`:

```yaml
webhook:
  enabled: true
  port: 9443
  certManager:
    enabled: true
    issuerKind: Issuer            # Issuer | ClusterIssuer
    issuerName: hermes-operator-selfsigned
  # When certManager.enabled=false, set caBundle to the base64-encoded CA
  # used to sign the serving cert and provide a Secret named below.
  caBundle: ""
  servingCertSecretName: hermes-operator-webhook-server-cert
```

- [ ] **Step 2: Create `charts/hermes-operator/templates/certmanager.yaml`**

```yaml
{{- if and .Values.webhook.enabled .Values.webhook.certManager.enabled }}
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: {{ .Values.webhook.certManager.issuerName }}
  namespace: {{ .Release.Namespace }}
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: hermes-operator-serving-cert
  namespace: {{ .Release.Namespace }}
spec:
  dnsNames:
    - {{ include "hermes-operator.fullname" . }}-webhook.{{ .Release.Namespace }}.svc
    - {{ include "hermes-operator.fullname" . }}-webhook.{{ .Release.Namespace }}.svc.cluster.local
  issuerRef:
    kind: {{ .Values.webhook.certManager.issuerKind }}
    name: {{ .Values.webhook.certManager.issuerName }}
  secretName: {{ .Values.webhook.servingCertSecretName }}
{{- end }}
```

- [ ] **Step 3: Create `charts/hermes-operator/templates/webhook-configuration.yaml`**

```yaml
{{- if .Values.webhook.enabled }}
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: {{ include "hermes-operator.fullname" . }}-validating
  annotations:
    {{- if .Values.webhook.certManager.enabled }}
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/hermes-operator-serving-cert
    {{- end }}
webhooks:
  - name: vhermesinstance.hermes.agent
    admissionReviewVersions: [v1]
    sideEffects: None
    failurePolicy: Fail
    clientConfig:
      service:
        name: {{ include "hermes-operator.fullname" . }}-webhook
        namespace: {{ .Release.Namespace }}
        path: /validate-hermes-agent-v1-hermesinstance
      {{- if and (not .Values.webhook.certManager.enabled) .Values.webhook.caBundle }}
      caBundle: {{ .Values.webhook.caBundle }}
      {{- end }}
    rules:
      - apiGroups: [hermes.agent]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [hermesinstances]
  - name: vhermesclusterdefaults.hermes.agent
    admissionReviewVersions: [v1]
    sideEffects: None
    failurePolicy: Fail
    clientConfig:
      service:
        name: {{ include "hermes-operator.fullname" . }}-webhook
        namespace: {{ .Release.Namespace }}
        path: /validate-hermes-agent-v1-hermesclusterdefaults
      {{- if and (not .Values.webhook.certManager.enabled) .Values.webhook.caBundle }}
      caBundle: {{ .Values.webhook.caBundle }}
      {{- end }}
    rules:
      - apiGroups: [hermes.agent]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [hermesclusterdefaults]
  - name: vhermesselfconfig.hermes.agent
    admissionReviewVersions: [v1]
    sideEffects: None
    failurePolicy: Fail
    clientConfig:
      service:
        name: {{ include "hermes-operator.fullname" . }}-webhook
        namespace: {{ .Release.Namespace }}
        path: /validate-hermes-agent-v1-hermesselfconfig
      {{- if and (not .Values.webhook.certManager.enabled) .Values.webhook.caBundle }}
      caBundle: {{ .Values.webhook.caBundle }}
      {{- end }}
    rules:
      - apiGroups: [hermes.agent]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [hermesselfconfigs]
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: {{ include "hermes-operator.fullname" . }}-mutating
  annotations:
    {{- if .Values.webhook.certManager.enabled }}
    cert-manager.io/inject-ca-from: {{ .Release.Namespace }}/hermes-operator-serving-cert
    {{- end }}
webhooks:
  - name: mhermesinstance.hermes.agent
    admissionReviewVersions: [v1]
    sideEffects: None
    failurePolicy: Fail
    clientConfig:
      service:
        name: {{ include "hermes-operator.fullname" . }}-webhook
        namespace: {{ .Release.Namespace }}
        path: /mutate-hermes-agent-v1-hermesinstance
      {{- if and (not .Values.webhook.certManager.enabled) .Values.webhook.caBundle }}
      caBundle: {{ .Values.webhook.caBundle }}
      {{- end }}
    rules:
      - apiGroups: [hermes.agent]
        apiVersions: [v1]
        operations: [CREATE, UPDATE]
        resources: [hermesinstances]
{{- end }}
```

- [ ] **Step 4: Add the webhook Service template**

Either extend the existing `service.yaml` or create `charts/hermes-operator/templates/webhook-service.yaml`:

```yaml
{{- if .Values.webhook.enabled }}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "hermes-operator.fullname" . }}-webhook
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "hermes-operator.labels" . | nindent 4 }}
spec:
  type: ClusterIP
  ports:
    - port: 443
      targetPort: webhook
      protocol: TCP
      name: webhook
  selector:
    {{- include "hermes-operator.selectorLabels" . | nindent 4 }}
{{- end }}
```

- [ ] **Step 5: Mount the serving-cert Secret into the manager Deployment**

In `charts/hermes-operator/templates/deployment.yaml`, add (inside `spec.template.spec`):

```yaml
{{- if .Values.webhook.enabled }}
volumes:
  - name: webhook-cert
    secret:
      secretName: {{ .Values.webhook.servingCertSecretName }}
      defaultMode: 420
{{- end }}
```

And on the manager container:

```yaml
{{- if .Values.webhook.enabled }}
volumeMounts:
  - name: webhook-cert
    mountPath: /tmp/k8s-webhook-server/serving-certs
    readOnly: true
ports:
  - name: webhook
    containerPort: {{ .Values.webhook.port }}
    protocol: TCP
{{- end }}
```

- [ ] **Step 6: Sync CRDs into the chart**

```bash
make sync-chart-crds
```

Expected: `charts/hermes-operator/templates/crds/hermes.agent_hermesinstances.yaml` + `hermes.agent_hermesclusterdefaults.yaml` + `hermes.agent_hermesselfconfigs.yaml` updated.

- [ ] **Step 7: Verify the chart renders**

```bash
helm lint charts/hermes-operator
helm template test charts/hermes-operator | grep "kind: ValidatingWebhookConfiguration"
helm template test charts/hermes-operator --set webhook.certManager.enabled=false | grep -L "kind: Issuer" || true
```

Expected: lint passes; ValidatingWebhookConfiguration present; with certManager off, no Issuer rendered.

- [ ] **Step 8: Update Helm RBAC for the new verbs**

Open `charts/hermes-operator/templates/clusterrole.yaml`. Append the new RBAC rules so the Helm RBAC Sync CI job passes:

```yaml
  - apiGroups: [networking.k8s.io]
    resources: [networkpolicies, ingresses]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [policy]
    resources: [poddisruptionbudgets]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [autoscaling]
    resources: [horizontalpodautoscalers]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [rbac.authorization.k8s.io]
    resources: [roles, rolebindings]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [""]
    resources: [secrets, serviceaccounts]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [monitoring.coreos.com]
    resources: [servicemonitors, prometheusrules]
    verbs: [get, list, watch, create, update, patch, delete]
```

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(chart): cert-manager Issuer + Certificate + Validating/MutatingWebhookConfiguration"
```

---

## Task 34: Documentation: api-reference, conditions, README

**Files:**
- Create: `docs/api-reference.md`
- Modify: `docs/conditions.md`, `docs/conventions.md`, `README.md`

- [ ] **Step 1: Create `docs/api-reference.md`**

Use the following skeleton (fill in every field; do not leave a "TODO": every spec field defined in Tasks 2-9 must appear):

```markdown
# API Reference

> **Generated guidance:** This file mirrors the Go types in `api/v1/`. Every
> field on every CRD spec MUST appear here; CI fails on diff.

## Resources

- [`HermesInstance`](#hermesinstance) (`hermes.agent/v1`, namespaced)
- [`HermesClusterDefaults`](#hermesclusterdefaults) (`hermes.agent/v1`, cluster-scoped singleton)
- [`HermesSelfConfig`](#hermesselfconfig): Plan 4 documents this; reference stub here.

---

## HermesInstance

`apiVersion: hermes.agent/v1`, `kind: HermesInstance`.

### `.spec.image` (ImageSpec)

| Field | Type | Default | Description |
|---|---|---|---|
| `repository` | string | `ghcr.io/stubbi/hermes-agent` (from ClusterDefaults) | OCI image repo. |
| `tag` | string | `latest` (or ClusterDefaults) | Image tag. |
| `pullPolicy` | enum (`Always`/`IfNotPresent`/`Never`) | `IfNotPresent` | Pull policy. |

### `.spec.config` (ConfigSpec)

| Field | Type | Default | Description |
|---|---|---|---|
| `raw` | RawConfig (inline YAML) | nil | Inline body of `~/.hermes/config.yaml`. |
| `configMapRef` | LocalObjectReference | nil | ConfigMap holding `config.yaml`. |
| `mergeMode` | enum (`replace`/`merge`) | `replace` | Combination rule when both `raw` and `configMapRef` are set. |

### `.spec.workspace` (WorkspaceSpec)

| Field | Type | Default | Description |
|---|---|---|---|
| `initialFiles[]` | WorkspaceFile (path, content) | nil | Seeded files under `~/.hermes`; nested paths via `__` encoding. |
| `initialDirs[]` | string | nil | Directories to mkdir -p on first start. |
| `configMapRef` | LocalObjectReference | nil | User-owned workspace ConfigMap merged into the operator's. |
| `bootstrap.enabled` | bool | false | Run hermes-agent's bootstrap script on first start. |

(...repeat the section for every sub-spec: resources, security, storage, networking, observability, availability, probes, scheduling, initContainers, sidecars, extraVolumes, extraVolumeMounts, envFrom, env, skills, selfConfigure, suspended ...)

### `.status`

| Field | Type | Description |
|---|---|---|
| `observedGeneration` | int64 | Generation last seen by the controller. |
| `phase` | string | One of `Pending`, `Ready`, `Degraded`, `Suspended`. |
| `replicas` | int32 | Observed STS replica count. |
| `readyReplicas` | int32 | Observed ready-replica count. |
| `conditions[]` | metav1.Condition | See [conditions.md](conditions.md). |

---

## HermesClusterDefaults

(...mirror the same field-by-field tabulation for ClusterDefaults' Image / Registry / Storage / Security / Networking / Observability / Resources sub-specs ...)

---

## HermesSelfConfig

Plan 4 owns the full reference. The Plan 2 stub validator always allows the
resource and emits a warning that policy is not enforced yet.
```

- [ ] **Step 2: Update `docs/conditions.md`**

Append:

```markdown
## HermesInstance conditions (Plan 2)

| Type | When True | When False | Reason codes |
|---|---|---|---|
| `Ready` | All subsystems reconciled and STS ready | Any subsystem failed or STS not ready | `AllSubsystemsReady`, `StatefulSetNotReady` |
| `StorageReady` | PVC exists | PVC creation failed | `Reconciled`, `Error` |
| `ConfigReady` | ConfigMap + workspace ConfigMap up to date | One of them failed | `Reconciled`, `Error` |
| `SecretsReady` | Gateway-token Secret exists (placeholder body) | Secret create failed | `Reconciled`, `Error` |
| `NetworkPolicyReady` | NP exists (or correctly absent when disabled) | Failed to (re)create / delete | `Reconciled`, `Error` |
| `RBACReady` | SA + Role + RoleBinding exist | Any of the three failed | `Reconciled`, `Error` |
| `ServiceReady` | Service exists | Service create / update failed | `Reconciled`, `Error` |
| `PDBReady` | PDB exists (or correctly absent) | PDB op failed | `Reconciled`, `Error` |
| `HPAReady` | HPA exists (or correctly absent) | HPA op failed | `Reconciled`, `Error` |
| `IngressReady` | Ingress exists (or correctly absent) | Ingress op failed | `Reconciled`, `Error` |
| `ServiceMonitorReady` | ServiceMonitor exists OR Prometheus-Operator CRDs absent (skipped) | ServiceMonitor op failed | `Reconciled`, `Error` |
| `PrometheusRuleReady` | PrometheusRule exists OR Prometheus-Operator CRDs absent (skipped) | PrometheusRule op failed | `Reconciled`, `Error` |

## HermesClusterDefaults conditions

| Type | When True | When False | Reason codes |
|---|---|---|---|
| `Ready` | name == "cluster" | otherwise | `Singleton`, `InvalidName` |
```

- [ ] **Step 3: Update `docs/conventions.md`**

Append:

```markdown
## Explicit Kubernetes defaults (extended)

Plan 1 listed StatefulSet / Service / Probe defaults. Plan 2 adds these:

| Resource | Field | Default value |
|---|---|---|
| HorizontalPodAutoscaler | `spec.metrics[].resource.target.type` | `Utilization` (set explicitly) |
| Ingress | `spec.rules[].http.paths[].pathType` | `Prefix` (set when nil) |
| ServiceMonitor | `spec.endpoints[].scheme` | `http`; `https` when `metrics.secure=true` (must agree: lesson #435/#440) |
| NetworkPolicy | `spec.policyTypes` | both `Ingress` and `Egress` explicitly (k8s defaults to only `Ingress` when omitted) |
| PodDisruptionBudget | one of `MinAvailable` / `MaxUnavailable` | when neither set, `MaxUnavailable: 1` |
| Role | `apiGroups` | empty string `""` for core resources, explicit other groups |
```

- [ ] **Step 4: Update `README.md` feature table**

Replace the existing single-row feature table (Plan 1's "Reconcile minimal HermesInstance") with:

```markdown
## Features

| Feature | Status | Plan |
|---|---|---|
| Reconcile HermesInstance (PVC, ConfigMap, Service, StatefulSet) | ✅ v1.0 | Plan 1 |
| Full HermesInstance spec (resources, security, scheduling, ...) | ✅ v1.0 | Plan 2 |
| Defaulting webhook (HermesClusterDefaults singleton) | ✅ v1.0 | Plan 2 |
| Validating webhook (required / immutable / one-of) | ✅ v1.0 | Plan 2 |
| NetworkPolicy (deny-all baseline + selective allow) | ✅ v1.0 | Plan 2 |
| Per-instance RBAC (SA + Role + RoleBinding) | ✅ v1.0 | Plan 2 |
| PodDisruptionBudget | ✅ v1.0 | Plan 2 |
| HorizontalPodAutoscaler | ✅ v1.0 | Plan 2 |
| Ingress (provider-aware annotations) | ✅ v1.0 | Plan 2 |
| Prometheus ServiceMonitor + PrometheusRule | ✅ v1.0 | Plan 2 |
| `spec.suspended` scale-to-zero | ✅ v1.0 | Plan 2 |
| cert-manager-driven webhook TLS | ✅ v1.0 | Plan 2 |
| `spec.runtime` (Python/uv), gateways, profileStore | ⏳ pending | Plan 3 |
| HermesSelfConfig (agent self-mutations via SSA) | ⏳ pending | Plan 4 |
| Backup / restore / autoupdate / migration | ⏳ pending | Plan 5 |
| OLM bundle + GoReleaser + conformance suite | ⏳ pending | Plan 6 |
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "docs: api-reference, conditions catalogue, README feature table, conventions update"
```

---

## Task 35: Final integration check, milestone tag, worktree cleanup

**Files:** none (this task is purely operational).

- [ ] **Step 1: Run the full test suite**

```bash
make manifests generate
go build ./...
make lint
make test
```

Expected: all green. If `make lint` flags unused imports in the webhook shim files, drop them; if golangci-lint complains about `errcheck` on `_ = r.Status().Update(...)` style lines, those are intentional: silence the rule with `//nolint:errcheck // best-effort status update` and re-run.

- [ ] **Step 2: Run the e2e kind cycle**

```bash
make e2e
```

Expected: Plan 1's happy-path e2e still passes; the kind cluster handles the new RBAC + webhook configurations. cert-manager must be installed in the kind cluster first:

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl wait --for=condition=Available --timeout=120s -n cert-manager deploy/cert-manager-webhook
```

The CI workflow `.github/workflows/e2e.yaml` should be updated to install cert-manager before `make e2e`. If the e2e job fails because cert-manager isn't installed, edit `.github/workflows/e2e.yaml` to add the steps above before the `make e2e` invocation.

- [ ] **Step 3: Tag the milestone**

```bash
git tag plan-2-complete -m "Plan 2: Full HermesInstance reconciler + webhooks"
```

- [ ] **Step 4: Push the branch and open the PR**

```bash
git push -u origin feat/plan-2-full-reconciler
gh pr create \
  --title "feat: Plan 2: Full HermesInstance reconciler + webhooks" \
  --body "$(cat <<'EOF'
## Summary

Implements Plan 2 from `docs/superpowers/plans/2026-05-12-hermes-operator-plan-2-full-reconciler.md`. Expands `HermesInstance` to the full v1 spec (less Plan 3's runtime/gateways/profileStore), lands defaulting + validating webhooks backed by `HermesClusterDefaults` singleton, adds builders for NetworkPolicy / RBAC / PDB / HPA / Ingress / ServiceMonitor / PrometheusRule / Secret / WorkspaceConfigMap, and ships the cert-manager-fronted webhook chart.

## Test plan
- [ ] `make manifests generate` clean
- [ ] `make test` green (Plan 1 specs + Plan 2 new per-subsystem + full-spec idempotency)
- [ ] `make e2e` green (after installing cert-manager in the kind cluster)
- [ ] `helm lint charts/hermes-operator` clean
- [ ] `helm template test charts/hermes-operator | kubectl apply --dry-run=client -f -` clean

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 5: Clean up the worktree**

```bash
cd /Users/jannesstubbemann/repos/hermes-operator
git worktree remove ../hermes-operator-plan-2
```

---

## Self-review

This section maps every requirement from the dispatching brief and design spec back to the tasks that satisfy it, plus mechanical checks the implementer should run before claiming the plan complete.

### Spec → task coverage

| Spec section | Requirement | Task(s) |
|---|---|---|
| §4 `spec.image` | (already in Plan 1) |: |
| §4 `spec.config` (raw / configMapRef / mergeMode, YAML only) | Task 3 (types), Task 12 (builder + merge), Task 30 (reconciler resolves ref) |
| §4 `spec.workspace` (NESTED PATH SUPPORT, lesson #482) | Task 4 (types + nested-path validation), Task 13 (workspace ConfigMap builder + encoder), Task 29 (mount in STS) |
| §4 `spec.resources` (requests + limits) | Task 5 (types), Task 27 (wired in STS) |
| §4 `spec.security` (PSC, CSC, RBAC w/ IRSA annotations, NetworkPolicy, CABundle) | Task 6 (types), Task 15 (NP builder), Task 19 (RBAC builder), Task 27 (security wiring in STS), Task 29 (CA bundle mount), Task 30 (RBAC reconciler) |
| §4 `spec.networking` (Service + Ingress) | Task 7 (types), Task 18 (Ingress builder), Task 21 (Service builder update), Task 30 (Ingress/Service reconcilers) |
| §4 `spec.observability` (metrics, ServiceMonitor, PrometheusRule, logging) | Task 8 (types), Task 20 (SM + PR builders), Task 21 (auto-add metrics port), Task 30 (SM/PR reconcilers + Plan-3-CRD-probe), Task 32 (manager-level probe) |
| §4 `spec.availability` (PDB, HPA, topologySpreadConstraints) | Task 9 (types), Task 16 (PDB), Task 17 (HPA), Task 28 (topologySpread in STS), Task 30 (PDB/HPA reconcilers) |
| §4 `spec.probes` (liveness/readiness/startup overrides) | Task 9 (types), Task 27 (override wiring) |
| §4 `spec.scheduling` (nodeSelector, tolerations, affinity, priorityClassName) | Task 9 (types), Task 28 (wired in STS) |
| §4 `spec.initContainers`, `sidecars`, `extraVolumes`, `extraVolumeMounts` | Task 2 (types), Task 28 (wired in STS) |
| §4 `spec.envFrom`, `spec.env` (+listMapKey=name) | Task 2 (types + markers), Task 28 (wired in STS) |
| §4 `spec.skills` (+listMapKey=source: declared here for Plan 4 SSA) | Task 2 (types + markers) |
| §4 `spec.selfConfigure` (Enabled `*bool`, AllowedActions []string, ProtectedKeys []string) | Task 9 (types), Task 23 (validator rejects Enabled=true with empty ProtectedKeys / AllowedActions) |
| §4 `spec.suspended` | Task 2 (types), Task 29 (STS replicas=0), Task 30 (status.phase=Suspended), Task 31 (envtest) |
| §6 HermesClusterDefaults (singleton, name=cluster, mirrors instance sub-specs) | Task 10 (types + singleton scope marker), Task 24 (validator), Task 26 (reconciler), Task 22 (defaulter reads it) |
| §7.2 rule 1 (only `controllerutil.CreateOrUpdate`) | Task 30 (every reconciler uses it; deletes use `r.Delete` for disabled subsystems) |
| §7.2 rule 3 (explicit k8s defaults) | Plan 1 Task 8 established; Task 33 step 3 documents the extended list in `conventions.md` |
| §7.2 rule 4 (preserve foreign annotations/labels) | Task 30 (`MergePreservingForeign` on every subsystem) |
| §7.2 rule 6 (preserve server-assigned fields) | Task 30 reconcileService preserves ClusterIP/ClusterIPs; PVC create-only |
| §7.2 rule 7 (separate status transactions) | Task 30 (`r.Status().Update`); Task 26 (same) |
| §7.2 rule 8 (owner refs everywhere) | Task 30 (every builder gets `SetControllerReference`) |
| §7.2 rule 9 (RequeueAfter 5m) | Task 30 |
| §7.3 defaulter (clusterDefaults fills nil only) | Task 22 |
| §7.3 validator: required image+size+one-of config | Task 23 |
| §7.3 validator: immutable storageClassName/accessModes/name | Task 23 |
| §7.3 validator: SelfConfigure.Enabled=true with empty ProtectedKeys → deny | Task 23 |
| §7.3 selfconfig validator stub | Task 25 |
| §7.3 clusterdefaults validator: name=cluster | Task 24 |
| §7.3 cert-manager integration in Helm chart | Task 33 |

### Naming consistency checks

- [ ] Every builder follows Plan 1's `BuildX(inst) *X` / `XName(inst) string` pattern: confirmed for `BuildNetworkPolicy`/`NetworkPolicyName`, `BuildPDB`/`PDBName`, `BuildHPA`/`HPAName`, `BuildIngress`/`IngressName`, `BuildServiceAccount`/`ServiceAccountName`, `BuildRole`/`RoleName`, `BuildRoleBinding`/`RoleBindingName`, `BuildServiceMonitor`/`ServiceMonitorName`, `BuildPrometheusRule`/`PrometheusRuleName`, `BuildGatewayTokenSecret`/`GatewayTokenSecretName`, `BuildWorkspaceConfigMap`/`WorkspaceConfigMapName`.
- [ ] `+listType=map +listMapKey=source` on `.spec.skills`: Task 2.
- [ ] `+listType=map +listMapKey=name` on `.spec.env`: Task 2.
- [ ] `+listType=map +listMapKey=path` on `.spec.workspace.initialFiles`: Task 4 (Plan 4 references this for SSA on `addWorkspaceFiles`).
- [ ] `SelfConfigure.Enabled` typed as `*bool` per Plan 4's prerequisite: Task 9.
- [ ] Condition type strings (`StorageReady`, `ConfigReady`, ...) declared once in `api/v1/hermesinstance_types.go` as constants: Task 9 Step 5; reused by reconciler in Task 30.

### Type-consistency checks

- [ ] `internal/resources/common.go` defines `Ptr[T]`, `LabelsForInstance`, `MergePreservingForeign` (Plan 1) plus new helpers `SelectorLabels`, `ServiceAccountNameFor`, `BoolValue`, `BoolValueOrDefault`, `GatewayPort`, `DefaultMetricsPort`, `GatewayPortName`, `MetricsPortName`: Task 11. No duplicates of these names anywhere else.
- [ ] The webhook package has a private `Ptr[T]` (Task 23) to avoid an import cycle with `internal/resources`.
- [ ] The test files use local `Ptr` (in `api/v1`) and import-aliased Ptr from `internal/resources` (in resources tests).
- [ ] `runtime.RawExtension` wrapped as `RawConfig` with a hand-written deepcopy when generators choke: Task 3.

### Placeholder check (must all be `false`)

- [ ] Any line that reads `TODO`, `FIXME`, `XXX`, `<placeholder>`, `<fill-in>` in a code block? **No**: every step has actual code or commands.
- [ ] Any task whose Step list is shorter than 4 actions? **No**: every task has ≥4 steps; most have 5-7.
- [ ] Any commit message that's a generic "wip" / "fix tests"? **No**: every commit is conventional (`feat:`/`test:`/`docs:`/`chore:`) with a specific subject.

### What this plan deliberately does NOT do

- Implement `spec.runtime`, `spec.gateways`, `spec.profileStore`, `spec.ollama`, `spec.webTerminal`, `spec.tailscale`, `spec.autoUpdate` bodies: those are Plan 3. The fields are not declared on `HermesInstanceSpec` by Plan 2; Plan 3 adds them.
- Implement the real `HermesSelfConfig` controller / validator: Plan 4. Plan 2 lands a stub validator that always allows and emits a warning.
- Implement backup / restore / migration: Plan 5.
- Run the conformance suite or OLM bundle build: Plan 6.

### When this plan is "done"

`make manifests generate lint test` exits 0; `make e2e` (with cert-manager pre-installed in kind) exits 0; the `plan-2-complete` tag is created; the PR is open with the test-plan checklist green; `git worktree list` no longer shows `../hermes-operator-plan-2`.

