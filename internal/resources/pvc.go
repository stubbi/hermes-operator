package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// PVCName returns the deterministic PVC name for a HermesInstance.
func PVCName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-data"
}

// BuildPVC returns the desired PersistentVolumeClaim. PVCs are immutable
// after creation (k8s rule); callers must only create, never update.
func BuildPVC(inst *hermesv1.HermesInstance) *corev1.PersistentVolumeClaim {
	size := inst.Spec.Storage.Persistence.Size
	if size == "" {
		size = "1Gi"
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: inst.Spec.Storage.Persistence.StorageClassName,
		},
	}
}
