/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package resources

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildSnapshotJob_NameAndMounts(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
	}
	stamp := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	job := BuildSnapshotJob(inst, "user-42", "snapshot-payload", stamp)
	assert.Equal(t, "demo-snapshot-user-42-20260512080000", job.Name)
	assert.Equal(t, "agents", job.Namespace)

	spec := job.Spec.Template.Spec
	assert.Equal(t, corev1.RestartPolicyNever, spec.RestartPolicy, "Jobs use RestartPolicyNever")
	assert.Len(t, spec.Containers, 1)

	mounts := spec.Containers[0].VolumeMounts
	assert.Len(t, mounts, 1)
	assert.Equal(t, "honcho-data", mounts[0].Name)
	assert.Equal(t, "/data", mounts[0].MountPath)

	vols := spec.Volumes
	assert.Len(t, vols, 1)
	assert.NotNil(t, vols[0].PersistentVolumeClaim)
	assert.Equal(t, "demo-honcho-data", vols[0].PersistentVolumeClaim.ClaimName)
}

func TestBuildSnapshotJob_HardenedSecurity(t *testing.T) {
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}}
	job := BuildSnapshotJob(inst, "p", "data", time.Now())
	c := job.Spec.Template.Spec.Containers[0]
	assert.NotNil(t, c.SecurityContext)
	assert.True(t, *c.SecurityContext.ReadOnlyRootFilesystem)
	assert.False(t, *c.SecurityContext.AllowPrivilegeEscalation)
	assert.Equal(t, []corev1.Capability{"ALL"}, c.SecurityContext.Capabilities.Drop)
}
