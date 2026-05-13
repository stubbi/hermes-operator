package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
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
	dirs := cm.Data[InitialDirsKey]
	assert.Contains(t, dirs, "data\n")
	assert.Contains(t, dirs, "data/raw\n")
}

func TestBuildWorkspaceConfigMap_EmptyIsStillEmitted(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	cm := BuildWorkspaceConfigMap(inst)
	assert.Equal(t, "demo-workspace", cm.Name)
	assert.NotNil(t, cm.Data)
}
