/*
Copyright 2026 stubbi. Apache-2.0.
*/

package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesSelfConfigValidator validates HermesSelfConfig creates and updates.
// It checks the existence of the parent HermesInstance, the well-formedness
// of patchConfig (must be valid JSON), and a few cross-field invariants
// (e.g. addProfileSnapshot requires honcho enabled on the parent).
//
// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesselfconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesselfconfigs,verbs=create;update,versions=v1,name=vhermesselfconfig.kb.io,admissionReviewVersions=v1
type HermesSelfConfigValidator struct {
	Client client.Client
}

var _ admission.CustomValidator = (*HermesSelfConfigValidator)(nil)

func (v *HermesSelfConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}

func (v *HermesSelfConfigValidator) ValidateUpdate(ctx context.Context, _, obj runtime.Object) (admission.Warnings, error) {
	return v.validate(ctx, obj)
}

func (v *HermesSelfConfigValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *HermesSelfConfigValidator) validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	sc, ok := obj.(*hermesv1.HermesSelfConfig)
	if !ok {
		return nil, fmt.Errorf("expected HermesSelfConfig, got %T", obj)
	}

	if sc.Spec.InstanceRef == "" {
		return nil, fmt.Errorf("spec.instanceRef is required")
	}

	if v.Client != nil {
		parent := &hermesv1.HermesInstance{}
		err := v.Client.Get(ctx, types.NamespacedName{Name: sc.Spec.InstanceRef, Namespace: sc.Namespace}, parent)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("spec.instanceRef %q: no HermesInstance with that name in namespace %q", sc.Spec.InstanceRef, sc.Namespace)
			}
			return nil, fmt.Errorf("loading parent instance: %w", err)
		}
		if sc.Spec.AddProfileSnapshot != nil {
			if parent.Spec.ProfileStore.Honcho.Enabled == nil || !*parent.Spec.ProfileStore.Honcho.Enabled {
				return nil, fmt.Errorf("spec.addProfileSnapshot requires parent .spec.profileStore.honcho.enabled=true")
			}
		}
	}

	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		var tmp map[string]interface{}
		if err := json.Unmarshal(sc.Spec.PatchConfig.Raw, &tmp); err != nil {
			return nil, fmt.Errorf("spec.patchConfig is not a valid JSON merge patch: %w", err)
		}
	}

	mutations := 0
	for _, has := range []bool{
		len(sc.Spec.AddSkills) > 0,
		sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0,
		len(sc.Spec.AddEnvVars) > 0,
		len(sc.Spec.AddWorkspaceFiles) > 0,
		sc.Spec.AddProfileSnapshot != nil,
	} {
		if has {
			mutations++
		}
	}
	if mutations > 1 {
		return admission.Warnings{
			"this HermesSelfConfig requests multiple mutations; consider one mutation per resource for atomic audit trails",
		}, nil
	}
	return nil, nil
}
