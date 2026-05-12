package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesclusterdefaults,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesclusterdefaults,verbs=create;update,versions=v1,name=vhermesclusterdefaults.hermes.agent,admissionReviewVersions=v1

// RegisterHermesClusterDefaultsWebhook wires the validator with the manager.
func RegisterHermesClusterDefaultsWebhook(mgr ctrl.Manager, val admission.CustomValidator) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesClusterDefaults{}).
		WithValidator(val).
		Complete()
}
