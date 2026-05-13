package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-hermes-agent-v1-hermesinstance,mutating=true,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesinstances,verbs=create;update,versions=v1,name=mhermesinstance.hermes.agent,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-hermes-agent-v1-hermesinstance,mutating=false,failurePolicy=fail,sideEffects=None,groups=hermes.agent,resources=hermesinstances,verbs=create;update,versions=v1,name=vhermesinstance.hermes.agent,admissionReviewVersions=v1

var hermesinstancelog = logf.Log.WithName("hermesinstance-webhook")

// RegisterHermesInstanceWebhook wires both the defaulter and the validator with the manager.
func RegisterHermesInstanceWebhook(mgr ctrl.Manager, def admission.CustomDefaulter, val admission.CustomValidator) error {
	hermesinstancelog.Info("registering HermesInstance webhook")
	return ctrl.NewWebhookManagedBy(mgr).
		For(&HermesInstance{}).
		WithDefaulter(def).
		WithValidator(val).
		Complete()
}

var _ = webhook.Admission{}
