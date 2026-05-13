package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesClusterDefaultsValidator enforces design §6: name must be "cluster".
type HermesClusterDefaultsValidator struct{}

var _ admission.CustomValidator = &HermesClusterDefaultsValidator{}

func (v *HermesClusterDefaultsValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validateHCD(obj)
}

func (v *HermesClusterDefaultsValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return validateHCD(newObj)
}

func (v *HermesClusterDefaultsValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateHCD(obj runtime.Object) (admission.Warnings, error) {
	hcd, ok := obj.(*hermesv1.HermesClusterDefaults)
	if !ok {
		return nil, fmt.Errorf("expected *HermesClusterDefaults, got %T", obj)
	}
	if hcd.Name != "cluster" {
		return nil, fmt.Errorf("HermesClusterDefaults must be the singleton named \"cluster\" (got %q)", hcd.Name)
	}
	return nil, nil
}
