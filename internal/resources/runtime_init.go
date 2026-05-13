package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BuildRuntimeInitContainers returns the ordered init containers required by
// spec.runtime. Order: init-apt → init-uv → init-pip. Each container mounts the
// full data volume (no subPath, lesson openclaw #450).
func BuildRuntimeInitContainers(inst *hermesv1.HermesInstance) []corev1.Container {
	var out []corev1.Container
	if len(inst.Spec.Runtime.ExtraAptPackages) > 0 {
		out = append(out, buildAptInit(inst))
	}
	if uvEnabled(inst) {
		out = append(out, buildUVSyncInit(inst))
	}
	if len(inst.Spec.Runtime.ExtraPipPackages) > 0 {
		out = append(out, buildPipInit(inst))
	}
	return out
}

// BuildRuntimeVolumes returns additional Volumes beyond data PVC + config CM.
func BuildRuntimeVolumes(inst *hermesv1.HermesInstance) []corev1.Volume {
	var out []corev1.Volume
	if !uvEnabled(inst) {
		return out
	}
	cache := inst.Spec.Runtime.UV.CacheVolume
	vol := corev1.Volume{Name: "uv-cache"}
	switch {
	case cache.PersistentVolumeClaim != nil:
		vol.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: cache.PersistentVolumeClaim}
	case cache.EmptyDir != nil:
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: cache.EmptyDir}
	default:
		size := resource.MustParse("1Gi")
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &size}}
	}
	out = append(out, vol)
	return out
}

// BuildRuntimeVolumeMounts returns the additional mounts for the main hermes
// container.
func BuildRuntimeVolumeMounts(inst *hermesv1.HermesInstance) []corev1.VolumeMount {
	if !uvEnabled(inst) {
		return nil
	}
	return []corev1.VolumeMount{
		{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
	}
}

func uvEnabled(inst *hermesv1.HermesInstance) bool {
	if inst.Spec.Runtime.UV.Enabled == nil {
		return true
	}
	return *inst.Spec.Runtime.UV.Enabled
}

func dataVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{Name: "data", MountPath: "/home/hermes/.hermes"}
}

func buildUVSyncInit(inst *hermesv1.HermesInstance) corev1.Container {
	extra := inst.Spec.Runtime.UV.ExtraIndexURL
	indexArg := ""
	if extra != "" {
		indexArg = fmt.Sprintf("--extra-index-url=%s ", shellQuote(extra))
	}
	cmd := fmt.Sprintf(
		"set -eu; cd /home/hermes/.hermes; cp /opt/venv-template/pyproject.toml /opt/venv-template/uv.lock .; uv sync --frozen %s",
		indexArg,
	)
	return corev1.Container{
		Name:                     "init-uv",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(int64(1000)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
			{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
		},
	}
}

func buildAptInit(inst *hermesv1.HermesInstance) corev1.Container {
	pkgs := strings.Join(quoteEach(inst.Spec.Runtime.ExtraAptPackages), " ")
	cmd := fmt.Sprintf(
		"set -eu; apt-get update; apt-get install -y --no-install-recommends %s; rm -rf /var/lib/apt/lists/*",
		pkgs,
	)
	return corev1.Container{
		Name:                     "init-apt",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(false),
			RunAsUser:                Ptr(int64(0)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}, Add: []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETUID", "SETGID"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
		},
	}
}

func buildPipInit(inst *hermesv1.HermesInstance) corev1.Container {
	venvPath := "/home/hermes/.hermes/.venv-extras"
	pkgs := strings.Join(quoteEach(inst.Spec.Runtime.ExtraPipPackages), " ")
	cmd := fmt.Sprintf(
		"set -eu; test -d %[1]s || uv venv %[1]s; VIRTUAL_ENV=%[1]s uv pip install %[2]s",
		venvPath, pkgs,
	)
	return corev1.Container{
		Name:                     "init-pip",
		Image:                    imageRef(inst),
		ImagePullPolicy:          pullPolicy(inst),
		Command:                  []string{"/bin/sh", "-c", cmd},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(int64(1000)),
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			dataVolumeMount(),
			{Name: "uv-cache", MountPath: "/home/hermes/.cache/uv"},
		},
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func quoteEach(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = shellQuote(s)
	}
	return out
}
