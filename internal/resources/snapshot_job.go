/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package resources

import (
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BuildSnapshotJob constructs a one-shot Job that writes a profile snapshot
// to /data/snapshots/<profileID>/<timestamp>.json on the Honcho PVC.
// Name is deterministic: `<inst>-snapshot-<profileID>-<YYYYMMDDHHMMSS>`.
func BuildSnapshotJob(inst *hermesv1.HermesInstance, profileID, data string, when time.Time) *batchv1.Job {
	stamp := when.UTC().Format("20060102150405")
	name := fmt.Sprintf("%s-snapshot-%s-%s", inst.Name, sanitizeProfileID(profileID), stamp)
	labels := LabelsForInstance(inst)
	labels["hermes.agent/component"] = "snapshot"
	labels["hermes.agent/profile-id"] = sanitizeProfileID(profileID)

	rfc3339 := when.UTC().Format(time.RFC3339)
	relPath := fmt.Sprintf("/data/snapshots/%s/%s.json", profileID, rfc3339)
	escaped := strings.ReplaceAll(data, "'", `'\''`)
	cmd := fmt.Sprintf(`set -eu; mkdir -p "$(dirname '%s')"; printf '%%s' '%s' > '%s'`, relPath, escaped, relPath)

	one := int32(1)
	ttlSeconds := int32(3600)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &one,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
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
						Name:                     "writer",
						Image:                    "busybox:1.36",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						Command:                  []string{"/bin/sh", "-c", cmd},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: Ptr(false),
							ReadOnlyRootFilesystem:   Ptr(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "honcho-data", MountPath: "/data"},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "honcho-data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: HonchoPVCName(inst),
							},
						},
					}},
				},
			},
		},
	}
}

// sanitizeProfileID lowercases and replaces non-[a-z0-9-] with "-".
func sanitizeProfileID(id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
