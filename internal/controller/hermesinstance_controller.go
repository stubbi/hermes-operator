/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// HermesInstanceReconciler reconciles a HermesInstance.
type HermesInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// PrometheusOperatorCRDsPresent caches whether ServiceMonitor/PrometheusRule
	// CRDs are installed. Probed once at startup by cmd/manager.
	PrometheusOperatorCRDsPresent bool

	Backup     *BackupReconciler
	Restore    *RestoreReconciler
	AutoUpdate *AutoUpdateReconciler
	Migration  *MigrationReconciler
}

const operatorLabelPrefix = "hermes.agent/"

const (
	reasonProfileStoreDisabled       = "Disabled"
	reasonProfileStoreDeploymentDown = "DeploymentNotReady"
	reasonProfileStoreReady          = "Ready"
)

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims;secrets;serviceaccounts;events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies;ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;prometheusrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=openclaw.rocks,resources=openclawinstances,verbs=get;list;watch

func (r *HermesInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	inst := &hermesv1.HermesInstance{}
	if err := r.Get(ctx, req.NamespacedName, inst); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Deletion path — hand to backup finalizer if requested.
	if !inst.DeletionTimestamp.IsZero() {
		if r.Backup != nil {
			res, held, err := r.Backup.HandleDeletion(ctx, inst)
			if err != nil {
				return ctrl.Result{}, err
			}
			if held {
				return res, nil
			}
		}
		return ctrl.Result{}, nil
	}

	// Add the backup-on-delete finalizer when spec.backup.onDelete=true (uses r.Patch — lesson #437).
	if r.Backup != nil {
		if err := r.Backup.EnsureFinalizer(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
	}

	steps := []struct {
		name string
		cond string
		fn   func(context.Context, *hermesv1.HermesInstance) error
	}{
		{"Secret", hermesv1.ConditionTypeSecretsReady, r.reconcileSecret},
		{"PVC", hermesv1.ConditionTypeStorageReady, r.reconcilePVC},
		{"ConfigMap", hermesv1.ConditionTypeConfigReady, r.reconcileConfigMap},
		{"WorkspaceConfigMap", hermesv1.ConditionTypeConfigReady, r.reconcileWorkspaceConfigMap},
		{"NetworkPolicy", hermesv1.ConditionTypeNetworkPolicyReady, r.reconcileNetworkPolicy},
		{"RBAC", hermesv1.ConditionTypeRBACReady, r.reconcileRBAC},
		{"Service", hermesv1.ConditionTypeServiceReady, r.reconcileService},
		{"PDB", hermesv1.ConditionTypePDBReady, r.reconcilePDB},
		{"HPA", hermesv1.ConditionTypeHPAReady, r.reconcileHPA},
		{"Ingress", hermesv1.ConditionTypeIngressReady, r.reconcileIngress},
		{"ServiceMonitor", hermesv1.ConditionTypeServiceMonitorReady, r.reconcileServiceMonitor},
		{"PrometheusRule", hermesv1.ConditionTypePrometheusRuleReady, r.reconcilePrometheusRule},
		{"StatefulSet", "StatefulSetReady", r.reconcileStatefulSet},
		{"Honcho", "ProfileStoreReady", r.reconcileHoncho},
	}
	for _, s := range steps {
		if err := s.fn(ctx, inst); err != nil {
			r.setCondition(inst, s.cond, metav1.ConditionFalse, "Error", err.Error())
			_ = r.Status().Update(ctx, inst)
			logger.Error(err, "subsystem failed", "subsystem", s.name)
			return ctrl.Result{}, fmt.Errorf("reconcile %s: %w", s.name, err)
		}
		r.setCondition(inst, s.cond, metav1.ConditionTrue, "Reconciled", s.name+" up to date")
	}

	// Sub-controller chain — backup CronJob, migration latch, restore latch, autoupdate poll.
	if r.Backup != nil {
		if err := r.Backup.ReconcileCronJob(ctx, inst); err != nil {
			logger.Error(err, "backup CronJob reconcile error")
		}
	}
	if r.Migration != nil {
		if _, _, err := r.Migration.Reconcile(ctx, inst); err != nil {
			logger.Error(err, "migration reconcile error")
		}
	}
	if r.Restore != nil {
		if _, _, err := r.Restore.Reconcile(ctx, inst); err != nil {
			logger.Error(err, "restore reconcile error")
		}
	}
	if r.AutoUpdate != nil {
		if _, err := r.AutoUpdate.Reconcile(ctx, inst); err != nil {
			logger.Error(err, "autoupdate reconcile error")
		}
	}

	r.updateProfileStoreCondition(ctx, inst)

	if err := r.updateStatus(ctx, inst); err != nil {
		logger.Error(err, "status update failed")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// --- per-subsystem reconcilers ---

func (r *HermesInstanceReconciler) reconcileSecret(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: resources.GatewayTokenSecretName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildGatewayTokenSecret(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		obj.Type = desired.Type
		if obj.Data == nil {
			obj.Data = desired.Data
		}
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePVC(ctx context.Context, inst *hermesv1.HermesInstance) error {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: resources.PVCName(inst), Namespace: inst.Namespace,
	}}
	err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
	if apierrors.IsNotFound(err) {
		desired := resources.BuildPVC(inst)
		if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, desired)
	}
	return err
}

func (r *HermesInstanceReconciler) reconcileConfigMap(ctx context.Context, inst *hermesv1.HermesInstance) error {
	body, err := r.resolveConfigBody(ctx, inst)
	if err != nil {
		return err
	}
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ConfigMapName(inst), Namespace: inst.Namespace,
	}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildConfigMap(inst, body)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Data = desired.Data
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) resolveConfigBody(ctx context.Context, inst *hermesv1.HermesInstance) (string, error) {
	cs := inst.Spec.Config
	if cs.ConfigMapRef == nil {
		return "", nil
	}
	user := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: cs.ConfigMapRef.Name, Namespace: inst.Namespace}, user); err != nil {
		return "", fmt.Errorf("resolve configMapRef %q: %w", cs.ConfigMapRef.Name, err)
	}
	base := user.Data["config.yaml"]
	if cs.Raw == nil {
		return base, nil
	}
	if cs.MergeMode == hermesv1.ConfigMergeModeMerge {
		return resources.MergeYAMLBodies(base, string(cs.Raw.Raw))
	}
	return "", nil
}

func (r *HermesInstanceReconciler) reconcileWorkspaceConfigMap(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: resources.WorkspaceConfigMapName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildWorkspaceConfigMap(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Data = desired.Data
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileNetworkPolicy(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValueOrDefault(inst.Spec.Security.NetworkPolicy.Enabled, true)
	obj := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
		Name: resources.NetworkPolicyName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildNetworkPolicy(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileRBAC(ctx context.Context, inst *hermesv1.HermesInstance) error {
	create := resources.BoolValueOrDefault(inst.Spec.Security.RBAC.CreateServiceAccount, true)
	if !create {
		return nil
	}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ServiceAccountName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error {
		desired := resources.BuildServiceAccount(inst)
		sa.Labels = resources.MergePreservingForeign(sa.Labels, desired.Labels, operatorLabelPrefix)
		sa.Annotations = resources.MergePreservingForeign(sa.Annotations, desired.Annotations, operatorLabelPrefix)
		sa.AutomountServiceAccountToken = desired.AutomountServiceAccountToken
		return controllerutil.SetControllerReference(inst, sa, r.Scheme)
	}); err != nil {
		return fmt.Errorf("sa: %w", err)
	}
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
		Name: resources.RoleName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		desired := resources.BuildRole(inst)
		role.Labels = resources.MergePreservingForeign(role.Labels, desired.Labels, operatorLabelPrefix)
		role.Rules = desired.Rules
		return controllerutil.SetControllerReference(inst, role, r.Scheme)
	}); err != nil {
		return fmt.Errorf("role: %w", err)
	}
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: resources.RoleBindingName(inst), Namespace: inst.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		desired := resources.BuildRoleBinding(inst)
		rb.Labels = resources.MergePreservingForeign(rb.Labels, desired.Labels, operatorLabelPrefix)
		rb.Subjects = desired.Subjects
		rb.RoleRef = desired.RoleRef
		return controllerutil.SetControllerReference(inst, rb, r.Scheme)
	}); err != nil {
		return fmt.Errorf("rolebinding: %w", err)
	}
	return nil
}

func (r *HermesInstanceReconciler) reconcileService(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: resources.ServiceName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildService(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		clusterIP := obj.Spec.ClusterIP
		clusterIPs := obj.Spec.ClusterIPs
		obj.Spec = desired.Spec
		if clusterIP != "" {
			obj.Spec.ClusterIP = clusterIP
			obj.Spec.ClusterIPs = clusterIPs
		}
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePDB(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Availability.PodDisruptionBudget.Enabled)
	obj := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{
		Name: resources.PDBName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildPDB(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileHPA(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{
		Name: resources.HPAName(inst), Namespace: inst.Namespace,
	}}
	if !resources.IsHPAEnabled(inst) {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildHPA(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileIngress(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.Networking.Ingress.Enabled)
	obj := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{
		Name: resources.IngressName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		return r.deleteIfExists(ctx, obj)
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildIngress(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Annotations = resources.MergePreservingForeign(obj.Annotations, desired.Annotations, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileServiceMonitor(ctx context.Context, inst *hermesv1.HermesInstance) error {
	// If the Prometheus Operator CRDs are not present there is nothing to manage —
	// no ServiceMonitor can exist, so skip entirely (don't even try to delete).
	if !r.PrometheusOperatorCRDsPresent {
		return nil
	}
	enabled := resources.BoolValue(inst.Spec.Observability.ServiceMonitor.Enabled)
	if !enabled {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(resources.ServiceMonitorGVK())
		obj.SetName(resources.ServiceMonitorName(inst))
		obj.SetNamespace(inst.Namespace)
		return r.deleteIfExists(ctx, obj)
	}
	desired := resources.BuildServiceMonitor(inst)
	if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
		return err
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resources.ServiceMonitorGVK())
	obj.SetName(desired.GetName())
	obj.SetNamespace(desired.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Object["spec"] = desired.Object["spec"]
		obj.SetLabels(resources.MergePreservingForeign(obj.GetLabels(), desired.GetLabels(), operatorLabelPrefix))
		obj.SetOwnerReferences(desired.GetOwnerReferences())
		return nil
	})
	return err
}

func (r *HermesInstanceReconciler) reconcilePrometheusRule(ctx context.Context, inst *hermesv1.HermesInstance) error {
	// If the Prometheus Operator CRDs are not present there is nothing to manage —
	// no PrometheusRule can exist, so skip entirely (don't even try to delete).
	if !r.PrometheusOperatorCRDsPresent {
		return nil
	}
	enabled := resources.BoolValue(inst.Spec.Observability.PrometheusRule.Enabled)
	if !enabled {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(resources.PrometheusRuleGVK())
		obj.SetName(resources.PrometheusRuleName(inst))
		obj.SetNamespace(inst.Namespace)
		return r.deleteIfExists(ctx, obj)
	}
	desired := resources.BuildPrometheusRule(inst)
	if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
		return err
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resources.PrometheusRuleGVK())
	obj.SetName(desired.GetName())
	obj.SetNamespace(desired.GetNamespace())
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Object["spec"] = desired.Object["spec"]
		obj.SetLabels(resources.MergePreservingForeign(obj.GetLabels(), desired.GetLabels(), operatorLabelPrefix))
		obj.SetOwnerReferences(desired.GetOwnerReferences())
		return nil
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileStatefulSet(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: resources.StatefulSetName(inst), Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		extraInits := []corev1.Container{}
		if c := resources.BuildRestoreInitContainer(inst); c != nil {
			extraInits = append(extraInits, *c)
		}
		if c := resources.BuildMigrationInitContainer(inst); c != nil {
			extraInits = append(extraInits, *c)
		}
		desired := resources.BuildStatefulSet(inst, extraInits)
		if r.Migration != nil {
			if vol := r.Migration.BuildSourceVolume(inst); vol != nil {
				desired.Spec.Template.Spec.Volumes = append(desired.Spec.Template.Spec.Volumes, *vol)
			}
		}
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileHoncho(ctx context.Context, inst *hermesv1.HermesInstance) error {
	enabled := resources.BoolValue(inst.Spec.ProfileStore.Honcho.Enabled)
	persistEnabled := resources.BoolValueOrDefault(inst.Spec.ProfileStore.Honcho.Persistence.Enabled, true)

	// PVC: create-only when enabled && persistence enabled; leave on disable (data safety).
	if enabled && persistEnabled {
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name: resources.HonchoPVCName(inst), Namespace: inst.Namespace,
		}}
		err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, pvc)
		if apierrors.IsNotFound(err) {
			desired := resources.BuildHonchoPVC(inst)
			desired.Labels = resources.MergePreservingForeign(desired.Labels, desired.Labels, operatorLabelPrefix)
			if err := controllerutil.SetControllerReference(inst, desired, r.Scheme); err != nil {
				return fmt.Errorf("honcho pvc owner ref: %w", err)
			}
			if err := r.Create(ctx, desired); err != nil { // reconcile-guard:allow — PVC is create-only (leave on disable for data safety)
				return fmt.Errorf("honcho pvc create: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("honcho pvc get: %w", err)
		}
	}

	// Service: create/update when enabled; delete when disabled.
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: resources.HonchoServiceName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		if err := r.deleteIfExists(ctx, svc); err != nil {
			return fmt.Errorf("honcho svc delete: %w", err)
		}
	} else {
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
			desired := resources.BuildHonchoService(inst)
			svc.Labels = resources.MergePreservingForeign(svc.Labels, desired.Labels, operatorLabelPrefix)
			svc.Annotations = resources.MergePreservingForeign(svc.Annotations, desired.Annotations, operatorLabelPrefix)
			clusterIP := svc.Spec.ClusterIP
			clusterIPs := svc.Spec.ClusterIPs
			svc.Spec = desired.Spec
			if clusterIP != "" {
				svc.Spec.ClusterIP = clusterIP
				svc.Spec.ClusterIPs = clusterIPs
			}
			return controllerutil.SetControllerReference(inst, svc, r.Scheme)
		}); err != nil {
			return fmt.Errorf("honcho svc: %w", err)
		}
	}

	// Deployment: create/update when enabled; delete when disabled.
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: resources.HonchoDeploymentName(inst), Namespace: inst.Namespace,
	}}
	if !enabled {
		if err := r.deleteIfExists(ctx, dep); err != nil {
			return fmt.Errorf("honcho deployment delete: %w", err)
		}
	} else {
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
			desired := resources.BuildHonchoDeployment(inst)
			dep.Labels = resources.MergePreservingForeign(dep.Labels, desired.Labels, operatorLabelPrefix)
			dep.Spec = desired.Spec
			return controllerutil.SetControllerReference(inst, dep, r.Scheme)
		}); err != nil {
			return fmt.Errorf("honcho deployment: %w", err)
		}
	}

	// NetworkPolicy: create/update when enabled AND global NetworkPolicy is enabled; otherwise delete.
	npEnabled := enabled && resources.BoolValueOrDefault(inst.Spec.Security.NetworkPolicy.Enabled, true)
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
		Name: resources.HonchoDeploymentName(inst), Namespace: inst.Namespace,
	}}
	if !npEnabled {
		if err := r.deleteIfExists(ctx, np); err != nil {
			return fmt.Errorf("honcho netpol delete: %w", err)
		}
	} else {
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
			desired := resources.BuildHonchoNetworkPolicy(inst)
			np.Labels = resources.MergePreservingForeign(np.Labels, desired.Labels, operatorLabelPrefix)
			np.Spec = desired.Spec
			return controllerutil.SetControllerReference(inst, np, r.Scheme)
		}); err != nil {
			return fmt.Errorf("honcho netpol: %w", err)
		}
	}

	return nil
}

func (r *HermesInstanceReconciler) updateProfileStoreCondition(ctx context.Context, inst *hermesv1.HermesInstance) {
	cond := metav1.Condition{
		Type:               "ProfileStoreReady",
		ObservedGeneration: inst.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}

	if !resources.BoolValue(inst.Spec.ProfileStore.Honcho.Enabled) {
		cond.Status = metav1.ConditionTrue
		cond.Reason = reasonProfileStoreDisabled
		cond.Message = "Honcho profile store disabled (spec.profileStore.honcho.enabled=false)"
		meta.SetStatusCondition(&inst.Status.Conditions, cond)
		return
	}

	var dep appsv1.Deployment
	key := types.NamespacedName{Namespace: inst.Namespace, Name: resources.HonchoDeploymentName(inst)}
	if err := r.Get(ctx, key, &dep); err != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = reasonProfileStoreDeploymentDown
		cond.Message = fmt.Sprintf("Honcho Deployment not found: %v", err)
	} else if dep.Status.ReadyReplicas >= 1 {
		cond.Status = metav1.ConditionTrue
		cond.Reason = reasonProfileStoreReady
		cond.Message = "Honcho Deployment has >=1 ready replica"
	} else {
		cond.Status = metav1.ConditionFalse
		cond.Reason = reasonProfileStoreDeploymentDown
		cond.Message = fmt.Sprintf("Honcho Deployment has %d/%d ready replicas", dep.Status.ReadyReplicas, dep.Status.Replicas)
	}

	meta.SetStatusCondition(&inst.Status.Conditions, cond)
}

// --- helpers ---

func (r *HermesInstanceReconciler) deleteIfExists(ctx context.Context, obj client.Object) error {
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return client.IgnoreNotFound(r.Delete(ctx, obj))
}

func (r *HermesInstanceReconciler) setCondition(inst *hermesv1.HermesInstance, t string, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               t,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: inst.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	})
}

func (r *HermesInstanceReconciler) updateStatus(ctx context.Context, inst *hermesv1.HermesInstance) error {
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		return err
	}
	inst.Status.Replicas = sts.Status.Replicas
	inst.Status.ReadyReplicas = sts.Status.ReadyReplicas
	inst.Status.ObservedGeneration = inst.Generation
	switch {
	case inst.Spec.Suspended:
		inst.Status.Phase = "Suspended"
	case sts.Status.ReadyReplicas > 0 && sts.Status.ReadyReplicas == sts.Status.Replicas:
		inst.Status.Phase = "Ready"
		r.setCondition(inst, hermesv1.ConditionTypeReady, metav1.ConditionTrue, "AllSubsystemsReady", "")
	default:
		inst.Status.Phase = "Pending"
		r.setCondition(inst, hermesv1.ConditionTypeReady, metav1.ConditionFalse, "StatefulSetNotReady", "")
	}
	return r.Status().Update(ctx, inst)
}

// SetupWithManager wires watches for every owned type.
func (r *HermesInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesInstance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Named("hermesinstance").
		Complete(r)
}
