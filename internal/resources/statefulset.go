package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

// StatefulSetName returns the deterministic name.
func StatefulSetName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildStatefulSet constructs the desired StatefulSet. Every k8s server-side
// default is set explicitly to avoid metadata.generation thrash on reconcile.
// extraInits is prepended before operator-managed init containers so that
// restore/migration runs BEFORE runtime-init touches the PVC.
func BuildStatefulSet(inst *hermesv1.HermesInstance, extraInits []corev1.Container) *appsv1.StatefulSet {
	labels := LabelsForInstance(inst)
	selector := map[string]string{
		"app.kubernetes.io/name":     "hermes-agent",
		"app.kubernetes.io/instance": inst.Name,
	}
	image := imageRef(inst)

	// Build PodSecurityContext with override support.
	//
	// The upstream hermes-agent image runs under s6-overlay: PID 1 (/init) must
	// start as root so the stage2 hook can remap the in-image `hermes` user to
	// HERMES_UID/HERMES_GID and chown the /opt/data volume, after which every
	// supervised service drops privileges via s6-setuidgid. We therefore cannot
	// pin RunAsNonRoot/RunAsUser here; the in-container drop to uid 1000 is driven
	// by the HERMES_UID/HERMES_GID env (see below). FSGroup still gives the PVC
	// group ownership the dropped user needs.
	podSecurityCtx := &corev1.PodSecurityContext{
		FSGroup: Ptr(int64(1000)),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
	if inst.Spec.Security.PodSecurityContext != nil {
		podSecurityCtx = inst.Spec.Security.PodSecurityContext.DeepCopy()
	}

	// Build ContainerSecurityContext with override support.
	//
	// s6 stage2 needs CHOWN/SETUID/SETGID/DAC_OVERRIDE/FOWNER to remap the user
	// and chown the volume, and a writable root filesystem (/run, /etc for the
	// supervision tree), so we don't drop ALL caps or force a read-only rootfs the
	// way the old hand-rolled image allowed. Privilege escalation stays disabled.
	containerSecurityCtx := &corev1.SecurityContext{
		AllowPrivilegeEscalation: Ptr(false),
	}
	if inst.Spec.Security.ContainerSecurityContext != nil {
		containerSecurityCtx = inst.Spec.Security.ContainerSecurityContext.DeepCopy()
	}

	// Build container with override support
	c := corev1.Container{
		Name:            "hermes",
		Image:           image,
		ImagePullPolicy: pullPolicy(inst),
		// CMD args route through the image's main-wrapper to `hermes gateway run`
		// — the foreground gateway daemon. With the API server enabled (env below)
		// it serves the OpenAI-compatible endpoint + /health on the gateway port.
		Args:                     []string{"gateway", "run"},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Ports: []corev1.ContainerPort{{
			Name:          GatewayPortName,
			ContainerPort: GatewayPort,
			Protocol:      corev1.ProtocolTCP,
		}},
		SecurityContext: containerSecurityCtx,
		VolumeMounts: []corev1.VolumeMount{
			// HERMES_HOME on the upstream image is /opt/data (the persistent volume).
			// config.yaml lives on the data volume and is written by the
			// config-seed init container (BuildConfigSeedContainer), not mounted
			// from the ConfigMap: the agent migrates it in place on schema bumps
			// and a read-only subPath mount made that fail silently (#160).
			{Name: "data", MountPath: "/opt/data"},
			{Name: "tmp", MountPath: "/tmp"},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromString(GatewayPortName),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      2,
			FailureThreshold:    6,
			SuccessThreshold:    1, // explicit k8s default
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromString(GatewayPortName),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       15,
			TimeoutSeconds:      2,
			FailureThreshold:    3,
			SuccessThreshold:    1,
		},
	}

	// Set resources from spec, falling back to the operator defaults for
	// whichever side the spec leaves unset.
	c.Resources = agentResources(inst)

	// Set probe overrides
	if inst.Spec.Probes.Liveness != nil {
		c.LivenessProbe = inst.Spec.Probes.Liveness.DeepCopy()
	}
	if inst.Spec.Probes.Readiness != nil {
		c.ReadinessProbe = inst.Spec.Probes.Readiness.DeepCopy()
	}
	if inst.Spec.Probes.Startup != nil {
		c.StartupProbe = inst.Spec.Probes.Startup.DeepCopy()
	}

	// Mount workspace ConfigMap (unconditional)
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
		Name:      "workspace",
		MountPath: "/home/hermes/.hermes-workspace-seed",
		ReadOnly:  true,
	})

	// Prepare CA bundle volume source if configured
	var caBundleVolumeSource *corev1.VolumeSource
	if inst.Spec.Security.CABundle.ConfigMapName != "" || inst.Spec.Security.CABundle.SecretName != "" {
		key := inst.Spec.Security.CABundle.Key
		if key == "" {
			key = "ca.crt"
		}
		switch {
		case inst.Spec.Security.CABundle.ConfigMapName != "":
			caBundleVolumeSource = &corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: inst.Spec.Security.CABundle.ConfigMapName},
					Items:                []corev1.KeyToPath{{Key: key, Path: "ca.crt"}},
					DefaultMode:          Ptr(int32(0o644)),
				},
			}
		case inst.Spec.Security.CABundle.SecretName != "":
			caBundleVolumeSource = &corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  inst.Spec.Security.CABundle.SecretName,
					Items:       []corev1.KeyToPath{{Key: key, Path: "ca.crt"}},
					DefaultMode: Ptr(int32(0o644)),
				},
			}
		}
		// Mount CA bundle into container
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

	// --- Agent runtime env (upstream s6 image contract) ---
	// HERMES_HOME points the agent at the persistent volume; HERMES_UID/GID drive
	// the s6 stage2 privilege drop to a non-root user. API_SERVER_* enables the
	// OpenAI-compatible gateway API + /health on the gateway port — the readiness
	// surface. The key authenticates /v1; /health is unauthenticated.
	c.Env = append(c.Env, BuildAgentRuntimeEnv(inst)...)

	// --- Plan 3: runtime/gateways/honcho wiring (operator-managed env) ---
	c.Env = append(c.Env, BuildGatewayEnv(inst)...)
	c.Env = append(c.Env, BuildHonchoConsumerEnv(inst)...)
	c.EnvFrom = append(c.EnvFrom, BuildGatewayEnvFrom(inst)...)
	// --- end Plan 3 operator env ---

	// Extend container with extra volume mounts, env, and envFrom
	c.VolumeMounts = append(c.VolumeMounts, inst.Spec.ExtraVolumeMounts...)
	c.Env = append(c.Env, inst.Spec.Env...)
	c.EnvFrom = append(c.EnvFrom, inst.Spec.EnvFrom...)

	// Secrets broker: wraps the entrypoint and adds its env last so the
	// operator-rendered config wins over a same-named user env var.
	brokerInit, brokerVol := applySecretsBroker(inst, &c)

	// Build PodSpec with scheduling and service account
	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyAlways,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: Ptr(int64(30)),
		ShareProcessNamespace:         shareProcessNamespace(inst),
		SecurityContext:               podSecurityCtx,
		NodeSelector:                  inst.Spec.Scheduling.NodeSelector,
		Tolerations:                   inst.Spec.Scheduling.Tolerations,
		Affinity:                      inst.Spec.Scheduling.Affinity,
		PriorityClassName:             inst.Spec.Scheduling.PriorityClassName,
		RuntimeClassName:              runtimeClassName(inst),
		TopologySpreadConstraints:     inst.Spec.Availability.TopologySpreadConstraints,
		ServiceAccountName:            ServiceAccountNameFor(inst),
		Containers:                    []corev1.Container{c},
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
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: PVCName(inst),
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
	}

	// Append workspace volume (unconditional)
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: WorkspaceConfigMapName(inst)},
				DefaultMode:          Ptr(int32(0o644)),
			},
		},
	})

	// Append CA bundle volume if configured
	if caBundleVolumeSource != nil {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{Name: "ca-bundle", VolumeSource: *caBundleVolumeSource})
	}

	// Operator-managed tailscale sidecar goes BEFORE user sidecars. Its serve
	// config comes from the instance ConfigMap (key remapped to serve.json) and
	// it gets a dedicated /tmp emptyDir: containerboot writes its socket, state,
	// and TLS certs under /tmp, and the pod-level "tmp" volume is deliberately
	// not shared so the hermes container cannot reach tailscaled's LocalAPI
	// socket.
	if ts := BuildTailscaleSidecar(inst); ts != nil {
		podSpec.Containers = append(podSpec.Containers, *ts)
		podSpec.Volumes = append(podSpec.Volumes,
			corev1.Volume{
				Name: tailscaleServeVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapName(inst)},
						Items:                []corev1.KeyToPath{{Key: tailscaleServeKey, Path: tailscaleServeFile}},
						DefaultMode:          Ptr(int32(0o644)),
					},
				},
			},
			corev1.Volume{
				Name: tailscaleTmpVolume,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		)
	}

	// Append sidecars and extra volumes (init containers assembled below)
	podSpec.Containers = append(podSpec.Containers, inst.Spec.Sidecars...)
	podSpec.Volumes = append(podSpec.Volumes, inst.Spec.ExtraVolumes...)
	if brokerVol != nil {
		podSpec.Volumes = append(podSpec.Volumes, *brokerVol)
	}

	// Determine replicas based on suspended state
	replicas := int32(1)
	if inst.Spec.Suspended {
		replicas = 0
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName(inst),
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:          ServiceName(inst),
			Replicas:             Ptr(replicas),
			RevisionHistoryLimit: Ptr(int32(10)),
			PodManagementPolicy:  appsv1.OrderedReadyPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: Ptr(int32(0)),
				},
			},
			// Explicitly set the PVC retention policy to avoid unnecessary spec updates on
			// reconcile when the API server has already defaulted this field.
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}

	// Assemble init containers: extraInits (restore/migration), config-seed, then
	// user-supplied.
	// The upstream s6 image is self-contained (its own Python env, browser, node,
	// deps), so the old runtime-init chain (init-apt/init-uv/init-pip that uv-sync'd
	// the env onto the PVC) is gone — the agent's runtime lives in the image, only
	// mutable state lives on /opt/data.
	//
	// config-seed runs after extraInits so a restored or migrated volume still
	// gets the declared config.yaml, and before user init containers so they see
	// the final file.
	inits := append([]corev1.Container{}, extraInits...)
	if brokerInit != nil {
		inits = append(inits, *brokerInit)
	}
	inits = append(inits, BuildConfigSeedContainer(inst, containerSecurityCtx.DeepCopy()))
	inits = append(inits, inst.Spec.InitContainers...)
	sts.Spec.Template.Spec.InitContainers = inits

	return sts
}

// shareProcessNamespace returns the effective ShareProcessNamespace value,
// defaulting to false. The upstream hermes-agent image runs s6-overlay, whose
// /init MUST be PID 1 (s6-overlay-suexec aborts otherwise). Sharing the process
// namespace inserts the infrastructure (pause) container as PID 1, so it is
// incompatible with s6 — and unnecessary, because s6 reaps zombies itself
// (non-blocking on SIGCHLD), which is exactly why the upstream image replaced
// tini with s6. An explicit opt-in via spec is still honored.
func shareProcessNamespace(inst *hermesv1.HermesInstance) *bool {
	if inst.Spec.ShareProcessNamespace != nil {
		return inst.Spec.ShareProcessNamespace
	}
	return Ptr(false)
}

// Built-in resource floor for the agent container. `spec.resources` is
// optional, so before #124 a rendered instance carried `resources: {}` and the
// container ran unbounded in the BestEffort QoS class. That is the wrong
// default for a workload that executes model-driven code: one runaway instance
// can starve every other pod on its node, and BestEffort is the first thing the
// kubelet evicts under pressure.
//
// The requests are deliberately modest. The limits are deliberately generous:
// the shipped agent image bundles a browser stack (Playwright/Chromium), whose
// working set dwarfs the idle agent, so a tight memory limit would OOM-kill
// real instances on upgrade rather than catch runaways.
var (
	defaultAgentRequests = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("500m"),
		corev1.ResourceMemory: resource.MustParse("512Mi"),
	}
	defaultAgentLimits = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}
)

// agentResources resolves the agent container's resources. Precedence, highest
// first: the instance's own spec.resources, then HermesClusterDefaults (already
// folded into the instance by the defaulting webhook), then the operator
// built-ins above.
//
// Requests and limits are resolved independently, matching the granularity
// ApplyClusterDefaults already uses: a spec that sets only requests keeps those
// verbatim and picks up the default limits. spec.resources.applyOperatorDefaults
// = false opts out of the built-ins for whichever side is unset, leaving it
// genuinely unbounded.
func agentResources(inst *hermesv1.HermesInstance) corev1.ResourceRequirements {
	out := inst.Spec.Resources.ToContainerResourceRequirements()
	if inst.Spec.Resources.ApplyOperatorDefaults != nil && !*inst.Spec.Resources.ApplyOperatorDefaults {
		return out
	}
	if out.Requests == nil {
		out.Requests = defaultAgentRequests.DeepCopy()
	}
	if out.Limits == nil {
		out.Limits = defaultAgentLimits.DeepCopy()
	}
	return out
}

// runtimeClassName maps the spec field onto the pod's optional
// RuntimeClassName. Unset must stay nil rather than a pointer to "": an empty
// string is not the cluster default, it is a RuntimeClass whose name is empty,
// which the API server rejects.
func runtimeClassName(inst *hermesv1.HermesInstance) *string {
	if inst.Spec.Scheduling.RuntimeClassName == "" {
		return nil
	}
	return Ptr(inst.Spec.Scheduling.RuntimeClassName)
}

func imageRef(inst *hermesv1.HermesInstance) string {
	repo := inst.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/paperclipinc/hermes-agent"
	}
	if digest := inst.Spec.Image.Digest; digest != "" {
		return fmt.Sprintf("%s@%s", repo, digest)
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
