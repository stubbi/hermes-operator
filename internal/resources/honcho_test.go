package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func instWithHoncho(h hermesv1.HonchoSpec) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec:       hermesv1.HermesInstanceSpec{ProfileStore: hermesv1.ProfileStoreSpec{Honcho: h}},
	}
}

func TestHonchoNaming(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(true)})
	assert.Equal(t, "demo-honcho", HonchoServiceName(inst))
	assert.Equal(t, "demo-honcho", HonchoDeploymentName(inst))
	assert.Equal(t, "demo-honcho-data", HonchoPVCName(inst))
}

func TestBuildHonchoPVC_Defaults(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled:     Ptr(true),
		Persistence: hermesv1.HonchoPersistenceSpec{Enabled: Ptr(true)},
	})
	pvc := BuildHonchoPVC(inst)
	assert.Equal(t, "demo-honcho-data", pvc.Name)
	assert.Equal(t, resource.MustParse("5Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
	assert.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, pvc.Spec.AccessModes)
}

func TestBuildHonchoService(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(true)})
	svc := BuildHonchoService(inst)
	assert.Equal(t, "demo-honcho", svc.Name)
	assert.NotEqual(t, corev1.ClusterIPNone, svc.Spec.ClusterIP, "regular ClusterIP, not headless")
	assert.Equal(t, "demo-honcho", svc.Spec.Selector["app.kubernetes.io/instance"])
	assert.Equal(t, "honcho", svc.Spec.Selector["app.kubernetes.io/name"])
	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(8000), svc.Spec.Ports[0].Port)
}

func TestBuildHonchoDeployment(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled: Ptr(true),
		Image:   hermesv1.HonchoImageSpec{Repository: "ghcr.io/plastic-labs/honcho", Tag: "0.2.0"},
		APIKeySecretRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
			Key:                  "api-key",
		},
		Persistence: hermesv1.HonchoPersistenceSpec{Enabled: Ptr(true)},
	})
	dep := BuildHonchoDeployment(inst)
	assert.Equal(t, "demo-honcho", dep.Name)
	cs := dep.Spec.Template.Spec.Containers
	assert.Len(t, cs, 1)
	assert.Equal(t, "ghcr.io/plastic-labs/honcho:0.2.0", cs[0].Image)

	assert.Equal(t, corev1.RestartPolicyAlways, dep.Spec.Template.Spec.RestartPolicy)
	assert.Equal(t, corev1.DNSClusterFirst, dep.Spec.Template.Spec.DNSPolicy)
	assert.NotNil(t, dep.Spec.RevisionHistoryLimit)
	assert.Equal(t, int32(10), *dep.Spec.RevisionHistoryLimit)
	assert.NotNil(t, dep.Spec.ProgressDeadlineSeconds)

	var apiKeyEnv *corev1.EnvVar
	for i, e := range cs[0].Env {
		if e.Name == "HONCHO_API_KEY" {
			apiKeyEnv = &cs[0].Env[i]
		}
	}
	if assert.NotNil(t, apiKeyEnv) {
		assert.Equal(t, "honcho-secret", apiKeyEnv.ValueFrom.SecretKeyRef.Name)
		assert.Equal(t, "api-key", apiKeyEnv.ValueFrom.SecretKeyRef.Key)
	}

	var dataMount *corev1.VolumeMount
	for i, m := range cs[0].VolumeMounts {
		if m.Name == "honcho-data" {
			dataMount = &cs[0].VolumeMounts[i]
		}
	}
	if assert.NotNil(t, dataMount) {
		assert.Equal(t, "/data", dataMount.MountPath)
		assert.Equal(t, "", dataMount.SubPath)
	}
}

func TestBuildHonchoConsumerEnv(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{
		Enabled: Ptr(true),
		APIKeySecretRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"},
			Key:                  "api-key",
		},
	})
	env := BuildHonchoConsumerEnv(inst)
	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}
	assert.Equal(t, "http://demo-honcho:8000", byName["HONCHO_BASE_URL"].Value)
	assert.NotNil(t, byName["HONCHO_API_KEY"].ValueFrom)
	assert.Equal(t, "honcho-secret", byName["HONCHO_API_KEY"].ValueFrom.SecretKeyRef.Name)
}

func TestBuildHonchoConsumerEnv_Disabled(t *testing.T) {
	inst := instWithHoncho(hermesv1.HonchoSpec{Enabled: Ptr(false)})
	assert.Empty(t, BuildHonchoConsumerEnv(inst))
}
