package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
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
	assert.Equal(t, "demo", sts.Spec.Template.Spec.ServiceAccountName)

	inst.Spec.Security.RBAC.ServiceAccountName = "byo-sa"
	sts2 := BuildStatefulSet(inst)
	assert.Equal(t, "byo-sa", sts2.Spec.Template.Spec.ServiceAccountName)
}

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
