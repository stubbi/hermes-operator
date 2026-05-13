package resources

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
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
