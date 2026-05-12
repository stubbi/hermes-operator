package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesInstanceValidator enforces design §7.3 rules.
type HermesInstanceValidator struct {
	Client client.Client
}

var _ admission.CustomValidator = &HermesInstanceValidator{}

// Ptr is the package-local generic pointer helper.
func Ptr[T any](v T) *T { return &v }

// intOrStr is a test/internal helper.
func intOrStr(s string) intstr.IntOrString { return intstr.FromString(s) }

// ValidateCreate runs the full sanity ruleset on a fresh resource.
func (v *HermesInstanceValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	inst, ok := obj.(*hermesv1.HermesInstance)
	if !ok {
		return nil, fmt.Errorf("expected *HermesInstance, got %T", obj)
	}
	warns, err := validateCommon(inst)
	if err != nil {
		return warns, err
	}
	gwWarns, gwErr := v.validateGateways(ctx, inst)
	warns = append(warns, gwWarns...)
	if gwErr != nil {
		return warns, gwErr
	}
	return warns, nil
}

// ValidateUpdate runs the create rules + immutability rules.
func (v *HermesInstanceValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldI, ok1 := oldObj.(*hermesv1.HermesInstance)
	newI, ok2 := newObj.(*hermesv1.HermesInstance)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("ValidateUpdate types: old=%T new=%T", oldObj, newObj)
	}
	if err := validateImmutable(oldI, newI); err != nil {
		return nil, err
	}
	warns, err := validateCommon(newI)
	if err != nil {
		return warns, err
	}
	gwWarns, gwErr := v.validateGateways(ctx, newI)
	warns = append(warns, gwWarns...)
	if gwErr != nil {
		return warns, gwErr
	}
	return warns, nil
}

// ValidateDelete is a no-op.
func (v *HermesInstanceValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *HermesInstanceValidator) validateGateways(ctx context.Context, inst *hermesv1.HermesInstance) (admission.Warnings, error) {
	var warnings admission.Warnings
	g := inst.Spec.Gateways

	check := func(field string, enabled *bool, ref *corev1.SecretKeySelector, required bool) error {
		if enabled == nil || !*enabled {
			return nil
		}
		if ref == nil {
			if required {
				return fmt.Errorf("%s is required when the gateway is enabled", field)
			}
			return nil
		}
		if v.Client == nil {
			// No client available — skip the existence check, fail-open.
			return nil
		}
		var s corev1.Secret
		err := v.Client.Get(ctx, types.NamespacedName{Namespace: inst.Namespace, Name: ref.Name}, &s)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf(
					"%s references Secret %q which is not present yet in namespace %q; the instance will block on rollout until the secret is created",
					field, ref.Name, inst.Namespace,
				))
				return nil
			}
			return fmt.Errorf("look up %s: %w", field, err)
		}
		if ref.Key != "" {
			if _, ok := s.Data[ref.Key]; !ok {
				warnings = append(warnings, fmt.Sprintf(
					"%s references key %q in Secret %q which is not present in the Secret's data",
					field, ref.Key, ref.Name,
				))
			}
		}
		return nil
	}

	if err := check("spec.gateways.telegram.botTokenSecretRef", g.Telegram.Enabled, g.Telegram.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.discord.botTokenSecretRef", g.Discord.Enabled, g.Discord.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.slack.botTokenSecretRef", g.Slack.Enabled, g.Slack.BotTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.slack.appTokenSecretRef", g.Slack.Enabled, g.Slack.AppTokenSecretRef, false); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.slack.signingSecretRef", g.Slack.Enabled, g.Slack.SigningSecretRef, false); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.whatsapp.providerSecretRef", g.WhatsApp.Enabled, g.WhatsApp.ProviderSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.signal.phoneNumberSecretRef", g.Signal.Enabled, g.Signal.PhoneNumberSecretRef, true); err != nil {
		return warnings, err
	}
	if err := check("spec.gateways.signal.authTokenSecretRef", g.Signal.Enabled, g.Signal.AuthTokenSecretRef, true); err != nil {
		return warnings, err
	}
	if inst.Spec.ProfileStore.Honcho.Enabled != nil && *inst.Spec.ProfileStore.Honcho.Enabled {
		if err := check("spec.profileStore.honcho.apiKeySecretRef", inst.Spec.ProfileStore.Honcho.Enabled, inst.Spec.ProfileStore.Honcho.APIKeySecretRef, true); err != nil {
			return warnings, err
		}
	}

	return warnings, nil
}

func validateCommon(inst *hermesv1.HermesInstance) (admission.Warnings, error) {
	var warns admission.Warnings

	if inst.Spec.Image.Repository == "" {
		return warns, fmt.Errorf("spec.image.repository is required (set on the instance or via HermesClusterDefaults)")
	}
	if inst.Spec.Storage.Persistence.Size == "" {
		return warns, fmt.Errorf("spec.storage.persistence.size is required")
	}

	if inst.Spec.Config.Raw != nil && inst.Spec.Config.ConfigMapRef != nil && inst.Spec.Config.MergeMode == "" {
		warns = append(warns, "spec.config.raw and spec.config.configMapRef are both set without spec.config.mergeMode; defaults to 'replace' (Raw wins)")
	}

	if inst.Spec.SelfConfigure.Enabled != nil && *inst.Spec.SelfConfigure.Enabled {
		if len(inst.Spec.SelfConfigure.ProtectedKeys) == 0 {
			return warns, fmt.Errorf("spec.selfConfigure.enabled=true requires non-empty spec.selfConfigure.protectedKeys (explicit allowlist policy)")
		}
		if len(inst.Spec.SelfConfigure.AllowedActions) == 0 {
			return warns, fmt.Errorf("spec.selfConfigure.enabled=true requires non-empty spec.selfConfigure.allowedActions")
		}
		allowed := map[string]struct{}{
			"skills":         {},
			"config":         {},
			"envVars":        {},
			"workspaceFiles": {},
			"profiles":       {},
		}
		for _, a := range inst.Spec.SelfConfigure.AllowedActions {
			if _, ok := allowed[a]; !ok {
				return warns, fmt.Errorf("spec.selfConfigure.allowedActions contains unknown action %q (allowed: skills,config,envVars,workspaceFiles,profiles)", a)
			}
		}
	}

	pdb := inst.Spec.Availability.PodDisruptionBudget
	if pdb.MinAvailable != nil && pdb.MaxUnavailable != nil {
		return warns, fmt.Errorf("spec.availability.podDisruptionBudget: MinAvailable and MaxUnavailable are mutually exclusive")
	}

	hpa := inst.Spec.Availability.HorizontalPodAutoscaler
	if hpa.MinReplicas != nil && hpa.MaxReplicas != nil && *hpa.MinReplicas > *hpa.MaxReplicas {
		return warns, fmt.Errorf("spec.availability.horizontalPodAutoscaler: MinReplicas > MaxReplicas")
	}

	return warns, nil
}

func validateImmutable(oldI, newI *hermesv1.HermesInstance) error {
	if oldI.Spec.Storage.Persistence.StorageClassName != nil &&
		(newI.Spec.Storage.Persistence.StorageClassName == nil ||
			*oldI.Spec.Storage.Persistence.StorageClassName != *newI.Spec.Storage.Persistence.StorageClassName) {
		return fmt.Errorf("spec.storage.persistence.storageClassName is immutable")
	}
	if oldI.Name != newI.Name {
		return fmt.Errorf("metadata.name is immutable")
	}
	return nil
}

var _ = webhook.Admission{}
