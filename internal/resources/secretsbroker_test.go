package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
	"github.com/paperclipinc/hermes-operator/internal/secretsbroker"
)

func brokerSpec() *hermesv1.SecretsBrokerSpec {
	return &hermesv1.SecretsBrokerSpec{
		URL:            "https://broker.example/secrets/resolve/${EMPLOYEE_ID}",
		TimeoutSeconds: Ptr(int32(5)),
		MaxAttempts:    Ptr(int32(4)),
		EnvPrefix:      "BROKER_",
		Headers: []hermesv1.SecretsBrokerHeader{
			{Name: "Authorization", ValueFrom: &hermesv1.SecretsBrokerHeaderSource{SecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "broker-auth"}, Key: "header",
			}}},
			{Name: "X-Employee-Id", Value: "${EMPLOYEE_ID}"},
		},
	}
}

func TestBuildStatefulSet_NoSecretsBrokerByDefault(t *testing.T) {
	sts := BuildStatefulSet(minimalInstance(), nil)
	c := sts.Spec.Template.Spec.Containers[0]
	assert.Nil(t, c.Command, "image entrypoint untouched")
	assert.Nil(t, envByName(&c, secretsbroker.ConfigEnv))
	for _, ic := range sts.Spec.Template.Spec.InitContainers {
		assert.NotEqual(t, SecretsBrokerInitName, ic.Name)
	}
}

func TestBuildStatefulSet_SecretsBrokerWiring(t *testing.T) {
	old := OperatorImage
	OperatorImage = "ghcr.io/example/hermes-operator:v9.9.9"
	t.Cleanup(func() { OperatorImage = old })

	inst := minimalInstance()
	inst.Spec.SecretsBroker = brokerSpec()
	inst.Spec.Env = []corev1.EnvVar{{Name: "EMPLOYEE_ID", Value: "emp-7"}}
	sts := BuildStatefulSet(inst, nil)
	pod := sts.Spec.Template.Spec
	c := pod.Containers[0]

	assert.Equal(t, []string{"/opt/hermes-secrets-broker/resolver", "secrets-broker", "exec", "--", "/init", "/opt/hermes/docker/main-wrapper.sh"}, c.Command)
	assert.Equal(t, []string{"gateway", "run"}, c.Args, "args still route through the image wrapper")

	// The credential reaches the pod only as a secretKeyRef, never inline.
	hdr := envByName(&c, "HERMES_SECRETS_BROKER_HEADER_0")
	require.NotNil(t, hdr)
	require.NotNil(t, hdr.ValueFrom)
	assert.Equal(t, "broker-auth", hdr.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "header", hdr.ValueFrom.SecretKeyRef.Key)
	assert.Empty(t, hdr.Value)

	cfgEnv := envByName(&c, secretsbroker.ConfigEnv)
	require.NotNil(t, cfgEnv)
	cfg, err := secretsbroker.ParseConfig(cfgEnv.Value)
	require.NoError(t, err, "operator output must parse in the resolver")
	assert.Equal(t, "https://broker.example/secrets/resolve/${EMPLOYEE_ID}", cfg.URL)
	assert.Equal(t, map[string]string{"Authorization": "${HERMES_SECRETS_BROKER_HEADER_0}", "X-Employee-Id": "${EMPLOYEE_ID}"}, cfg.Headers)
	assert.Equal(t, 5, cfg.TimeoutSeconds)
	assert.Equal(t, 4, cfg.MaxAttempts)
	assert.Equal(t, "BROKER_", cfg.EnvPrefix)
	assert.Equal(t, secretsbroker.ConfigEnv, c.Env[len(c.Env)-1].Name, "operator config is appended after user env so it wins")

	var mount *corev1.VolumeMount
	for i := range c.VolumeMounts {
		if c.VolumeMounts[i].Name == SecretsBrokerVolume {
			mount = &c.VolumeMounts[i]
		}
	}
	require.NotNil(t, mount)
	assert.True(t, mount.ReadOnly)

	var vol *corev1.Volume
	for i := range pod.Volumes {
		if pod.Volumes[i].Name == SecretsBrokerVolume {
			vol = &pod.Volumes[i]
		}
	}
	require.NotNil(t, vol)
	require.NotNil(t, vol.EmptyDir, "resolver binary lives on an emptyDir, never the PVC")

	require.NotEmpty(t, pod.InitContainers)
	ic := pod.InitContainers[0]
	assert.Equal(t, SecretsBrokerInitName, ic.Name)
	assert.Equal(t, "ghcr.io/example/hermes-operator:v9.9.9", ic.Image)
	assert.Equal(t, []string{"/manager", "secrets-broker", "install", "/opt/hermes-secrets-broker/resolver"}, ic.Command)
	require.NotNil(t, ic.SecurityContext)
	assert.True(t, *ic.SecurityContext.ReadOnlyRootFilesystem)
	assert.True(t, *ic.SecurityContext.RunAsNonRoot)
	assert.Equal(t, []corev1.Capability{"ALL"}, ic.SecurityContext.Capabilities.Drop)
}

func TestBuildStatefulSet_SecretsBrokerOverrides(t *testing.T) {
	old := OperatorImage
	OperatorImage = "ghcr.io/example/hermes-operator:v9.9.9"
	t.Cleanup(func() { OperatorImage = old })

	inst := minimalInstance()
	inst.Spec.SecretsBroker = &hermesv1.SecretsBrokerSpec{
		URL:           "https://b/x",
		ResolverImage: "registry.local/hermes-operator@sha256:abc",
		Entrypoint:    []string{"/usr/local/bin/entry"},
	}
	sts := BuildStatefulSet(inst, []corev1.Container{{Name: "init-restore"}})
	pod := sts.Spec.Template.Spec
	assert.Equal(t, []string{"/opt/hermes-secrets-broker/resolver", "secrets-broker", "exec", "--", "/usr/local/bin/entry"}, pod.Containers[0].Command)
	require.Len(t, pod.InitContainers, 2)
	assert.Equal(t, "init-restore", pod.InitContainers[0].Name, "restore still runs first")
	assert.Equal(t, "registry.local/hermes-operator@sha256:abc", pod.InitContainers[1].Image)
}

func TestBuildStatefulSet_SecretsBrokerFailsClosedWithoutImage(t *testing.T) {
	old := OperatorImage
	OperatorImage = ""
	t.Cleanup(func() { OperatorImage = old })

	inst := minimalInstance()
	inst.Spec.SecretsBroker = &hermesv1.SecretsBrokerSpec{URL: "https://b/x"}
	sts := BuildStatefulSet(inst, nil)
	ic := sts.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, SecretsBrokerInitName, ic.Name)
	assert.Equal(t, imageRef(inst), ic.Image)
	require.Len(t, ic.Command, 4)
	assert.Contains(t, ic.Command[2], "exit 1")
	assert.Contains(t, ic.Command[3], "HERMES_OPERATOR_IMAGE")
}

func TestSecretsBrokerConfigJSON_IsDeterministic(t *testing.T) {
	a, _, err := SecretsBrokerConfigJSON(brokerSpec())
	require.NoError(t, err)
	b, _, err := SecretsBrokerConfigJSON(brokerSpec())
	require.NoError(t, err)
	assert.Equal(t, a, b, "a stable rendering avoids StatefulSet rollouts on every reconcile")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(a), &m))
	assert.NotContains(t, m, "allowInsecureHTTP", "zero values are omitted")
}
