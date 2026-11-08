package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

func TestBuildStatefulSet_NameNamespaceLabels(t *testing.T) {
	sts := BuildStatefulSet(minimalInstance(), nil)
	assert.Equal(t, "demo", sts.Name)
	assert.Equal(t, "agents", sts.Namespace)
	assert.Equal(t, "hermes-agent", sts.Labels["app.kubernetes.io/name"])
	assert.Equal(t, "demo", sts.Spec.ServiceName, "matches Service name for stable DNS")
}

func TestBuildStatefulSet_ContainerImage(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Image.Repository = "ghcr.io/stubbi/hermes-agent"
	inst.Spec.Image.Tag = "v1.0.0"
	sts := BuildStatefulSet(inst, nil)
	require := sts.Spec.Template.Spec.Containers
	assert.Len(t, require, 1)
	assert.Equal(t, "ghcr.io/stubbi/hermes-agent:v1.0.0", require[0].Image)
	assert.Equal(t, corev1.PullIfNotPresent, require[0].ImagePullPolicy, "explicit default")
}

func TestBuildStatefulSet_ContainerImageDigest(t *testing.T) {
	inst := minimalInstance()
	inst.Spec.Image.Repository = "ghcr.io/stubbi/hermes-agent"
	inst.Spec.Image.Tag = "v1.0.0"
	inst.Spec.Image.Digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	sts := BuildStatefulSet(inst, nil)
	c := sts.Spec.Template.Spec.Containers
	assert.Len(t, c, 1)
	assert.Equal(t,
		"ghcr.io/stubbi/hermes-agent@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		c[0].Image,
		"digest takes precedence over tag",
	)
}

func TestBuildStatefulSet_ShareProcessNamespace(t *testing.T) {
	t.Parallel()

	// Default: process namespace NOT shared. The upstream s6 image requires
	// /init to be PID 1, which sharing the PID namespace would break.
	sts := BuildStatefulSet(minimalInstance(), nil)
	require.NotNil(t, sts.Spec.Template.Spec.ShareProcessNamespace)
	assert.False(t, *sts.Spec.Template.Spec.ShareProcessNamespace, "defaults to false for s6 (PID 1 must be /init)")

	// Explicit opt-in still honored.
	inst := minimalInstance()
	inst.Spec.ShareProcessNamespace = Ptr(true)
	sts = BuildStatefulSet(inst, nil)
	require.NotNil(t, sts.Spec.Template.Spec.ShareProcessNamespace)
	assert.True(t, *sts.Spec.Template.Spec.ShareProcessNamespace, "honors explicit opt-in")
}

func TestBuildStatefulSet_ExplicitK8sDefaults(t *testing.T) {
	sts := BuildStatefulSet(minimalInstance(), nil)
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
	sts := BuildStatefulSet(minimalInstance(), nil)
	pc := sts.Spec.Template.Spec.SecurityContext
	cc := sts.Spec.Template.Spec.Containers[0].SecurityContext

	// Pod-level: the upstream s6 image must start as root (PID 1 /init remaps the
	// in-image user + chowns /opt/data), so RunAsNonRoot/RunAsUser/RunAsGroup are
	// NO LONGER pinned. FSGroup + RuntimeDefault seccomp remain.
	assert.Nil(t, pc.RunAsNonRoot, "no longer forced non-root (s6 needs root PID 1)")
	assert.Nil(t, pc.RunAsUser)
	assert.Nil(t, pc.RunAsGroup)
	assert.NotNil(t, pc.FSGroup)
	assert.Equal(t, int64(1000), *pc.FSGroup)
	assert.NotNil(t, pc.SeccompProfile)
	assert.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, pc.SeccompProfile.Type)

	// Container-level: only privilege escalation stays disabled. s6 stage2 needs a
	// writable rootfs and CHOWN/SETUID/SETGID caps, so ReadOnlyRootFilesystem and
	// Capabilities.Drop=[ALL] are NO LONGER set.
	assert.NotNil(t, cc.AllowPrivilegeEscalation)
	assert.False(t, *cc.AllowPrivilegeEscalation)
	assert.Nil(t, cc.ReadOnlyRootFilesystem, "writable rootfs required by s6")
	assert.Nil(t, cc.Capabilities, "no longer drops ALL caps (s6 stage2 needs CHOWN/SETUID/SETGID)")
}

func TestBuildStatefulSet_VolumesAndMounts(t *testing.T) {
	sts := BuildStatefulSet(minimalInstance(), nil)
	c := sts.Spec.Template.Spec.Containers[0]

	mountNames := map[string]string{}
	for _, m := range c.VolumeMounts {
		mountNames[m.Name] = m.MountPath
	}
	assert.Equal(t, "/opt/data", mountNames["data"], "PVC mounted at HERMES_HOME (/opt/data)")
	assert.Equal(t, "/opt/data/config.yaml", mountNames["config"], "configmap subPath at config.yaml")
	assert.Equal(t, "/tmp", mountNames["tmp"], "writable /tmp")
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
	sts := BuildStatefulSet(inst, nil)
	c := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("100m"), c.Resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("512Mi"), c.Resources.Limits[corev1.ResourceMemory])
}

// An instance that says nothing about resources must not render
// `resources: {}`. A BestEffort agent container executing model-driven code can
// starve its node and is the kubelet's first eviction candidate (#124).
func TestBuildStatefulSet_DefaultResourcesWhenUnset(t *testing.T) {
	t.Parallel()
	sts := BuildStatefulSet(minimalInstance(), nil)
	c := sts.Spec.Template.Spec.Containers[0]

	assert.Equal(t, resource.MustParse("500m"), c.Resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("512Mi"), c.Resources.Requests[corev1.ResourceMemory])
	assert.Equal(t, resource.MustParse("2"), c.Resources.Limits[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("4Gi"), c.Resources.Limits[corev1.ResourceMemory])
}

// Requests and limits default independently: setting one side must not drag in
// the operator value for the keys the user did set, and must not suppress the
// default on the side left unset.
func TestBuildStatefulSet_DefaultResourcesPerSide(t *testing.T) {
	t.Parallel()

	t.Run("requests set, limits defaulted", func(t *testing.T) {
		inst := minimalInstance()
		inst.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("50m"),
		}
		c := BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0]

		assert.Equal(t, resource.MustParse("50m"), c.Resources.Requests[corev1.ResourceCPU])
		_, hasMem := c.Resources.Requests[corev1.ResourceMemory]
		assert.False(t, hasMem, "an explicit requests block is used verbatim, not key-merged")
		assert.Equal(t, resource.MustParse("4Gi"), c.Resources.Limits[corev1.ResourceMemory])
	})

	t.Run("limits set, requests defaulted", func(t *testing.T) {
		inst := minimalInstance()
		inst.Spec.Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		}
		c := BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0]

		assert.Equal(t, resource.MustParse("8Gi"), c.Resources.Limits[corev1.ResourceMemory])
		assert.Equal(t, resource.MustParse("500m"), c.Resources.Requests[corev1.ResourceCPU])
	})
}

// The opt-out exists so an instance whose working set exceeds the default
// limits is not OOM-killed by an operator upgrade.
func TestBuildStatefulSet_ApplyOperatorDefaultsFalse(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Resources.ApplyOperatorDefaults = Ptr(false)
	c := BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0]

	assert.Nil(t, c.Resources.Requests)
	assert.Nil(t, c.Resources.Limits)
}

// Explicitly true is the same as unset.
func TestBuildStatefulSet_ApplyOperatorDefaultsTrue(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Resources.ApplyOperatorDefaults = Ptr(true)
	c := BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0]

	assert.Equal(t, resource.MustParse("500m"), c.Resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("4Gi"), c.Resources.Limits[corev1.ResourceMemory])
}

// The package-level defaults are shared, so a builder that handed them out
// directly would let one instance's mutation leak into every other instance.
func TestBuildStatefulSet_DefaultResourcesAreNotAliased(t *testing.T) {
	t.Parallel()
	first := BuildStatefulSet(minimalInstance(), nil).Spec.Template.Spec.Containers[0]
	first.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("1Mi")

	second := BuildStatefulSet(minimalInstance(), nil).Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("4Gi"), second.Resources.Limits[corev1.ResourceMemory])
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
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
	podSpec := sts.Spec.Template.Spec
	assert.Equal(t, "ssd", podSpec.NodeSelector["disktype"])
	assert.Len(t, podSpec.Tolerations, 1)
	assert.Equal(t, "hi", podSpec.PriorityClassName)
	assert.NotNil(t, podSpec.Affinity)
}

func TestBuildStatefulSet_RuntimeClassName(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Scheduling.RuntimeClassName = "gvisor"

	podSpec := BuildStatefulSet(inst, nil).Spec.Template.Spec

	if assert.NotNil(t, podSpec.RuntimeClassName) {
		assert.Equal(t, "gvisor", *podSpec.RuntimeClassName)
	}
}

// Unset must render nil, not a pointer to "". An empty RuntimeClassName is not
// "cluster default" — it names a RuntimeClass with an empty name, which the API
// server rejects.
func TestBuildStatefulSet_RuntimeClassNameUnsetIsNil(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()

	podSpec := BuildStatefulSet(inst, nil).Spec.Template.Spec

	assert.Nil(t, podSpec.RuntimeClassName)
}

func TestBuildStatefulSet_TopologySpread(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Availability.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
		{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
	}
	sts := BuildStatefulSet(inst, nil)
	assert.Len(t, sts.Spec.Template.Spec.TopologySpreadConstraints, 1)
}

func TestBuildStatefulSet_InitContainersAndSidecars(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.InitContainers = []corev1.Container{{Name: "user-init", Image: "alpine"}}
	inst.Spec.Sidecars = []corev1.Container{{Name: "user-side", Image: "alpine"}}
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
	assert.Equal(t, "demo", sts.Spec.Template.Spec.ServiceAccountName)

	inst.Spec.Security.RBAC.ServiceAccountName = "byo-sa"
	sts2 := BuildStatefulSet(inst, nil)
	assert.Equal(t, "byo-sa", sts2.Spec.Template.Spec.ServiceAccountName)
}

func TestBuildStatefulSet_WorkspaceVolumeMounted(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Workspace.InitialFiles = []hermesv1.WorkspaceFile{{Path: "a.md", Content: "x"}}
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
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
	sts := BuildStatefulSet(inst, nil)
	assert.NotNil(t, sts.Spec.Replicas)
	assert.Equal(t, int32(0), *sts.Spec.Replicas)
}

func TestBuildStatefulSet_NotSuspendedDefaultReplica(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	sts := BuildStatefulSet(inst, nil)
	assert.NotNil(t, sts.Spec.Replicas)
	assert.Equal(t, int32(1), *sts.Spec.Replicas)
}

func TestBuildStatefulSet_RuntimeInitContainersNotAppended(t *testing.T) {
	t.Parallel()
	// The upstream s6 image is self-contained, so the operator-managed runtime-init
	// chain (init-apt/init-uv/init-pip) is NO LONGER added to the StatefulSet even
	// when the (now-deprecated, ignored) spec.runtime fields are set.
	inst := minimalInstance()
	//nolint:staticcheck // SA1019: deliberately exercises the deprecated, ignored spec.runtime to prove it's a no-op.
	inst.Spec.Runtime = hermesv1.RuntimeSpec{
		UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
		ExtraPipPackages: []string{"polars"},
	}
	sts := BuildStatefulSet(inst, nil)
	names := []string{}
	for _, c := range sts.Spec.Template.Spec.InitContainers {
		names = append(names, c.Name)
	}
	assert.NotContains(t, names, "init-apt")
	assert.NotContains(t, names, "init-uv")
	assert.NotContains(t, names, "init-pip")
}

func TestBuildStatefulSet_GatewayEnvWired(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Gateways = hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg"},
				Key:                  "token",
			},
		},
	}
	sts := BuildStatefulSet(inst, nil)
	c := sts.Spec.Template.Spec.Containers[0]
	hasToken := false
	for _, e := range c.Env {
		if e.Name == "TELEGRAM_BOT_TOKEN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef.Name == "tg" {
			hasToken = true
		}
	}
	assert.True(t, hasToken, "TELEGRAM_BOT_TOKEN sourced from tg secret")
}

func TestBuildStatefulSet_HonchoEnvWired(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.ProfileStore = hermesv1.ProfileStoreSpec{
		Honcho: hermesv1.HonchoSpec{
			Enabled: Ptr(true),
			APIKeySecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
				Key:                  "api-key",
			},
		},
	}
	sts := BuildStatefulSet(inst, nil)
	c := sts.Spec.Template.Spec.Containers[0]
	byName := map[string]corev1.EnvVar{}
	for _, e := range c.Env {
		byName[e.Name] = e
	}
	assert.Equal(t, "http://demo-honcho:8000", byName["HONCHO_BASE_URL"].Value)
	assert.NotNil(t, byName["HONCHO_API_KEY"].ValueFrom)
}

func TestBuildStatefulSet_NoUVCacheVolume(t *testing.T) {
	t.Parallel()
	// The runtime-init chain is gone, so there is no longer a uv-cache volume or
	// mount on the StatefulSet even when UV is enabled — the agent's runtime lives
	// in the upstream image.
	inst := minimalInstance()
	//nolint:staticcheck // SA1019: deliberately exercises the deprecated, ignored spec.runtime to prove it's a no-op.
	inst.Spec.Runtime = hermesv1.RuntimeSpec{UV: hermesv1.UVSpec{Enabled: Ptr(true)}}
	sts := BuildStatefulSet(inst, nil)
	for _, v := range sts.Spec.Template.Spec.Volumes {
		assert.NotEqual(t, "uv-cache", v.Name, "uv-cache volume no longer present")
	}
	for _, m := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
		assert.NotEqual(t, "uv-cache", m.Name, "uv-cache mount no longer present")
	}
}

func TestBuildStatefulSet_IdempotentWithRuntimeGatewaysHoncho(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	//nolint:staticcheck // SA1019: deliberately exercises the deprecated, ignored spec.runtime to prove it's a no-op.
	inst.Spec.Runtime = hermesv1.RuntimeSpec{UV: hermesv1.UVSpec{Enabled: Ptr(true)}}
	inst.Spec.Gateways = hermesv1.GatewaysSpec{
		Telegram: hermesv1.TelegramGatewaySpec{
			Enabled: Ptr(true),
			BotTokenSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tg"}, Key: "token",
			},
		},
	}
	inst.Spec.ProfileStore = hermesv1.ProfileStoreSpec{
		Honcho: hermesv1.HonchoSpec{
			Enabled:         Ptr(true),
			APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho"}, Key: "api-key"},
		},
	}
	a := BuildStatefulSet(inst, nil)
	b := BuildStatefulSet(inst, nil)
	assert.Equal(t, a, b, "pure builder must be deterministic")
}

func TestBuildStatefulSet_AcceptsInitContainers(t *testing.T) {
	inst := minimalInstance()
	initC := corev1.Container{Name: "init-restore", Image: "restic/restic:0.16.4"}
	sts := BuildStatefulSet(inst, []corev1.Container{initC})
	require.NotNil(t, sts)
	// extraInits (restore/migration) come first, before any user-supplied
	// spec.InitContainers: restore writes to the PVC before anything else touches it.
	require.NotEmpty(t, sts.Spec.Template.Spec.InitContainers)
	assert.Equal(t, "init-restore", sts.Spec.Template.Spec.InitContainers[0].Name)
}

func TestBuildStatefulSet_TailscaleSidecar(t *testing.T) {
	t.Parallel()
	inst := tailscaleInstance()
	sts := BuildStatefulSet(inst, nil)

	var ts *corev1.Container
	for i := range sts.Spec.Template.Spec.Containers {
		if sts.Spec.Template.Spec.Containers[i].Name == "tailscale" {
			ts = &sts.Spec.Template.Spec.Containers[i]
		}
	}
	require.NotNil(t, ts, "tailscale sidecar must be present when enabled")
	assert.Equal(t, "tailscale/tailscale:v1.86.2", ts.Image)

	// serve-config volume from the instance ConfigMap with key->path mapping
	var serveVol *corev1.Volume
	var tmpVol *corev1.Volume
	for i := range sts.Spec.Template.Spec.Volumes {
		switch sts.Spec.Template.Spec.Volumes[i].Name {
		case "tailscale-serve":
			serveVol = &sts.Spec.Template.Spec.Volumes[i]
		case "tailscale-tmp":
			tmpVol = &sts.Spec.Template.Spec.Volumes[i]
		}
	}
	require.NotNil(t, serveVol, "serve-config volume must exist")
	require.NotNil(t, serveVol.ConfigMap)
	assert.Equal(t, ConfigMapName(inst), serveVol.ConfigMap.Name, "serve config comes from the instance ConfigMap")
	require.Len(t, serveVol.ConfigMap.Items, 1)
	assert.Equal(t, "tailscale-serve.json", serveVol.ConfigMap.Items[0].Key)
	assert.Equal(t, "serve.json", serveVol.ConfigMap.Items[0].Path)
	require.NotNil(t, tmpVol, "tailscale tmp emptyDir must exist")
	require.NotNil(t, tmpVol.EmptyDir)

	// Disabled: no sidecar, no tailscale volumes.
	stsOff := BuildStatefulSet(minimalInstance(), nil)
	for _, c := range stsOff.Spec.Template.Spec.Containers {
		assert.NotEqual(t, "tailscale", c.Name)
	}
	for _, v := range stsOff.Spec.Template.Spec.Volumes {
		assert.NotEqual(t, "tailscale-serve", v.Name)
		assert.NotEqual(t, "tailscale-tmp", v.Name)
	}
}
