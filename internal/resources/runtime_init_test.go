package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func instWithRuntime(r hermesv1.RuntimeSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{Runtime: r},
	}
}

func TestBuildRuntimeInitContainers_UVDefault(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(true)},
	})
	got := BuildRuntimeInitContainers(inst)
	assert.Len(t, got, 1, "uv-sync only")
	assert.Equal(t, "init-uv", got[0].Name)
	assert.Contains(t, got[0].Command[2], "uv sync --frozen", "frozen lockfile sync")
	hasFullData := false
	for _, m := range got[0].VolumeMounts {
		if m.Name == "data" && m.MountPath == "/home/hermes/.hermes" && m.SubPath == "" {
			hasFullData = true
		}
	}
	assert.True(t, hasFullData, "init container must mount the full data volume without subPath")
}

func TestBuildRuntimeInitContainers_UVDisabled(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(false)},
	})
	got := BuildRuntimeInitContainers(inst)
	for _, c := range got {
		assert.NotEqual(t, "init-uv", c.Name, "uv sync should be skipped when disabled")
	}
}

func TestBuildRuntimeInitContainers_ExtraApt(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraAptPackages: []string{"poppler-utils", "tesseract-ocr"},
	})
	got := BuildRuntimeInitContainers(inst)
	var aptC *corev1.Container
	for i, c := range got {
		if c.Name == "init-apt" {
			aptC = &got[i]
		}
	}
	if !assert.NotNil(t, aptC, "init-apt container missing") {
		return
	}
	assert.NotNil(t, aptC.SecurityContext)
	assert.NotNil(t, aptC.SecurityContext.RunAsUser)
	assert.Equal(t, int64(0), *aptC.SecurityContext.RunAsUser)
	assert.Contains(t, aptC.Command[2], "apt-get install -y --no-install-recommends 'poppler-utils' 'tesseract-ocr'")
}

func TestBuildRuntimeInitContainers_ExtraPip(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraPipPackages: []string{"pandas==2.2.0", "polars"},
	})
	got := BuildRuntimeInitContainers(inst)
	var pipC *corev1.Container
	for i, c := range got {
		if c.Name == "init-pip" {
			pipC = &got[i]
		}
	}
	if !assert.NotNil(t, pipC, "init-pip container missing") {
		return
	}
	assert.Contains(t, pipC.Command[2], "uv pip install")
	assert.Contains(t, pipC.Command[2], "pandas==2.2.0")
	assert.Contains(t, pipC.Command[2], "polars")
	assert.Contains(t, pipC.Command[2], "/home/hermes/.hermes/.venv-extras")
}

func TestBuildRuntimeInitContainers_Order(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraAptPackages: []string{"libxml2-dev"},
		ExtraPipPackages: []string{"lxml"},
	})
	got := BuildRuntimeInitContainers(inst)
	names := []string{}
	for _, c := range got {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"init-apt", "init-uv", "init-pip"}, names)
}

func TestBuildRuntimeVolumes_UVCacheEmptyDirDefault(t *testing.T) {
	inst := instWithRuntime(hermesv1.RuntimeSpec{
		UV: hermesv1.UVSpec{Enabled: Ptr(true)},
	})
	vols := BuildRuntimeVolumes(inst)
	found := false
	for _, v := range vols {
		if v.Name == "uv-cache" {
			found = true
			assert.NotNil(t, v.EmptyDir, "default to emptyDir")
		}
	}
	assert.True(t, found, "uv-cache volume present when uv enabled")
}
