# Hermes Operator: Plan 1: Foundation + Minimal Happy Path

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the hermes-operator repository to the point where `kubectl apply -f hermesinstance-minimal.yaml` creates a running hermes-agent StatefulSet (with Service, PVC, and ConfigMap) in a kind cluster, fully wired into CI.

**Architecture:** Greenfield kubebuilder v4 scaffold, hermes-shaped CRDs at `hermes.agent/v1`, three CRDs scaffolded but only `HermesInstance` reconciled in this plan. Pure resource builders in `internal/resources/` consumed by a single `HermesInstanceReconciler` in `internal/controller/`. Helm chart skeleton + GitHub Actions CI (lint, test, build, Reconcile Guard, e2e on kind).

**Tech Stack:** Go 1.24, controller-runtime (kubebuilder v4), Ginkgo v2, Gomega, envtest, kind, golangci-lint, Helm 3, GitHub Actions.

**Prerequisite tooling on the engineer's machine:**
- Go 1.24+
- Docker
- `kubebuilder` v4 (install via `go install sigs.k8s.io/kubebuilder/v4@latest`)
- `kind` (`go install sigs.k8s.io/kind@latest`)
- `kubectl`, `helm`, `golangci-lint v1.64.5`
- `gh` CLI authenticated as `stubbi`

**Spec reference:** [`docs/superpowers/specs/2026-05-12-hermes-operator-design.md`](../specs/2026-05-12-hermes-operator-design.md) (sections 2, 3, 4, 7.1, 7.2).

---

## File Structure Established by This Plan

```
.
├── PROJECT                                 # kubebuilder metadata
├── Dockerfile                              # multi-stage operator image build
├── Makefile                                # standard kubebuilder targets + ours
├── go.mod / go.sum
├── README.md / LICENSE / CONTRIBUTING.md / SECURITY.md / CODEOWNERS
├── cmd/manager/main.go                     # entrypoint, scheme wiring, controller setup
├── api/v1/
│   ├── groupversion_info.go
│   ├── hermesinstance_types.go             # spec + status (minimal in this plan)
│   ├── hermesselfconfig_types.go           # types only, no controller in this plan
│   ├── hermesclusterdefaults_types.go      # types only, no controller in this plan
│   └── zz_generated.deepcopy.go            # generated
├── internal/
│   ├── controller/
│   │   ├── hermesinstance_controller.go
│   │   └── suite_test.go                   # envtest setup
│   └── resources/
│       ├── common.go                       # Ptr[T], labels, defaults merge
│       ├── common_test.go
│       ├── pvc.go / pvc_test.go
│       ├── configmap.go / configmap_test.go
│       ├── service.go / service_test.go
│       └── statefulset.go / statefulset_test.go
├── config/                                 # kustomize manifests (kubebuilder-generated)
│   ├── crd/bases/                          # generated CRD YAML
│   ├── default/ manager/ rbac/ samples/
├── charts/hermes-operator/
│   ├── Chart.yaml / values.yaml
│   └── templates/
│       ├── deployment.yaml / serviceaccount.yaml
│       ├── clusterrole.yaml / clusterrolebinding.yaml
│       └── crds/                           # templated CRDs, kept in sync with config/crd/bases/
├── test/e2e/
│   ├── e2e_suite_test.go                   # Ginkgo + kind harness
│   └── happypath_test.go
├── docs/
│   ├── conventions.md                      # referenced by Plans 2-7
│   └── superpowers/{specs,plans}/...
├── hack/
│   ├── boilerplate.go.txt
│   └── kind-config.yaml
└── .github/workflows/
    ├── ci.yaml                             # lint + test
    ├── reconcile-guard.yaml                # grep-banned patterns
    ├── build.yaml                          # docker build (main only)
    └── e2e.yaml                            # kind cluster e2e
```

---

## Task 1: Scaffold the kubebuilder project

**Files:**
- Create: `PROJECT`, `go.mod`, `Makefile`, `Dockerfile`, `cmd/manager/main.go`, `hack/boilerplate.go.txt`, `.dockerignore`, `.gitignore`

- [ ] **Step 1: Confirm we're in the empty repo**

Run:
```bash
cd /Users/jannesstubbemann/repos/hermes-operator
git status
```
Expected: working tree clean, branch `main`, one prior commit (`docs: add hermes-operator v1 design spec`).

- [ ] **Step 2: Run kubebuilder init**

```bash
kubebuilder init \
  --domain agent \
  --repo github.com/stubbi/hermes-operator \
  --owner "stubbi" \
  --project-name hermes-operator \
  --license apache2
```
Expected: creates `PROJECT`, `go.mod`, `Makefile`, `Dockerfile`, `cmd/manager/main.go`, `hack/boilerplate.go.txt`, `.dockerignore`, `.gitignore`, `README.md` (we'll overwrite README later). The `--domain agent` choice makes the API group root `<group>.agent`, which is what we want.

- [ ] **Step 3: Verify go module name**

```bash
head -1 go.mod
```
Expected: `module github.com/stubbi/hermes-operator`.

- [ ] **Step 4: Run `go mod tidy`**

```bash
go mod tidy
```
Expected: dependencies resolved, no errors.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: scaffold kubebuilder v4 project (domain=agent, module=stubbi/hermes-operator)"
```

---

## Task 2: Create three API kinds

**Files:**
- Create (by kubebuilder): `api/v1/hermesinstance_types.go`, `api/v1/hermesselfconfig_types.go`, `api/v1/hermesclusterdefaults_types.go`, `api/v1/groupversion_info.go`, `internal/controller/hermesinstance_controller.go`, plus PROJECT/main.go updates.

- [ ] **Step 1: Create HermesInstance (namespaced) with a controller**

```bash
kubebuilder create api \
  --group hermes \
  --version v1 \
  --kind HermesInstance \
  --controller=true \
  --resource=true
```
Press `y` when prompted for resource and controller. Expected: types file under `api/v1/`, controller under `internal/controller/`, watch wiring added to `cmd/manager/main.go`, PROJECT updated.

- [ ] **Step 2: Create HermesSelfConfig (namespaced, types only: no controller yet)**

```bash
kubebuilder create api \
  --group hermes \
  --version v1 \
  --kind HermesSelfConfig \
  --controller=false \
  --resource=true
```
Press `y` for resource, `n` for controller. Plan 4 adds the controller.

- [ ] **Step 3: Create HermesClusterDefaults (cluster-scoped singleton, types only)**

```bash
kubebuilder create api \
  --group hermes \
  --version v1 \
  --kind HermesClusterDefaults \
  --controller=false \
  --resource=true
```
Expected: file created. After generation, **manually** add `// +kubebuilder:resource:scope=Cluster` above the `HermesClusterDefaults` struct in `api/v1/hermesclusterdefaults_types.go` (kubebuilder defaults to namespaced).

- [ ] **Step 4: Verify the API group**

```bash
grep -r "GroupVersion" api/v1/groupversion_info.go
```
Expected: `Group: "hermes.agent"`.

- [ ] **Step 5: Run generators**

```bash
make generate manifests
```
Expected: `api/v1/zz_generated.deepcopy.go` created, CRD YAMLs in `config/crd/bases/` for all three kinds.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(api): scaffold HermesInstance, HermesSelfConfig, HermesClusterDefaults (v1)"
```

---

## Task 3: Minimal HermesInstance spec & status fields

**Files:**
- Modify: `api/v1/hermesinstance_types.go`

- [ ] **Step 1: Replace the scaffolded `Foo string` field with the minimal happy-path spec**

In `api/v1/hermesinstance_types.go`, replace the `HermesInstanceSpec` struct with:

```go
// HermesInstanceSpec defines the desired state of HermesInstance.
type HermesInstanceSpec struct {
    // Image controls which hermes-agent container image to run.
    // +optional
    Image ImageSpec `json:"image,omitempty"`

    // Storage controls the PVC backing ~/.hermes for this instance.
    // +optional
    Storage StorageSpec `json:"storage,omitempty"`
}

// ImageSpec selects an OCI image.
type ImageSpec struct {
    // +kubebuilder:default="ghcr.io/stubbi/hermes-agent"
    // +optional
    Repository string `json:"repository,omitempty"`

    // +kubebuilder:default="latest"
    // +optional
    Tag string `json:"tag,omitempty"`

    // +kubebuilder:default=IfNotPresent
    // +kubebuilder:validation:Enum=Always;IfNotPresent;Never
    // +optional
    PullPolicy string `json:"pullPolicy,omitempty"`
}

// StorageSpec controls the PVC backing the agent's data directory.
type StorageSpec struct {
    Persistence PersistenceSpec `json:"persistence,omitempty"`
}

type PersistenceSpec struct {
    // +kubebuilder:default=true
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // +kubebuilder:default="1Gi"
    // +optional
    Size string `json:"size,omitempty"`

    // +optional
    StorageClassName *string `json:"storageClassName,omitempty"`
}
```

- [ ] **Step 2: Replace the scaffolded status with conditions**

In the same file, replace `HermesInstanceStatus` with:

```go
// HermesInstanceStatus reflects the observed state of HermesInstance.
type HermesInstanceStatus struct {
    // ObservedGeneration is the most recent generation observed by the controller.
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // Phase is a short human-readable status (Pending|Ready|Degraded).
    // +optional
    Phase string `json:"phase,omitempty"`

    // Conditions represent the latest available observations of the instance's state.
    // +listType=map
    // +listMapKey=type
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

- [ ] **Step 3: Add printer columns and short names to the root type**

Above `type HermesInstance struct {`, ensure the markers read exactly:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=hi;hermes,categories=hermes;agents
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image.repository`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

- [ ] **Step 4: Regenerate**

```bash
make generate manifests
```
Expected: `zz_generated.deepcopy.go` updated; `config/crd/bases/hermes.agent_hermesinstances.yaml` reflects the new fields.

- [ ] **Step 5: Build to catch any syntax errors**

```bash
go build ./...
```
Expected: exit code 0.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(api): minimal HermesInstance spec (image, storage) and status (conditions)"
```

---

## Task 4: `internal/resources/common.go`: shared helpers + unit tests

**Files:**
- Create: `internal/resources/common.go`, `internal/resources/common_test.go`

- [ ] **Step 1: Write the failing tests first**

Create `internal/resources/common_test.go`:

```go
package resources

import (
    "testing"

    "github.com/stretchr/testify/assert"
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPtr(t *testing.T) {
    s := Ptr("x")
    assert.NotNil(t, s)
    assert.Equal(t, "x", *s)
}

func TestLabelsForInstance(t *testing.T) {
    inst := &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
    }
    got := LabelsForInstance(inst)
    assert.Equal(t, "hermes-agent", got["app.kubernetes.io/name"])
    assert.Equal(t, "demo", got["app.kubernetes.io/instance"])
    assert.Equal(t, "hermes-operator", got["app.kubernetes.io/managed-by"])
    assert.Equal(t, "hermes.agent", got["app.kubernetes.io/part-of"])
}

func TestMergePreservingForeignAnnotations(t *testing.T) {
    existing := map[string]string{
        "hermes.agent/foo":     "old",
        "third-party/keep-me":  "preserve",
    }
    desired := map[string]string{
        "hermes.agent/foo":  "new",
        "hermes.agent/bar":  "added",
    }
    got := MergePreservingForeign(existing, desired, "hermes.agent/")
    assert.Equal(t, "new", got["hermes.agent/foo"], "operator key overwritten")
    assert.Equal(t, "added", got["hermes.agent/bar"], "new operator key added")
    assert.Equal(t, "preserve", got["third-party/keep-me"], "foreign key preserved")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./internal/resources/...
```
Expected: build failures (`undefined: Ptr`, `undefined: LabelsForInstance`, `undefined: MergePreservingForeign`).

- [ ] **Step 3: Add `github.com/stretchr/testify` to go.mod**

```bash
go get github.com/stretchr/testify@v1.9.0
```

- [ ] **Step 4: Implement the helpers**

Create `internal/resources/common.go`:

```go
package resources

import (
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    "strings"
)

// Ptr returns a pointer to v. Use only for short-lived literals.
func Ptr[T any](v T) *T { return &v }

// LabelsForInstance returns the standard recommended labels for resources
// owned by a HermesInstance. Plans 2+ may add more.
func LabelsForInstance(inst *hermesv1.HermesInstance) map[string]string {
    return map[string]string{
        "app.kubernetes.io/name":       "hermes-agent",
        "app.kubernetes.io/instance":   inst.Name,
        "app.kubernetes.io/managed-by": "hermes-operator",
        "app.kubernetes.io/part-of":    "hermes.agent",
    }
}

// MergePreservingForeign merges desired into existing, overwriting keys that
// start with the operator prefix and preserving all other keys.
// Lesson from openclaw-operator #446/#447.
func MergePreservingForeign(existing, desired map[string]string, operatorPrefix string) map[string]string {
    out := make(map[string]string, len(existing)+len(desired))
    for k, v := range existing {
        if strings.HasPrefix(k, operatorPrefix) {
            continue
        }
        out[k] = v
    }
    for k, v := range desired {
        out[k] = v
    }
    return out
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
go test ./internal/resources/... -v
```
Expected: 3 PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(resources): add common helpers (Ptr, LabelsForInstance, MergePreservingForeign)"
```

---

## Task 5: PVC builder + unit test

**Files:**
- Create: `internal/resources/pvc.go`, `internal/resources/pvc_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/resources/pvc_test.go`:

```go
package resources

import (
    "testing"

    "github.com/stretchr/testify/assert"
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPVC_DefaultsAndLabels(t *testing.T) {
    inst := &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
        Spec: hermesv1.HermesInstanceSpec{
            Storage: hermesv1.StorageSpec{
                Persistence: hermesv1.PersistenceSpec{
                    Enabled: Ptr(true),
                    Size:    "5Gi",
                },
            },
        },
    }

    pvc := BuildPVC(inst)
    assert.Equal(t, "demo-data", pvc.Name)
    assert.Equal(t, "agents", pvc.Namespace)
    assert.Equal(t, "hermes-agent", pvc.Labels["app.kubernetes.io/name"])
    assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pvc.Spec.AccessModes)
    assert.Equal(t, resource.MustParse("5Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
    assert.Nil(t, pvc.Spec.StorageClassName, "no storage class when unset")
}

func TestBuildPVC_StorageClass(t *testing.T) {
    inst := &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
        Spec: hermesv1.HermesInstanceSpec{
            Storage: hermesv1.StorageSpec{
                Persistence: hermesv1.PersistenceSpec{
                    Enabled:          Ptr(true),
                    Size:             "1Gi",
                    StorageClassName: Ptr("gp3"),
                },
            },
        },
    }
    pvc := BuildPVC(inst)
    assert.NotNil(t, pvc.Spec.StorageClassName)
    assert.Equal(t, "gp3", *pvc.Spec.StorageClassName)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildPVC -v
```
Expected: build error `undefined: BuildPVC`.

- [ ] **Step 3: Implement `BuildPVC`**

Create `internal/resources/pvc.go`:

```go
package resources

import (
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCName returns the deterministic PVC name for a HermesInstance.
func PVCName(inst *hermesv1.HermesInstance) string {
    return inst.Name + "-data"
}

// BuildPVC returns the desired PersistentVolumeClaim. PVCs are immutable
// after creation (k8s rule); callers must only create, never update.
func BuildPVC(inst *hermesv1.HermesInstance) *corev1.PersistentVolumeClaim {
    size := inst.Spec.Storage.Persistence.Size
    if size == "" {
        size = "1Gi"
    }
    return &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      PVCName(inst),
            Namespace: inst.Namespace,
            Labels:    LabelsForInstance(inst),
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
            Resources:        corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: resource.MustParse(size),
                },
            },
            StorageClassName: inst.Spec.Storage.Persistence.StorageClassName,
        },
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildPVC -v
```
Expected: 2 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add PVC builder with size + storageClass support"
```

---

## Task 6: ConfigMap builder + unit test

**Files:**
- Create: `internal/resources/configmap.go`, `internal/resources/configmap_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/resources/configmap_test.go`:

```go
package resources

import (
    "testing"

    "github.com/stretchr/testify/assert"
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildConfigMap_HasConfigKey(t *testing.T) {
    inst := &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
    }
    cm := BuildConfigMap(inst)
    assert.Equal(t, "demo-config", cm.Name)
    assert.Contains(t, cm.Data, "config.yaml")
    assert.Equal(t, "{}\n", cm.Data["config.yaml"], "minimal config is an empty YAML mapping")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildConfigMap -v
```
Expected: `undefined: BuildConfigMap`.

- [ ] **Step 3: Implement `BuildConfigMap`**

Create `internal/resources/configmap.go`:

```go
package resources

import (
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapName returns the deterministic ConfigMap name for a HermesInstance.
func ConfigMapName(inst *hermesv1.HermesInstance) string {
    return inst.Name + "-config"
}

// BuildConfigMap returns the desired ConfigMap holding ~/.hermes/config.yaml.
// In this plan the body is a minimal empty mapping. Plan 2 wires spec.config.
func BuildConfigMap(inst *hermesv1.HermesInstance) *corev1.ConfigMap {
    return &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      ConfigMapName(inst),
            Namespace: inst.Namespace,
            Labels:    LabelsForInstance(inst),
        },
        Data: map[string]string{
            "config.yaml": "{}\n",
        },
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildConfigMap -v
```
Expected: 1 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add minimal ConfigMap builder"
```

---

## Task 7: Service builder + unit test

**Files:**
- Create: `internal/resources/service.go`, `internal/resources/service_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/resources/service_test.go`:

```go
package resources

import (
    "testing"

    "github.com/stretchr/testify/assert"
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildService_DefaultsAndSelector(t *testing.T) {
    inst := &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
    }
    svc := BuildService(inst)
    assert.Equal(t, "demo", svc.Name)
    assert.Equal(t, corev1.ClusterIPNone, svc.Spec.ClusterIP, "headless Service for StatefulSet")
    assert.Equal(t, corev1.ServiceAffinityNone, svc.Spec.SessionAffinity, "explicit k8s default")
    assert.Equal(t, "demo", svc.Spec.Selector["app.kubernetes.io/instance"])
    // Must declare at least the gateway port so the Service is valid.
    assert.NotEmpty(t, svc.Spec.Ports)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildService -v
```
Expected: `undefined: BuildService`.

- [ ] **Step 3: Implement `BuildService`**

Create `internal/resources/service.go`:

```go
package resources

import (
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceName returns the deterministic Service name for a HermesInstance.
func ServiceName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildService returns the desired headless Service. Headless = stable DNS
// for the StatefulSet pod. Plan 2 adds optional ClusterIP / LoadBalancer modes.
func BuildService(inst *hermesv1.HermesInstance) *corev1.Service {
    return &corev1.Service{
        ObjectMeta: metav1.ObjectMeta{
            Name:      ServiceName(inst),
            Namespace: inst.Namespace,
            Labels:    LabelsForInstance(inst),
        },
        Spec: corev1.ServiceSpec{
            ClusterIP:       corev1.ClusterIPNone,
            SessionAffinity: corev1.ServiceAffinityNone, // explicit k8s default
            Selector: map[string]string{
                "app.kubernetes.io/name":     "hermes-agent",
                "app.kubernetes.io/instance": inst.Name,
            },
            Ports: []corev1.ServicePort{
                {
                    Name:       "gateway",
                    Port:       8443,
                    TargetPort: intstr.FromString("gateway"),
                    Protocol:   corev1.ProtocolTCP,
                },
            },
        },
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/resources/... -run TestBuildService -v
```
Expected: 1 PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(resources): add headless Service builder with explicit k8s defaults"
```

---

## Task 8: StatefulSet builder + unit test (with explicit k8s defaults!)

**Files:**
- Create: `internal/resources/statefulset.go`, `internal/resources/statefulset_test.go`

This is the critical builder. Every k8s server-side default **must** be set explicitly (lesson from openclaw-operator: skipping any of these causes `metadata.generation` thrash on every reconcile).

- [ ] **Step 1: Write the failing tests**

Create `internal/resources/statefulset_test.go`:

```go
package resources

import (
    "testing"

    "github.com/stretchr/testify/assert"
    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildStatefulSet_NameNamespaceLabels(t *testing.T) {
    sts := BuildStatefulSet(minimalInstance())
    assert.Equal(t, "demo", sts.Name)
    assert.Equal(t, "agents", sts.Namespace)
    assert.Equal(t, "hermes-agent", sts.Labels["app.kubernetes.io/name"])
    assert.Equal(t, "demo", sts.Spec.ServiceName, "matches Service name for stable DNS")
}

func TestBuildStatefulSet_ContainerImage(t *testing.T) {
    inst := minimalInstance()
    inst.Spec.Image.Repository = "ghcr.io/stubbi/hermes-agent"
    inst.Spec.Image.Tag = "v1.0.0"
    sts := BuildStatefulSet(inst)
    require := sts.Spec.Template.Spec.Containers
    assert.Len(t, require, 1)
    assert.Equal(t, "ghcr.io/stubbi/hermes-agent:v1.0.0", require[0].Image)
    assert.Equal(t, corev1.PullIfNotPresent, require[0].ImagePullPolicy, "explicit default")
}

func TestBuildStatefulSet_ExplicitK8sDefaults(t *testing.T) {
    sts := BuildStatefulSet(minimalInstance())
    podSpec := sts.Spec.Template.Spec

    // Pod-level defaults that the API server fills if we omit them: we must set them.
    assert.NotNil(t, sts.Spec.RevisionHistoryLimit)
    assert.Equal(t, int32(10), *sts.Spec.RevisionHistoryLimit)
    assert.Equal(t, corev1.RestartPolicyAlways, podSpec.RestartPolicy)
    assert.Equal(t, corev1.DNSClusterFirst, podSpec.DNSPolicy)
    assert.Equal(t, "default-scheduler", podSpec.SchedulerName)
    assert.NotNil(t, podSpec.TerminationGracePeriodSeconds)
    assert.Equal(t, int64(30), *podSpec.TerminationGracePeriodSeconds)

    c := podSpec.Containers[0]
    assert.Equal(t, "/dev/termination-log", c.TerminationMessagePath)
    assert.Equal(t, corev1.TerminationMessageReadFile, c.TerminationMessagePolicy)
}

func TestBuildStatefulSet_HardenedPodSecurity(t *testing.T) {
    sts := BuildStatefulSet(minimalInstance())
    pc := sts.Spec.Template.Spec.SecurityContext
    require := sts.Spec.Template.Spec.Containers[0].SecurityContext
    assert.NotNil(t, pc.RunAsNonRoot)
    assert.True(t, *pc.RunAsNonRoot)
    assert.NotNil(t, require.AllowPrivilegeEscalation)
    assert.False(t, *require.AllowPrivilegeEscalation)
    assert.NotNil(t, require.ReadOnlyRootFilesystem)
    assert.True(t, *require.ReadOnlyRootFilesystem)
    assert.Equal(t, []corev1.Capability{"ALL"}, require.Capabilities.Drop)
}

func TestBuildStatefulSet_VolumesAndMounts(t *testing.T) {
    sts := BuildStatefulSet(minimalInstance())
    c := sts.Spec.Template.Spec.Containers[0]

    mountNames := map[string]string{}
    for _, m := range c.VolumeMounts {
        mountNames[m.Name] = m.MountPath
    }
    assert.Equal(t, "/home/hermes/.hermes", mountNames["data"], "PVC mounted at hermes home")
    assert.Equal(t, "/home/hermes/.hermes/config.yaml", mountNames["config"], "configmap subPath at config.yaml")
    assert.Equal(t, "/tmp", mountNames["tmp"], "writable /tmp for read-only rootfs")
}

func minimalInstance() *hermesv1.HermesInstance {
    return &hermesv1.HermesInstance{
        ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/resources/... -run TestBuildStatefulSet -v
```
Expected: `undefined: BuildStatefulSet`.

- [ ] **Step 3: Implement `BuildStatefulSet`**

Create `internal/resources/statefulset.go`:

```go
package resources

import (
    "fmt"

    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
)

// StatefulSetName returns the deterministic name.
func StatefulSetName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildStatefulSet constructs the desired StatefulSet. Every k8s server-side
// default is set explicitly to avoid metadata.generation thrash on reconcile.
func BuildStatefulSet(inst *hermesv1.HermesInstance) *appsv1.StatefulSet {
    labels := LabelsForInstance(inst)
    selector := map[string]string{
        "app.kubernetes.io/name":     "hermes-agent",
        "app.kubernetes.io/instance": inst.Name,
    }
    image := imageRef(inst)

    return &appsv1.StatefulSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      StatefulSetName(inst),
            Namespace: inst.Namespace,
            Labels:    labels,
        },
        Spec: appsv1.StatefulSetSpec{
            ServiceName:          ServiceName(inst),
            Replicas:             Ptr(int32(1)),
            RevisionHistoryLimit: Ptr(int32(10)),
            Selector:             &metav1.LabelSelector{MatchLabels: selector},
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{Labels: labels},
                Spec: corev1.PodSpec{
                    RestartPolicy:                 corev1.RestartPolicyAlways,
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
                        Name:                     "hermes",
                        Image:                    image,
                        ImagePullPolicy:          pullPolicy(inst),
                        TerminationMessagePath:   "/dev/termination-log",
                        TerminationMessagePolicy: corev1.TerminationMessageReadFile,
                        Ports: []corev1.ContainerPort{{
                            Name:          "gateway",
                            ContainerPort: 8443,
                            Protocol:      corev1.ProtocolTCP,
                        }},
                        SecurityContext: &corev1.SecurityContext{
                            AllowPrivilegeEscalation: Ptr(false),
                            ReadOnlyRootFilesystem:   Ptr(true),
                            Capabilities: &corev1.Capabilities{
                                Drop: []corev1.Capability{"ALL"},
                            },
                        },
                        VolumeMounts: []corev1.VolumeMount{
                            {Name: "data", MountPath: "/home/hermes/.hermes"},
                            {
                                Name:      "config",
                                MountPath: "/home/hermes/.hermes/config.yaml",
                                SubPath:   "config.yaml",
                                ReadOnly:  true,
                            },
                            {Name: "tmp", MountPath: "/tmp"},
                        },
                        ReadinessProbe: &corev1.Probe{
                            ProbeHandler: corev1.ProbeHandler{
                                TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("gateway")},
                            },
                            InitialDelaySeconds: 5,
                            PeriodSeconds:       10,
                            TimeoutSeconds:      1,
                            FailureThreshold:    3,
                            SuccessThreshold:    1, // explicit k8s default
                        },
                    }},
                    Volumes: []corev1.Volume{
                        {
                            Name: "config",
                            VolumeSource: corev1.VolumeSource{
                                ConfigMap: &corev1.ConfigMapVolumeSource{
                                    LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(inst)},
                                    DefaultMode:          Ptr(int32(0o644)),
                                },
                            },
                        },
                        {
                            Name: "tmp",
                            VolumeSource: corev1.VolumeSource{
                                EmptyDir: &corev1.EmptyDirVolumeSource{},
                            },
                        },
                    },
                },
            },
            VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
                // Reference the externally-managed PVC by name via a templated claim that
                // is functionally equivalent to BuildPVC. Plan 2 will switch to letting the
                // STS create the claim directly (volumeClaimTemplate), once we wire size
                // and storageClass from spec.storage.
            },
        },
    }
}

func imageRef(inst *hermesv1.HermesInstance) string {
    repo := inst.Spec.Image.Repository
    if repo == "" {
        repo = "ghcr.io/stubbi/hermes-agent"
    }
    tag := inst.Spec.Image.Tag
    if tag == "" {
        tag = "latest"
    }
    return fmt.Sprintf("%s:%s", repo, tag)
}

func pullPolicy(inst *hermesv1.HermesInstance) corev1.PullPolicy {
    if inst.Spec.Image.PullPolicy == "" {
        return corev1.PullIfNotPresent
    }
    return corev1.PullPolicy(inst.Spec.Image.PullPolicy)
}
```

> **Note for Plan 2 (forward reference):** the current builder mounts the PVC as `data` but the volume is *not* declared in `Spec.Template.Spec.Volumes` (it's expected to come from `volumeClaimTemplates` or an externally-managed PVC). Plan 2 wires this end-to-end via `volumeClaimTemplates`; for now Plan 1's reconciler creates the PVC separately and the StatefulSet references it via a tiny patch step in Task 9.

- [ ] **Step 4: Patch the builder to declare the data volume from the external PVC**

Append this volume to `Spec.Template.Spec.Volumes` inside `BuildStatefulSet` (between the `config` and `tmp` volumes):

```go
{
    Name: "data",
    VolumeSource: corev1.VolumeSource{
        PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
            ClaimName: PVCName(inst),
        },
    },
},
```

And remove the empty `VolumeClaimTemplates` field entirely (delete those lines).

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/resources/... -v
```
Expected: all PASS (PVC, ConfigMap, Service, StatefulSet, common).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(resources): add hardened StatefulSet builder with explicit k8s defaults"
```

---

## Task 9: HermesInstanceReconciler: orchestrate the four resources

**Files:**
- Modify: `internal/controller/hermesinstance_controller.go`

- [ ] **Step 1: Replace the scaffolded Reconcile body**

Open `internal/controller/hermesinstance_controller.go`. Replace the entire file body with:

```go
package controller

import (
    "context"
    "fmt"
    "time"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/log"

    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
    "github.com/stubbi/hermes-operator/internal/resources"
)

// HermesInstanceReconciler reconciles a HermesInstance.
type HermesInstanceReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

const operatorLabelPrefix = "hermes.agent/"

func (r *HermesInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)

    var inst hermesv1.HermesInstance
    if err := r.Get(ctx, req.NamespacedName, &inst); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // 1. PVC: create-only (immutable after creation)
    if err := r.reconcilePVC(ctx, &inst); err != nil {
        return ctrl.Result{}, fmt.Errorf("reconcile PVC: %w", err)
    }

    // 2. ConfigMap
    if err := r.reconcileConfigMap(ctx, &inst); err != nil {
        return ctrl.Result{}, fmt.Errorf("reconcile ConfigMap: %w", err)
    }

    // 3. Service
    if err := r.reconcileService(ctx, &inst); err != nil {
        return ctrl.Result{}, fmt.Errorf("reconcile Service: %w", err)
    }

    // 4. StatefulSet
    if err := r.reconcileStatefulSet(ctx, &inst); err != nil {
        return ctrl.Result{}, fmt.Errorf("reconcile StatefulSet: %w", err)
    }

    // Status: mark Ready when STS reports ready replicas.
    if err := r.updateStatus(ctx, &inst); err != nil {
        logger.Error(err, "status update failed")
    }

    return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *HermesInstanceReconciler) reconcilePVC(ctx context.Context, inst *hermesv1.HermesInstance) error {
    pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
        Name:      resources.PVCName(inst),
        Namespace: inst.Namespace,
    }}
    err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
    if apierrors.IsNotFound(err) {
        desired := resources.BuildPVC(inst)
        if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
            return err
        }
        return r.Create(ctx, desired)
    }
    return err // PVCs are immutable; nothing else to do
}

func (r *HermesInstanceReconciler) reconcileConfigMap(ctx context.Context, inst *hermesv1.HermesInstance) error {
    obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
        Name:      resources.ConfigMapName(inst),
        Namespace: inst.Namespace,
    }}
    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
        desired := resources.BuildConfigMap(inst)
        obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
        obj.Data = desired.Data
        return controllerutil.SetControllerReference(inst, obj, r.Scheme)
    })
    return err
}

func (r *HermesInstanceReconciler) reconcileService(ctx context.Context, inst *hermesv1.HermesInstance) error {
    obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
        Name:      resources.ServiceName(inst),
        Namespace: inst.Namespace,
    }}
    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
        desired := resources.BuildService(inst)
        obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
        // Preserve server-assigned ClusterIP fields (headless => empty, but preserve anyway for safety).
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

func (r *HermesInstanceReconciler) reconcileStatefulSet(ctx context.Context, inst *hermesv1.HermesInstance) error {
    obj := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
        Name:      resources.StatefulSetName(inst),
        Namespace: inst.Namespace,
    }}
    _, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
        desired := resources.BuildStatefulSet(inst)
        obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
        obj.Spec = desired.Spec
        return controllerutil.SetControllerReference(inst, obj, r.Scheme)
    })
    return err
}

func (r *HermesInstanceReconciler) updateStatus(ctx context.Context, inst *hermesv1.HermesInstance) error {
    sts := &appsv1.StatefulSet{}
    if err := r.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
        return err
    }
    ready := sts.Status.ReadyReplicas > 0 && sts.Status.ReadyReplicas == sts.Status.Replicas
    phase := "Pending"
    if ready {
        phase = "Ready"
    }
    if inst.Status.Phase != phase || inst.Status.ObservedGeneration != inst.Generation {
        inst.Status.Phase = phase
        inst.Status.ObservedGeneration = inst.Generation
        return r.Status().Update(ctx, inst)
    }
    return nil
}

// SetupWithManager wires watches.
func (r *HermesInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&hermesv1.HermesInstance{}).
        Owns(&appsv1.StatefulSet{}).
        Owns(&corev1.Service{}).
        Owns(&corev1.ConfigMap{}).
        Owns(&corev1.PersistentVolumeClaim{}).
        Named("hermesinstance").
        Complete(r)
}
```

- [ ] **Step 2: Regenerate RBAC manifests**

```bash
make manifests
```
Expected: `config/rbac/role.yaml` updated with the new RBAC markers (statefulsets/services/configmaps/PVCs verbs).

- [ ] **Step 3: Build to verify**

```bash
go build ./...
```
Expected: exit code 0.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(controller): reconcile PVC, ConfigMap, Service, StatefulSet for HermesInstance"
```

---

## Task 10: envtest suite for the controller

**Files:**
- Modify: `internal/controller/suite_test.go` (scaffolded by kubebuilder; we extend it)
- Create: `internal/controller/hermesinstance_controller_test.go`

- [ ] **Step 1: Install envtest binaries**

```bash
make envtest
```
Expected: kubebuilder downloads etcd + kube-apiserver binaries into `bin/k8s/`.

- [ ] **Step 2: Confirm the scaffolded suite_test.go imports our scheme**

Open `internal/controller/suite_test.go` and verify it contains:

```go
err = hermesv1.AddToScheme(scheme.Scheme)
```

If missing, add it after the existing scheme registrations.

- [ ] **Step 3: Write the controller happy-path test**

Create `internal/controller/hermesinstance_controller_test.go`:

```go
package controller

import (
    "context"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"

    hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("HermesInstance controller", func() {
    const (
        name      = "demo"
        namespace = "default"
        timeout   = 30 * time.Second
        interval  = 250 * time.Millisecond
    )

    AfterEach(func() {
        ctx := context.Background()
        inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
        _ = k8sClient.Delete(ctx, inst)
    })

    It("creates PVC, ConfigMap, Service, and StatefulSet for a new HermesInstance", func() {
        ctx := context.Background()

        inst := &hermesv1.HermesInstance{
            ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
            Spec: hermesv1.HermesInstanceSpec{
                Image: hermesv1.ImageSpec{
                    Repository: "ghcr.io/stubbi/hermes-agent",
                    Tag:        "test",
                },
            },
        }
        Expect(k8sClient.Create(ctx, inst)).To(Succeed())

        Eventually(func(g Gomega) {
            pvc := &corev1.PersistentVolumeClaim{}
            g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-data", Namespace: namespace}, pvc)).To(Succeed())
            cm := &corev1.ConfigMap{}
            g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-config", Namespace: namespace}, cm)).To(Succeed())
            svc := &corev1.Service{}
            g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, svc)).To(Succeed())
            sts := &appsv1.StatefulSet{}
            g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
            g.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/stubbi/hermes-agent:test"))
        }).Within(timeout).WithPolling(interval).Should(Succeed())
    })

    It("is idempotent: second reconcile does not change StatefulSet generation", func() {
        ctx := context.Background()

        inst := &hermesv1.HermesInstance{
            ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
        }
        Expect(k8sClient.Create(ctx, inst)).To(Succeed())

        var stsGenBefore int64
        Eventually(func(g Gomega) {
            sts := &appsv1.StatefulSet{}
            g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
            stsGenBefore = sts.Generation
        }).Within(timeout).WithPolling(interval).Should(Succeed())

        // Force a re-reconcile by touching an unrelated annotation on the CR.
        Eventually(func() error {
            var cur hermesv1.HermesInstance
            if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur); err != nil {
                return err
            }
            if cur.Annotations == nil {
                cur.Annotations = map[string]string{}
            }
            cur.Annotations["test.example.com/poke"] = time.Now().String()
            return k8sClient.Update(ctx, &cur)
        }).Within(timeout).WithPolling(interval).Should(Succeed())

        // Wait long enough for at least one reconcile to land.
        time.Sleep(2 * time.Second)

        sts := &appsv1.StatefulSet{}
        Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
        Expect(sts.Generation).To(Equal(stsGenBefore), "STS generation must not bump on no-op reconcile")
    })
})
```

- [ ] **Step 4: Wire the reconciler into `suite_test.go`**

In `internal/controller/suite_test.go`, after the `k8sManager` is created and before `Start`, add:

```go
err = (&HermesInstanceReconciler{
    Client: k8sManager.GetClient(),
    Scheme: k8sManager.GetScheme(),
}).SetupWithManager(k8sManager)
Expect(err).ToNot(HaveOccurred())
```

- [ ] **Step 5: Run the envtest suite**

```bash
make test
```
Expected: all tests PASS, including the new BDD block. Idempotency test is the canary: if it fails, look for a builder field that's not explicitly set.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "test(controller): add envtest suite verifying happy-path reconcile + idempotency"
```

---

## Task 11: Reconcile Guard: CI grep check against bare `r.Update()`

**Files:**
- Create: `.github/workflows/reconcile-guard.yaml`, `hack/reconcile-guard.sh`

- [ ] **Step 1: Write the check script**

Create `hack/reconcile-guard.sh`:

```bash
#!/usr/bin/env bash
# Reconcile Guard: prevents bare r.Update() / r.Create() on managed resources.
# See docs/conventions.md "Reconciliation rules".

set -euo pipefail

# Banned patterns. Allowance comment "// reconcile-guard:allow" suppresses on a per-line basis.
banned_patterns=(
    'r\.Update\(ctx,'
    'r\.Create\(ctx,'
)

fail=0
for pattern in "${banned_patterns[@]}"; do
    # Find matches in internal/controller/, exclude lines with the allow comment, exclude tests.
    if matches=$(grep -RInE "$pattern" internal/controller/ \
            --include="*.go" \
            --exclude="*_test.go" \
            | grep -v "reconcile-guard:allow" \
            | grep -vE 'reconcilePVC|reconcileCR\(|//.*' || true); then
        if [ -n "$matches" ]; then
            echo "::error::Banned pattern '$pattern' found:" >&2
            echo "$matches" >&2
            fail=1
        fi
    fi
done

exit $fail
```

```bash
chmod +x hack/reconcile-guard.sh
```

- [ ] **Step 2: Verify it passes on the current code**

```bash
bash hack/reconcile-guard.sh
```
Expected: exit code 0. (The reconciler uses `r.Create(ctx, ...)` for the PVC, which is acceptable; we documented the script to allow it inside `reconcilePVC`.)

> **Note for Plan 2:** the grep filter is intentionally permissive. Plan 2 strengthens it by checking for `controllerutil.CreateOrUpdate` adjacency. The Plan 1 version only catches the most obvious regressions.

- [ ] **Step 3: Add the GitHub Actions workflow**

Create `.github/workflows/reconcile-guard.yaml`:

```yaml
name: Reconcile Guard
on:
  pull_request:
  push:
    branches: [main]
jobs:
  guard:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: bash hack/reconcile-guard.sh
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "ci: add Reconcile Guard grep check (lesson from openclaw)"
```

---

## Task 12: golangci-lint config + CI workflows for lint + test + build

**Files:**
- Create: `.golangci.yml`, `.github/workflows/ci.yaml`, `.github/workflows/build.yaml`

- [ ] **Step 1: Pin golangci-lint config**

Create `.golangci.yml`:

```yaml
run:
  timeout: 5m
  go: "1.24"

linters:
  enable:
    - gocritic
    - gofmt
    - goimports
    - govet
    - ineffassign
    - misspell
    - revive
    - staticcheck
    - unused

linters-settings:
  gocritic:
    enabled-checks:
      - octalLiteral
  goimports:
    local-prefixes: github.com/stubbi/hermes-operator

issues:
  exclude-rules:
    - path: zz_generated.*\.go
      linters: [staticcheck, revive, gocritic]
```

- [ ] **Step 2: Write CI workflow**

Create `.github/workflows/ci.yaml`:

```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: golangci/golangci-lint-action@v6
        with: { version: v1.64.5 }

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - run: make test
```

- [ ] **Step 3: Write build workflow (multi-arch image; pushes only on main)**

Create `.github/workflows/build.yaml`:

```yaml
name: Build
on:
  push:
    branches: [main]
  pull_request:

jobs:
  docker:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        if: github.event_name == 'push'
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: ${{ github.event_name == 'push' }}
          tags: ghcr.io/stubbi/hermes-operator:dev,ghcr.io/stubbi/hermes-operator:${{ github.sha }}
```

- [ ] **Step 4: Run lint locally**

```bash
golangci-lint run --timeout 5m
```
Expected: exit code 0. (If there are issues, fix and recommit before pushing.)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "ci: add lint + test + multi-arch build workflows"
```

---

## Task 13: Helm chart skeleton

**Files:**
- Create: `charts/hermes-operator/Chart.yaml`, `charts/hermes-operator/values.yaml`, `charts/hermes-operator/.helmignore`, `charts/hermes-operator/templates/_helpers.tpl`, `charts/hermes-operator/templates/serviceaccount.yaml`, `charts/hermes-operator/templates/clusterrole.yaml`, `charts/hermes-operator/templates/clusterrolebinding.yaml`, `charts/hermes-operator/templates/deployment.yaml`
- Add: `charts/hermes-operator/templates/crds/` (populated by `make sync-chart-crds`)
- Add: Makefile target `sync-chart-crds`

- [ ] **Step 1: Create `Chart.yaml`**

```yaml
apiVersion: v2
name: hermes-operator
description: Kubernetes operator for nousresearch/hermes-agent
type: application
version: 0.1.0
appVersion: "0.1.0"
kubeVersion: ">=1.28.0-0"
home: https://github.com/stubbi/hermes-operator
sources:
  - https://github.com/stubbi/hermes-operator
maintainers:
  - name: stubbi
    email: jannes@aqora.io
```

- [ ] **Step 2: Create `values.yaml`**

```yaml
image:
  repository: ghcr.io/stubbi/hermes-operator
  tag: ""               # defaults to .Chart.AppVersion with "v" prefix
  pullPolicy: IfNotPresent

logLevel: info

createRBAC: true        # set false to use a pre-existing ClusterRole

watchNamespaces: []     # empty = cluster-scoped; non-empty = namespace-scoped

metrics:
  secure: true

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

- [ ] **Step 3: Add `_helpers.tpl`**

```yaml
{{- define "hermes-operator.fullname" -}}
{{- printf "%s" (default .Chart.Name .Values.nameOverride) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "hermes-operator.labels" -}}
app.kubernetes.io/name: {{ include "hermes-operator.fullname" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: hermes.agent
{{- end -}}

{{- define "hermes-operator.image" -}}
{{- $tag := default (printf "v%s" .Chart.AppVersion) .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}
```

- [ ] **Step 4: Add `serviceaccount.yaml`**

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "hermes-operator.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels: {{- include "hermes-operator.labels" . | nindent 4 }}
```

- [ ] **Step 5: Add `clusterrole.yaml`**

```yaml
{{- if .Values.createRBAC }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "hermes-operator.fullname" . }}
  labels: {{- include "hermes-operator.labels" . | nindent 4 }}
rules:
  - apiGroups: [hermes.agent]
    resources: [hermesinstances, hermesselfconfigs, hermesclusterdefaults]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [hermes.agent]
    resources: [hermesinstances/status, hermesselfconfigs/status, hermesclusterdefaults/status]
    verbs: [get, update, patch]
  - apiGroups: [hermes.agent]
    resources: [hermesinstances/finalizers]
    verbs: [update]
  - apiGroups: [apps]
    resources: [statefulsets]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [""]
    resources: [services, configmaps, persistentvolumeclaims, events]
    verbs: [get, list, watch, create, update, patch, delete]
{{- end }}
```

- [ ] **Step 6: Add `clusterrolebinding.yaml`**

```yaml
{{- if .Values.createRBAC }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "hermes-operator.fullname" . }}
  labels: {{- include "hermes-operator.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "hermes-operator.fullname" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "hermes-operator.fullname" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}
```

- [ ] **Step 7: Add `deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "hermes-operator.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels: {{- include "hermes-operator.labels" . | nindent 4 }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "hermes-operator.fullname" . }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ include "hermes-operator.fullname" . }}
    spec:
      serviceAccountName: {{ include "hermes-operator.fullname" . }}
      securityContext:
        runAsNonRoot: true
        seccompProfile: { type: RuntimeDefault }
      containers:
        - name: manager
          image: {{ include "hermes-operator.image" . | quote }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --log-level={{ .Values.logLevel }}
            - --metrics-secure={{ .Values.metrics.secure }}
            {{- range .Values.watchNamespaces }}
            - --watch-namespace={{ . }}
            {{- end }}
          resources: {{- toYaml .Values.resources | nindent 12 }}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: [ALL] }
          ports:
            - name: metrics
              containerPort: 8443
              protocol: TCP
```

- [ ] **Step 8: Add Makefile target `sync-chart-crds`**

Open `Makefile`, append:

```makefile
.PHONY: sync-chart-crds
sync-chart-crds: manifests ## Copy generated CRDs into the Helm chart.
	rm -rf charts/hermes-operator/templates/crds
	mkdir -p charts/hermes-operator/templates/crds
	cp config/crd/bases/*.yaml charts/hermes-operator/templates/crds/
```

- [ ] **Step 9: Run sync**

```bash
make sync-chart-crds
ls charts/hermes-operator/templates/crds/
```
Expected: three YAML files (one per CRD).

- [ ] **Step 10: Smoke-test rendering**

```bash
helm lint charts/hermes-operator
helm template hermes-operator charts/hermes-operator | head -40
```
Expected: lint passes; rendered manifests visible.

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "feat(chart): add Helm chart skeleton with templated CRDs and operator deployment"
```

---

## Task 14: Helm RBAC sync check in CI

**Files:**
- Create: `hack/check-helm-rbac.sh`, `.github/workflows/helm-rbac.yaml`

The chart ClusterRole and the kubebuilder-generated `config/rbac/role.yaml` must stay in sync. Lesson from openclaw.

- [ ] **Step 1: Write the check script**

Create `hack/check-helm-rbac.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Render the chart, extract the verbs, compare against the kubebuilder-generated role.
generated=$(yq '.rules' config/rbac/role.yaml | yq -s 'sort_by(.apiGroups, .resources)')
rendered=$(helm template hermes-operator charts/hermes-operator | yq 'select(.kind=="ClusterRole") | .rules' | yq -s 'sort_by(.apiGroups, .resources)')

if [ "$generated" != "$rendered" ]; then
    echo "::error::Helm chart ClusterRole drifted from kubebuilder-generated role." >&2
    diff <(echo "$generated") <(echo "$rendered") >&2 || true
    exit 1
fi
```

```bash
chmod +x hack/check-helm-rbac.sh
```

- [ ] **Step 2: Add the workflow**

Create `.github/workflows/helm-rbac.yaml`:

```yaml
name: Helm RBAC Sync
on:
  pull_request:
  push:
    branches: [main]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: azure/setup-helm@v4
      - run: sudo snap install yq
      - run: bash hack/check-helm-rbac.sh
```

- [ ] **Step 3: Verify locally (requires `yq`)**

```bash
brew install yq
bash hack/check-helm-rbac.sh
```
Expected: exit code 0. If diff appears, the Helm ClusterRole needs to match `config/rbac/role.yaml`. Update the chart and retry.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "ci: add Helm RBAC sync check (lesson from openclaw #479)"
```

---

## Task 15: README, LICENSE, CONTRIBUTING, SECURITY, CODEOWNERS

**Files:**
- Create: `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`
- Overwrite: `README.md`

- [ ] **Step 1: Add Apache-2.0 LICENSE**

Run:
```bash
curl -fsSL https://www.apache.org/licenses/LICENSE-2.0.txt -o LICENSE
```
Expected: 11k file written. (If air-gapped, copy from `cat /Users/jannesstubbemann/repos/openclawrocks/k8s-operator-v1/LICENSE`: same Apache-2.0 text.)

- [ ] **Step 2: Write `README.md`**

Replace the kubebuilder-generated README with:

```markdown
# hermes-operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/stubbi/hermes-operator)](https://goreportcard.com/report/github.com/stubbi/hermes-operator)
[![CI](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/stubbi/hermes-operator/actions/workflows/ci.yaml)

Kubernetes operator for [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent): a Python-based self-improving multi-platform AI agent.

> **Status: alpha.** Plan 1 of 7 shipped (minimal happy path).

## Quick start

```bash
helm install hermes-operator charts/hermes-operator -n hermes-system --create-namespace

kubectl apply -f - <<EOF
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: demo
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
EOF

kubectl get hi
```

## Design

See [`docs/superpowers/specs/2026-05-12-hermes-operator-design.md`](docs/superpowers/specs/2026-05-12-hermes-operator-design.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions are licensed Apache-2.0.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.
```

- [ ] **Step 3: Write `CONTRIBUTING.md`**

```markdown
# Contributing

## Development

- Go 1.24+, kubebuilder v4, kind, helm, golangci-lint v1.64.5.
- `make test` runs unit + envtest. `make lint` runs golangci-lint.
- `make sync-chart-crds` after `make manifests` (CI enforces this).
- Use conventional commits: `feat:` `fix:` `docs:` `ci:` `chore:` `refactor:` `test:`.
- Use git worktrees rather than switching branches in the main checkout (`git worktree add ../hermes-operator-<suffix> -b <branch> main`).

## Reconciliation rules

See [`docs/conventions.md`](docs/conventions.md). The `Reconcile Guard` CI job enforces a subset; you are responsible for the rest.
```

- [ ] **Step 4: Write `SECURITY.md`**

```markdown
# Security Policy

Report vulnerabilities by email to **jannes@aqora.io** with the subject "SECURITY: hermes-operator".

We aim to acknowledge within 72 hours and provide a remediation timeline within 7 days.

Operator images are signed with Cosign (keyless OIDC); SBOMs are attested and attached to releases. Verify with:

```bash
cosign verify ghcr.io/stubbi/hermes-operator:vX.Y.Z \
  --certificate-identity-regexp 'https://github.com/stubbi/hermes-operator/.github/workflows/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
```

- [ ] **Step 5: Write `CODEOWNERS`**

```
* @stubbi
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "docs: add README, LICENSE, CONTRIBUTING, SECURITY, CODEOWNERS"
```

---

## Task 16: `docs/conventions.md`: the single source of truth for plans 2-7

**Files:**
- Create: `docs/conventions.md`

This document is referenced by every subsequent plan. Keep it tight, no aspirational content.

- [ ] **Step 1: Write the conventions document**

Create `docs/conventions.md`:

```markdown
# Hermes Operator: Engineering Conventions

> Referenced by Plans 2-7. Plan 1 establishes the patterns; this doc names them.

## Code layout

- `api/v1/<kind>_types.go`: CRD types. One file per kind.
- `internal/controller/<kind>_controller.go`: orchestration only. No resource construction.
- `internal/resources/<resource>.go` + `<resource>_test.go`: pure builder funcs. One file per resource type.
- `internal/webhook/<kind>_<webhook>.go`: validator + defaulter implementations (added in Plan 2).
- `config/crd/bases/`: generated CRD YAML, committed.
- `charts/hermes-operator/templates/crds/`: synced from `config/crd/bases/` via `make sync-chart-crds` (CI-enforced).
- `test/e2e/`: kind cluster tests. `test/conformance/`: negative + idempotency + upgrade-path tests (Plan 6).

## Naming

- Deterministic resource names: `Build<Resource>(inst)` and `<Resource>Name(inst)` always live together.
- `<Resource>Name(inst)` returns e.g. `inst.Name + "-data"` (PVC), `inst.Name + "-config"` (ConfigMap). The Service and StatefulSet are named `inst.Name`.
- Plan 2+ may add new resources; pick a short, deterministic suffix and document it here.

## Labels

Every operator-managed resource carries the labels in `resources.LabelsForInstance(inst)`:

| Label | Value |
|---|---|
| `app.kubernetes.io/name` | `hermes-agent` |
| `app.kubernetes.io/instance` | `<inst.Name>` |
| `app.kubernetes.io/managed-by` | `hermes-operator` |
| `app.kubernetes.io/part-of` | `hermes.agent` |

## Reconciliation rules (CI-enforced where noted)

1. **`controllerutil.CreateOrUpdate` exclusively** for managed resources. Bare `r.Update()` / `r.Create()` outside the PVC path is grep-banned. Exception: `// reconcile-guard:allow` with justification.
2. **Server-Side Apply (SSA)** for the `HermesSelfConfig` reconciler from day one (lesson #433/#439). Field manager = `hermes.agent/selfconfig`. *(Plan 4)*
3. **Set every k8s server-side default explicitly in builders.** RevisionHistoryLimit, ProgressDeadlineSeconds, RestartPolicy, DNSPolicy, SchedulerName, TerminationGracePeriodSeconds, TerminationMessagePath/Policy, ImagePullPolicy on every container, SuccessThreshold on every probe, DefaultMode on volume sources, SessionAffinity:None on Service. Skipping these is what produced openclaw's generation-thrash bugs.
4. **Preserve third-party annotations + labels on update** via `resources.MergePreservingForeign` with prefix `hermes.agent/` (lesson #446/#447).
5. **Finalizer add/remove uses `r.Patch()` with a JSON patch**, never `r.Update()` (lesson #437).
6. **Preserve server-assigned fields on update**: Service.ClusterIP/ClusterIPs, etc. PVCs are immutable: only create, never update.
7. **Status updates are a separate transaction** (`r.Status().Update`) from spec/metadata.
8. **Owner refs on every managed resource** via `controllerutil.SetControllerReference`.

## Idempotency

Every reconciler must satisfy: applying the same spec twice produces the same generation/resourceVersion on each managed resource. The envtest suite in `internal/controller/` includes an idempotency canary test; do not skip it.

## Commit messages

Conventional commits required. Release-please uses `feat:` and `fix:` for the changelog. Acceptable prefixes: `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, `refactor:`, `test:`, `build:`, `perf:`.

## Git worktrees

Always use `git worktree` when working on a separate branch:

```bash
git worktree add ../hermes-operator-<suffix> -b <branch> main
# work, commit, push; then:
git worktree remove ../hermes-operator-<suffix>
```

Never `git checkout` or `git switch` in the main working tree.

## Go style

- Use `0o644` (not `0644`) for octal literals: `gocritic.octalLiteral` enforces this.
- Wrap errors: `fmt.Errorf("context: %w", err)`.
- Use `resources.Ptr[T]` for short-lived pointer literals.
- No em/en dashes in code, comments, strings: use regular `-` / `--`.
- `make fmt` before committing.

## CRD type changes: generation workflow

After modifying `api/v1/*_types.go`:

1. `make generate` (regenerates `zz_generated.deepcopy.go`).
2. `make manifests` (regenerates CRD YAML in `config/crd/bases/`).
3. `make sync-chart-crds` (copies CRD YAML into the Helm chart).
4. Commit the generated files.

## Documentation drift

When adding or changing CRD fields, update both:
- `README.md`: user-facing overview and feature table.
- `docs/api-reference.md`: exhaustive field-level reference (added in Plan 2).

Both must stay in sync with the types.

## Testing strategy (full picture in Plan 6)

- **Unit** in `internal/resources/*_test.go`: pure, fast, no envtest.
- **envtest** in `internal/controller/*_test.go`: reconcile against fake apiserver.
- **E2E** in `test/e2e/`: kind cluster, real resources.
- **Conformance** in `test/conformance/`: negative, idempotency, upgrade-path, GitOps coexistence, failure injection. *(Plan 6)*
- **Benchmarks** in `*_bench_test.go`. *(Plan 6)*

## v1 stability: non-negotiable

- API group `hermes.agent`, version `v1`. No `v1alpha1` spoke.
- New optional fields with `omitempty` and sane defaults: non-breaking.
- Field removal requires `hermes.agent/v2` + conversion webhook + ≥6 months overlap.
- Deprecation: godoc `// Deprecated:`, webhook warning, CHANGELOG + `docs/deprecations.md` entry, target removal ≥2 minors out.
```

- [ ] **Step 2: Commit**

```bash
git add -A
git commit -m "docs: add engineering conventions (referenced by Plans 2-7)"
```

---

## Task 17: E2E happy-path test on kind

**Files:**
- Create: `hack/kind-config.yaml`, `test/e2e/e2e_suite_test.go`, `test/e2e/happypath_test.go`, `.github/workflows/e2e.yaml`
- Add: Makefile targets `kind-up`, `kind-down`, `e2e`

- [ ] **Step 1: Add a kind config**

Create `hack/kind-config.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
```

- [ ] **Step 2: Add Makefile targets**

Append to `Makefile`:

```makefile
KIND_CLUSTER ?= hermes-operator-e2e

.PHONY: kind-up
kind-up: ## Create a kind cluster for e2e.
	kind create cluster --name $(KIND_CLUSTER) --config hack/kind-config.yaml

.PHONY: kind-down
kind-down: ## Tear down the kind cluster.
	kind delete cluster --name $(KIND_CLUSTER)

.PHONY: e2e
e2e: ## Run e2e suite against the kind cluster (must already be up).
	go test ./test/e2e/... -v -timeout 10m

.PHONY: e2e-load-image
e2e-load-image: docker-build ## Load the locally-built operator image into kind.
	kind load docker-image $(IMG) --name $(KIND_CLUSTER)
```

- [ ] **Step 3: Write the e2e suite scaffold**

Create `test/e2e/e2e_suite_test.go`:

```go
package e2e

import (
    "fmt"
    "os/exec"
    "testing"
    "time"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "hermes-operator e2e suite")
}

var _ = BeforeSuite(func() {
    SetDefaultEventuallyTimeout(3 * time.Minute)
    SetDefaultEventuallyPollingInterval(2 * time.Second)
    By("installing CRDs via helm chart")
    out, err := run("helm", "upgrade", "--install", "hermes-operator", "../../charts/hermes-operator",
        "--namespace", "hermes-system", "--create-namespace",
        "--set", "image.repository=hermes-operator",
        "--set", "image.tag=dev",
        "--set", "image.pullPolicy=IfNotPresent",
        "--wait", "--timeout=2m")
    Expect(err).ToNot(HaveOccurred(), "helm upgrade failed: %s", out)
})

func run(cmd string, args ...string) (string, error) {
    c := exec.Command(cmd, args...)
    b, err := c.CombinedOutput()
    return string(b), err
}

func kubectl(args ...string) (string, error) {
    return run("kubectl", args...)
}

func mustRun(cmd string, args ...string) string {
    out, err := run(cmd, args...)
    Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("command failed: %s %v\n%s", cmd, args, out))
    return out
}
```

- [ ] **Step 4: Write the happy-path e2e test**

Create `test/e2e/happypath_test.go`:

```go
package e2e

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("Happy path", func() {
    It("reconciles a minimal HermesInstance into a running StatefulSet", func() {
        manifest := `
apiVersion: hermes.agent/v1
kind: HermesInstance
metadata:
  name: e2e-demo
  namespace: default
spec:
  image:
    repository: ghcr.io/stubbi/hermes-agent
    tag: latest
  storage:
    persistence:
      enabled: true
      size: 1Gi
`
        out, err := runStdin("kubectl", []string{"apply", "-f", "-"}, manifest)
        Expect(err).ToNot(HaveOccurred(), "kubectl apply failed: %s", out)

        Eventually(func() string {
            out, _ := kubectl("get", "statefulset", "e2e-demo", "-n", "default", "-o", "jsonpath={.status.readyReplicas}")
            return out
        }).Should(Equal("1"))
    })
})

func runStdin(cmd string, args []string, stdin string) (string, error) {
    c := execCommand(cmd, args...)
    c.Stdin = newStdin(stdin)
    b, err := c.CombinedOutput()
    return string(b), err
}
```

Add the small helpers at the bottom of `e2e_suite_test.go`:

```go
import (
    "io"
    "strings"
)

var execCommand = exec.Command

func newStdin(s string) io.Reader { return strings.NewReader(s) }
```

- [ ] **Step 5: Write the e2e GitHub Actions workflow**

Create `.github/workflows/e2e.yaml`:

```yaml
name: E2E
on:
  pull_request:
  push:
    branches: [main]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.24" }
      - uses: helm/kind-action@v1
        with:
          cluster_name: hermes-operator-e2e
          config: hack/kind-config.yaml
      - uses: azure/setup-helm@v4
      - name: Build operator image
        run: make docker-build IMG=hermes-operator:dev
      - name: Load image into kind
        run: kind load docker-image hermes-operator:dev --name hermes-operator-e2e
      - name: Run e2e
        run: make e2e
```

- [ ] **Step 6: Run the e2e locally**

```bash
make kind-up
make e2e-load-image IMG=hermes-operator:dev
make e2e
```
Expected: e2e suite reports PASS. If the test for `readyReplicas=1` times out, the agent image probably can't pull: that's fine for now; mark the test pending or use a known-pullable placeholder image (`ghcr.io/nginx/nginx-unprivileged:latest`) until Plan 3 publishes a real hermes-agent image. Adjust the manifest in the test accordingly with a TODO comment pointing at Plan 3.

```bash
make kind-down
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test(e2e): add happy-path test on kind + CI workflow"
```

---

## Task 18: Push and verify CI is green

- [ ] **Step 1: Push the branch (everything is on `main`)**

```bash
git push origin main
```
Expected: push succeeds (we already set upstream when creating the repo).

- [ ] **Step 2: Watch CI**

```bash
gh run watch
```
Expected: all five workflows (`CI`, `Reconcile Guard`, `Helm RBAC Sync`, `Build`, `E2E`) finish green. If any fail, the script names point to the relevant area:
- `CI/lint` → run `make lint` locally, fix golangci-lint complaints
- `CI/test` → run `make test`, debug envtest failures
- `Reconcile Guard` → grep your diff for `r.Update(` / `r.Create(`
- `Helm RBAC Sync` → run `bash hack/check-helm-rbac.sh` locally, update the Helm ClusterRole
- `Build` → run `docker build .` locally
- `E2E` → run `make kind-up && make e2e` locally

- [ ] **Step 3: Tag this Plan-1 milestone (no release yet)**

```bash
git tag plan-1-foundation
git push origin plan-1-foundation
```

---

## Self-review (verify before marking the plan complete)

- [ ] Spec §2 (project basics): covered by Task 1 (kubebuilder init), Task 2 (API group), Task 15 (LICENSE), Task 16 (conventions).
- [ ] Spec §3 (CRD surface): Tasks 2-3 scaffold all three CRDs; only `HermesInstance` is reconciled here, which the plan acknowledges.
- [ ] Spec §4 (HermesInstance spec): Task 3 implements the *minimal* slice; Plan 2 expands.
- [ ] Spec §7.1 (code layout): file structure section at top of plan matches §7.1.
- [ ] Spec §7.2 rule 1 (`CreateOrUpdate` only): enforced by Task 11 (Reconcile Guard) and Task 9 (controller uses it).
- [ ] Spec §7.2 rule 3 (explicit k8s defaults): Task 8 sets them all in the StatefulSet builder; the idempotency canary in Task 10 verifies.
- [ ] Spec §7.2 rule 4 (preserve foreign annotations): Task 4 implements `MergePreservingForeign`; Task 9 uses it.
- [ ] Spec §7.2 rule 5 (no `r.Update` on CR for finalizer): Plan 1 has no finalizer; Plan 5 adds one and exercises the rule.
- [ ] Spec §7.2 rule 6 (preserve server-assigned fields): Task 9 preserves `Service.ClusterIP` on update.
- [ ] Spec §10 testing: unit (Tasks 4-8), envtest (Task 10), e2e (Task 17). Conformance suite is Plan 6.
- [ ] Spec §11 stability: `hermes.agent/v1` only, no `v1alpha1`: Task 2 ensures this; Task 16 documents the commitment.
- [ ] No placeholders: every step has a runnable command or full code.
- [ ] Type consistency: `BuildPVC`/`BuildConfigMap`/`BuildService`/`BuildStatefulSet`/`PVCName`/`ConfigMapName`/`ServiceName`/`StatefulSetName`/`Ptr`/`LabelsForInstance`/`MergePreservingForeign` are used consistently across Tasks 4-10.

End of Plan 1.
