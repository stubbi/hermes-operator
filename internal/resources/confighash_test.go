package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func TestHashConfigMapData_DeterministicAndOrderIndependent(t *testing.T) {
	t.Parallel()
	a := HashConfigMapData(map[string]string{"config.yaml": "model: x\n", "other": "1"})
	b := HashConfigMapData(map[string]string{"other": "1", "config.yaml": "model: x\n"})
	assert.Equal(t, a, b)
	assert.Len(t, a, 64)
	assert.Equal(t, HashConfigMapData(nil), HashConfigMapData(map[string]string{}))
}

func TestHashConfigMapData_ChangesWithContentAndKeys(t *testing.T) {
	t.Parallel()
	base := HashConfigMapData(map[string]string{"config.yaml": "model: x\n"})
	assert.NotEqual(t, base, HashConfigMapData(map[string]string{"config.yaml": "model: y\n"}))
	assert.NotEqual(t, base, HashConfigMapData(map[string]string{"config.yml": "model: x\n"}))
	// Length prefixing: key/value boundary shifts must not collide.
	assert.NotEqual(t,
		HashConfigMapData(map[string]string{"a": "bc"}),
		HashConfigMapData(map[string]string{"ab": "c"}))
}

func TestSetConfigHashAnnotations_PreservesExisting(t *testing.T) {
	t.Parallel()
	sts := &appsv1.StatefulSet{}
	sts.Spec.Template.Annotations = map[string]string{"keep": "me"}
	SetConfigHashAnnotations(sts, map[string]string{"config.yaml": "a"}, nil)
	ann := sts.Spec.Template.Annotations
	assert.Equal(t, "me", ann["keep"])
	assert.Equal(t, HashConfigMapData(map[string]string{"config.yaml": "a"}), ann[ConfigHashAnnotation])
	assert.Equal(t, HashConfigMapData(nil), ann[WorkspaceHashAnnotation])
}
