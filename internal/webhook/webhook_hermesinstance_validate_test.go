package webhook

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
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

func TestValidator_RuntimeSetWarnsDeprecated(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Runtime: hermesv1.RuntimeSpec{Python: "3.11"}, // any non-zero value
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	found := false
	for _, w := range warns {
		if strings.Contains(w, "spec.runtime is deprecated") {
			found = true
		}
	}
	assert.True(t, found, "expected a spec.runtime deprecation warning, got: %v", warns)
}

func TestValidator_NoRuntimeNoDeprecationWarning(t *testing.T) {
	t.Parallel()
	v := &HermesInstanceValidator{}
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	for _, w := range warns {
		assert.NotContains(t, w, "spec.runtime is deprecated",
			"unset spec.runtime must not warn")
	}
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
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true), AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionSkills}},
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
				AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionProfiles},
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
				AllowedActions: []hermesv1.SelfConfigAction{"reboot-cluster"},
				ProtectedKeys:  []string{"provider.apiKey"},
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reboot-cluster")
}

func tailscaleEnabledInstance(secretName string) *hermesv1.HermesInstance {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image:   hermesv1.ImageSpec{Repository: "x"},
			Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
			Tailscale: hermesv1.TailscaleSpec{
				Enabled: Ptr(true),
			},
		},
	}
	if secretName != "" {
		inst.Spec.Tailscale.AuthKey = &hermesv1.TailscaleAuthKey{
			SecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "authkey",
			},
		}
	}
	return inst
}

func TestValidateTailscale_RequiresAuthKey(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)

	// authKey nil entirely.
	inst := tailscaleEnabledInstance("")
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.tailscale.authKey")

	// authKey set but secretRef nil.
	inst = tailscaleEnabledInstance("")
	inst.Spec.Tailscale.AuthKey = &hermesv1.TailscaleAuthKey{}
	_, err = v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.tailscale.authKey")

	// Same rule on update.
	old := tailscaleEnabledInstance("ts-auth")
	_, err = v.ValidateUpdate(context.Background(), old, tailscaleEnabledInstance(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.tailscale.authKey")
}

func TestValidateTailscale_MissingSecretWarns(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t)
	inst := tailscaleEnabledInstance("missing")
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.NotEmpty(t, warnings)
}

func TestValidateTailscale_MissingKeyWarns(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ts-auth", Namespace: "agents"},
		Data:       map[string][]byte{"other": []byte("x")},
	})
	inst := tailscaleEnabledInstance("ts-auth")
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	assert.NotEmpty(t, warnings)
}

func TestValidateTailscale_ReservedNames(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ts-auth", Namespace: "agents"},
		Data:       map[string][]byte{"authkey": []byte("x")},
	})

	// User sidecar named "tailscale" collides with the operator-managed container.
	inst := tailscaleEnabledInstance("ts-auth")
	inst.Spec.Sidecars = []corev1.Container{{Name: "tailscale", Image: "busybox"}}
	_, err := v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tailscale")
	assert.Contains(t, err.Error(), "spec.sidecars")

	// User extra volume named "tailscale-serve" collides.
	inst = tailscaleEnabledInstance("ts-auth")
	inst.Spec.ExtraVolumes = []corev1.Volume{{Name: "tailscale-serve"}}
	_, err = v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tailscale-serve")
	assert.Contains(t, err.Error(), "spec.extraVolumes")

	// User extra volume named "tailscale-tmp" collides.
	inst = tailscaleEnabledInstance("ts-auth")
	inst.Spec.ExtraVolumes = []corev1.Volume{{Name: "tailscale-tmp"}}
	_, err = v.ValidateCreate(context.Background(), inst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tailscale-tmp")
}

func TestValidateTailscale_HappyPathAndDisabledSkipped(t *testing.T) {
	t.Parallel()
	v := newValidatorWithObjs(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ts-auth", Namespace: "agents"},
		Data:       map[string][]byte{"authkey": []byte("x")},
	})

	// Enabled with the secret + key present: no error, no tailscale warnings.
	inst := tailscaleEnabledInstance("ts-auth")
	warnings, err := v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	for _, w := range warnings {
		assert.NotContains(t, w, "tailscale")
	}

	// Disabled: validation is skipped entirely, even with reserved names in use.
	inst = tailscaleEnabledInstance("")
	inst.Spec.Tailscale.Enabled = Ptr(false)
	inst.Spec.Sidecars = []corev1.Container{{Name: "tailscale", Image: "busybox"}}
	inst.Spec.ExtraVolumes = []corev1.Volume{{Name: "tailscale-serve"}}
	warnings, err = v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
	for _, w := range warnings {
		assert.NotContains(t, w, "tailscale")
	}

	// Enabled flag nil counts as disabled too.
	inst.Spec.Tailscale.Enabled = nil
	_, err = v.ValidateCreate(context.Background(), inst)
	assert.NoError(t, err)
}

func TestValidateRestoreFromImmutableAfterLatch(t *testing.T) {
	old := &hermesv1.HermesInstance{
		Spec:   hermesv1.HermesInstanceSpec{RestoreFrom: "k1"},
		Status: hermesv1.HermesInstanceStatus{RestoredFrom: "k1"},
	}
	newer := old.DeepCopy()
	newer.Spec.RestoreFrom = "k2"
	errs := validateImmutableTerminals(old, newer)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "spec.restoreFrom")
}

func TestValidateMigrationImmutableAfterCompleted(t *testing.T) {
	old := &hermesv1.HermesInstance{
		Spec: hermesv1.HermesInstanceSpec{
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Mode: "copy",
					Source: hermesv1.MigrationFromOpenClawSource{
						OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{Name: "x", Namespace: "y"},
					},
				},
			},
		},
		Status: hermesv1.HermesInstanceStatus{Migration: hermesv1.MigrationStatus{Completed: true}},
	}
	newer := old.DeepCopy()
	newer.Spec.Migration.FromOpenClaw.Mode = "move"
	errs := validateImmutableTerminals(old, newer)
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "migration")
}

func TestValidateMutualExclusion(t *testing.T) {
	inst := &hermesv1.HermesInstance{
		Spec: hermesv1.HermesInstanceSpec{
			RestoreFrom: "k1",
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Source: hermesv1.MigrationFromOpenClawSource{
						OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{Name: "x", Namespace: "y"},
					},
				},
			},
		},
	}
	errs := validateRestoreMigrationMutualExclusion(inst)
	assert.NotEmpty(t, errs)
}

func TestValidateMigrationSourceExactlyOne(t *testing.T) {
	both := &hermesv1.HermesInstance{
		Spec: hermesv1.HermesInstanceSpec{
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Source: hermesv1.MigrationFromOpenClawSource{
						OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{Name: "x", Namespace: "y"},
						BackupRef:           &hermesv1.MigrationBackupRef{S3: hermesv1.MigrationBackupS3{Bucket: "b", Key: "k", Endpoint: "e", CredentialsSecretRef: hermesv1.LocalObjectReference{Name: "s"}}},
					},
				},
			},
		},
	}
	assert.NotEmpty(t, validateMigrationSourceExactlyOne(both))

	neither := &hermesv1.HermesInstance{
		Spec: hermesv1.HermesInstanceSpec{
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Source: hermesv1.MigrationFromOpenClawSource{},
				},
			},
		},
	}
	assert.NotEmpty(t, validateMigrationSourceExactlyOne(neither))

	one := &hermesv1.HermesInstance{
		Spec: hermesv1.HermesInstanceSpec{
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Source: hermesv1.MigrationFromOpenClawSource{
						OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{Name: "x", Namespace: "y"},
					},
				},
			},
		},
	}
	assert.Empty(t, validateMigrationSourceExactlyOne(one))
}
