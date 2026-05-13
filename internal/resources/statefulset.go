package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
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

	// Build PodSecurityContext with override support
	podSecurityCtx := &corev1.PodSecurityContext{
		RunAsNonRoot: Ptr(true),
		RunAsUser:    Ptr(int64(1000)),
		RunAsGroup:   Ptr(int64(1000)),
		FSGroup:      Ptr(int64(1000)),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
	if inst.Spec.Security.PodSecurityContext != nil {
		podSecurityCtx = inst.Spec.Security.PodSecurityContext.DeepCopy()
	}

	// Build ContainerSecurityContext with override support
	containerSecurityCtx := &corev1.SecurityContext{
		AllowPrivilegeEscalation: Ptr(false),
		ReadOnlyRootFilesystem:   Ptr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
	if inst.Spec.Security.ContainerSecurityContext != nil {
		containerSecurityCtx = inst.Spec.Security.ContainerSecurityContext.DeepCopy()
	}

	// Build container with override support
	c := corev1.Container{
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
		SecurityContext: containerSecurityCtx,
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
	}

	// Set resources from spec
	c.Resources = inst.Spec.Resources.ToContainerResourceRequirements()

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

	// --- Plan 3: runtime/gateways/honcho wiring (operator-managed env) ---
	c.Env = append(c.Env, BuildGatewayEnv(inst)...)
	c.Env = append(c.Env, BuildHonchoConsumerEnv(inst)...)
	c.EnvFrom = append(c.EnvFrom, BuildGatewayEnvFrom(inst)...)
	c.VolumeMounts = append(c.VolumeMounts, BuildRuntimeVolumeMounts(inst)...)
	// --- end Plan 3 operator env ---

	// Extend container with extra volume mounts, env, and envFrom
	c.VolumeMounts = append(c.VolumeMounts, inst.Spec.ExtraVolumeMounts...)
	c.Env = append(c.Env, inst.Spec.Env...)
	c.EnvFrom = append(c.EnvFrom, inst.Spec.EnvFrom...)

	// Build PodSpec with scheduling and service account
	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyAlways,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: Ptr(int64(30)),
		SecurityContext:               podSecurityCtx,
		NodeSelector:                  inst.Spec.Scheduling.NodeSelector,
		Tolerations:                   inst.Spec.Scheduling.Tolerations,
		Affinity:                      inst.Spec.Scheduling.Affinity,
		PriorityClassName:             inst.Spec.Scheduling.PriorityClassName,
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

	// Append sidecars and extra volumes (init containers assembled below)
	podSpec.Containers = append(podSpec.Containers, inst.Spec.Sidecars...)
	podSpec.Volumes = append(podSpec.Volumes, inst.Spec.ExtraVolumes...)

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

	// Assemble init containers: extraInits (restore/migration) → operator-managed
	// (runtime-init) → user-supplied. Order matters: restore must populate the PVC
	// before runtime-init starts writing to it.
	inits := append([]corev1.Container{}, extraInits...)
	inits = append(inits, BuildRuntimeInitContainers(inst)...)
	inits = append(inits, inst.Spec.InitContainers...)
	sts.Spec.Template.Spec.InitContainers = inits

	// --- Plan 3: runtime volumes ---
	sts.Spec.Template.Spec.Volumes = append(
		sts.Spec.Template.Spec.Volumes,
		BuildRuntimeVolumes(inst)...,
	)
	// --- end Plan 3 ---

	return sts
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
