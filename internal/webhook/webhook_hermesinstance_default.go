package webhook

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesInstanceDefaulter fills nil fields on a HermesInstance from the
// HermesClusterDefaults singleton (name "cluster"). It never overrides
// explicit values.
type HermesInstanceDefaulter struct {
	client.Client
}

var _ admission.CustomDefaulter = &HermesInstanceDefaulter{}

// Default implements admission.CustomDefaulter.
func (d *HermesInstanceDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	inst, ok := obj.(*hermesv1.HermesInstance)
	if !ok {
		return fmt.Errorf("expected *HermesInstance, got %T", obj)
	}
	hcd := &hermesv1.HermesClusterDefaults{}
	err := d.Get(ctx, types.NamespacedName{Name: "cluster"}, hcd)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get HermesClusterDefaults: %w", err)
	}
	ApplyClusterDefaults(inst, hcd)
	return nil
}

// ApplyClusterDefaults mutates inst in place, filling nil fields from hcd.
func ApplyClusterDefaults(inst *hermesv1.HermesInstance, hcd *hermesv1.HermesClusterDefaults) {
	if inst.Spec.Image.Repository == "" {
		inst.Spec.Image.Repository = hcd.Spec.Image.Repository
	}
	if inst.Spec.Image.Tag == "" {
		inst.Spec.Image.Tag = hcd.Spec.Image.Tag
	}
	if inst.Spec.Image.PullPolicy == "" {
		inst.Spec.Image.PullPolicy = hcd.Spec.Image.PullPolicy
	}

	if inst.Spec.Storage.Persistence.Size == "" {
		inst.Spec.Storage.Persistence.Size = hcd.Spec.Storage.Persistence.Size
	}
	if inst.Spec.Storage.Persistence.StorageClassName == nil {
		inst.Spec.Storage.Persistence.StorageClassName = hcd.Spec.Storage.Persistence.StorageClassName
	}

	if inst.Spec.Resources.Requests == nil {
		inst.Spec.Resources.Requests = hcd.Spec.Resources.Requests
	}
	if inst.Spec.Resources.Limits == nil {
		inst.Spec.Resources.Limits = hcd.Spec.Resources.Limits
	}

	if inst.Spec.Security.RBAC.Annotations == nil {
		inst.Spec.Security.RBAC.Annotations = hcd.Spec.Security.ServiceAccount.Annotations
	}
	if inst.Spec.Security.NetworkPolicy.Enabled == nil {
		inst.Spec.Security.NetworkPolicy.Enabled = hcd.Spec.Security.NetworkPolicy.Enabled
	}
	if inst.Spec.Security.NetworkPolicy.AllowDNS == nil {
		inst.Spec.Security.NetworkPolicy.AllowDNS = hcd.Spec.Security.NetworkPolicy.AllowDNS
	}
	if inst.Spec.Security.CABundle.ConfigMapName == "" && inst.Spec.Security.CABundle.SecretName == "" {
		inst.Spec.Security.CABundle = hcd.Spec.Security.CABundle
	}

	if inst.Spec.Networking.Service.Type == "" {
		inst.Spec.Networking.Service.Type = hcd.Spec.Networking.Service.Type
	}

	if inst.Spec.Observability.Metrics.Enabled == nil {
		inst.Spec.Observability.Metrics.Enabled = hcd.Spec.Observability.Metrics.Enabled
	}
	if inst.Spec.Observability.Metrics.Port == 0 {
		inst.Spec.Observability.Metrics.Port = hcd.Spec.Observability.Metrics.Port
	}
	if inst.Spec.Observability.Metrics.Secure == nil {
		inst.Spec.Observability.Metrics.Secure = hcd.Spec.Observability.Metrics.Secure
	}
	if inst.Spec.Observability.ServiceMonitor.Enabled == nil {
		inst.Spec.Observability.ServiceMonitor.Enabled = hcd.Spec.Observability.ServiceMonitor.Enabled
	}
	if inst.Spec.Observability.PrometheusRule.Enabled == nil {
		inst.Spec.Observability.PrometheusRule.Enabled = hcd.Spec.Observability.PrometheusRule.Enabled
	}
	if inst.Spec.Observability.Logging.Format == "" {
		inst.Spec.Observability.Logging.Format = hcd.Spec.Observability.Logging.Format
	}
	if inst.Spec.Observability.Logging.Level == "" {
		inst.Spec.Observability.Logging.Level = hcd.Spec.Observability.Logging.Level
	}
}
