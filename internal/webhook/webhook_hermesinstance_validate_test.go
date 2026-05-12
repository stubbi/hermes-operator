package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestValidator_DenyEmptyImageRepository(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err, "image.repository is required")
}

func TestValidator_DenyConfigRawAndConfigMapRefWithoutMergeMode(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Config: hermesv1.ConfigSpec{
				Raw:          &hermesv1.RawConfig{RawExtension: runtime.RawExtension{Raw: []byte("{}")}},
				ConfigMapRef: &corev1.LocalObjectReference{Name: "x"},
				MergeMode:    "",
			},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.NotEmpty(t, warns)
}

func TestValidator_DenySelfConfigureEnabledNoProtectedKeys(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:         hermesv1.ImageSpec{Repository: "x"},
			Storage:       hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true), AllowedActions: []string{"skills"}},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
}

func TestValidator_DenyImmutableStorageClassName(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	old := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Size: "1Gi", StorageClassName: Ptr("gp3")},
			},
		},
	}
	newer := old.DeepCopy()
	newer.Spec.Storage.Persistence.StorageClassName = Ptr("io2")

	_, err := v.ValidateUpdate(context.Background(), old, newer)
	assert.Error(t, err)
}

func TestValidator_DenyBothPDBValuesSet(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	mi := intOrStr("50%")
	mu := intOrStr("1")
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &mi, MaxUnavailable: &mu},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
}

func TestValidator_AllowHappyPath(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.Empty(t, warns)
}

func newValidatorWithObjs(t *testing.T, objs ...client.Object) *HermesInstanceValidator {
	t.Helper()
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &HermesInstanceValidator{Client: c}
}

func TestValidateGateways_TelegramSecretMissingProducesWarning(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{
					Enabled: Ptr(true),
					BotTokenSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
						Key:                  "token",
					},
				},
			},
		},
	}
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.NotEmpty(t, warnings)
}

func TestValidateGateways_TelegramEnabledWithoutSecretRefDenied(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{Enabled: Ptr(true)},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "botTokenSecretRef")
}

func TestValidateGateways_SecretExistsNoWarning(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "tg", Namespace: "agents"},
		Data:       map[string][]byte{"token": []byte("x")},
	})
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Gateways: hermesv1.GatewaysSpec{
				Telegram: hermesv1.TelegramGatewaySpec{
					Enabled: Ptr(true),
					BotTokenSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "tg"},
						Key:                  "token",
					},
				},
			},
		},
	}
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	for _, w := range warnings {
		assert.NotContains(t, w, "gateways.telegram")
	}
}

func TestValidateSelfConfigure_ProfilesActionAllowed(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			SelfConfigure: hermesv1.SelfConfigureSpec{
				Enabled:        Ptr(true),
				AllowedActions: []string{"profiles"},
				ProtectedKeys:  []string{"provider.apiKey"},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
}

func TestValidateSelfConfigure_UnknownActionDenied(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			SelfConfigure: hermesv1.SelfConfigureSpec{
				Enabled:        Ptr(true),
				AllowedActions: []string{"reboot-cluster"},
				ProtectedKeys:  []string{"provider.apiKey"},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reboot-cluster")
}
