package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HonchoDeploymentName, HonchoServiceName, HonchoPVCName return deterministic
// resource names. The PVC name is locked because Plan 4 Task 11 hard-codes
// `<inst>-honcho-data`.
func HonchoDeploymentName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho"
}
func HonchoServiceName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho"
}
func HonchoPVCName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-honcho-data"
}

// HonchoLabels returns labels for the Honcho sub-stack.
func HonchoLabels(inst *hermesv1.HermesInstance) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "honcho",
		"app.kubernetes.io/instance":   HonchoDeploymentName(inst),
		"app.kubernetes.io/managed-by": "hermes-operator",
		"app.kubernetes.io/part-of":    "hermes.agent",
		"hermes.agent/instance":        inst.Name,
	}
}

// honchoEnabled reports whether the user opted into Honcho.
func honchoEnabled(inst *hermesv1.HermesInstance) bool {
	return BoolValue(inst.Spec.ProfileStore.Honcho.Enabled)
}

// BuildHonchoPVC returns the Honcho data PVC (5Gi default).
func BuildHonchoPVC(inst *hermesv1.HermesInstance) *corev1.PersistentVolumeClaim {
	p := inst.Spec.ProfileStore.Honcho.Persistence
	size := p.Size
	if size == "" {
		size = "5Gi"
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoPVCName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
			StorageClassName: p.StorageClassName,
		},
	}
}

// BuildHonchoService returns a ClusterIP Service.
func BuildHonchoService(inst *hermesv1.HermesInstance) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoServiceName(inst),
			Namespace: inst.Namespace,
			Labels:    HonchoLabels(inst),
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
			Selector: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8000,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// BuildHonchoDeployment returns the Honcho Deployment.
func BuildHonchoDeployment(inst *hermesv1.HermesInstance) *appsv1.Deployment {
	h := inst.Spec.ProfileStore.Honcho
	image := honchoImageRef(h)
	labels := HonchoLabels(inst)

	env := []corev1.EnvVar{
		{Name: "HONCHO_DATA_DIR", Value: "/data"},
	}
	if h.APIKeySecretRef != nil {
		env = append(env, corev1.EnvVar{
			Name: "HONCHO_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: h.APIKeySecretRef.LocalObjectReference,
					Key:                  h.APIKeySecretRef.Key,
				},
			},
		})
	}

	mounts := []corev1.VolumeMount{
		{Name: "honcho-data", MountPath: "/data"},
		{Name: "tmp", MountPath: "/tmp"},
	}
	volumes := []corev1.Volume{
		{Name: "honcho-data", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: HonchoPVCName(inst)},
		}},
		{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HonchoDeploymentName(inst),
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:                Ptr(int32(1)),
			RevisionHistoryLimit:    Ptr(int32(10)),
			ProgressDeadlineSeconds: Ptr(int32(600)),
			Strategy:                appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     "honcho",
				"app.kubernetes.io/instance": HonchoDeploymentName(inst),
			}},
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
						Name:                     "honcho",
						Image:                    image,
						ImagePullPolicy:          honchoPullPolicy(h),
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Env:                      env,
						Resources:                h.Resources,
						Ports: []corev1.ContainerPort{{
							Name: "http", ContainerPort: 8000, Protocol: corev1.ProtocolTCP,
						}},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: Ptr(false),
							ReadOnlyRootFilesystem:   Ptr(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
						VolumeMounts: mounts,
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("http"),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
							TimeoutSeconds:      2,
							FailureThreshold:    3,
							SuccessThreshold:    1,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("http")},
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       30,
							TimeoutSeconds:      5,
							FailureThreshold:    3,
							SuccessThreshold:    1,
						},
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

// BuildHonchoConsumerEnv returns env vars added to the hermes container.
func BuildHonchoConsumerEnv(inst *hermesv1.HermesInstance) []corev1.EnvVar {
	if !honchoEnabled(inst) {
		return nil
	}
	out := []corev1.EnvVar{
		{Name: "HONCHO_BASE_URL", Value: fmt.Sprintf("http://%s:8000", HonchoServiceName(inst))},
	}
	if ref := inst.Spec.ProfileStore.Honcho.APIKeySecretRef; ref != nil {
		out = append(out, corev1.EnvVar{
			Name: "HONCHO_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: ref.LocalObjectReference,
					Key:                  ref.Key,
				},
			},
		})
	}
	return out
}

func honchoImageRef(h hermesv1.HonchoSpec) string {
	repo := h.Image.Repository
	if repo == "" {
		repo = "ghcr.io/plastic-labs/honcho"
	}
	tag := h.Image.Tag
	if tag == "" {
		tag = "0.1.0"
	}
	return fmt.Sprintf("%s:%s", repo, tag)
}

func honchoPullPolicy(h hermesv1.HonchoSpec) corev1.PullPolicy {
	if h.Image.PullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return corev1.PullPolicy(h.Image.PullPolicy)
}
