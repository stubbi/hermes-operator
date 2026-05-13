package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesselfconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesselfconfigs,verbs=create;update,versions=v1,name=vhermesselfconfig.hermes.agent,admissionReviewVersions=v1

// RegisterHermesSelfConfigWebhook wires the validator with the manager.
func RegisterHermesSelfConfigWebhook(mgr ctrl.Manager, val admission.CustomValidator) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesSelfConfig{}).
		WithValidator(val).
		Complete()
}
