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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"
)

// HermesInstanceReconciler reconciles a HermesInstance.
type HermesInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

const operatorLabelPrefix = "hermes.agent/"

func (r *HermesInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var inst hermesv1.HermesInstance
	if err := r.Get(ctx, req.NamespacedName, &inst); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcilePVC(ctx, &inst); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile PVC: %w", err)
	}
	if err := r.reconcileConfigMap(ctx, &inst); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile ConfigMap: %w", err)
	}
	if err := r.reconcileService(ctx, &inst); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Service: %w", err)
	}
	if err := r.reconcileStatefulSet(ctx, &inst); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile StatefulSet: %w", err)
	}
	if err := r.updateStatus(ctx, &inst); err != nil {
		logger.Error(err, "status update failed")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *HermesInstanceReconciler) reconcilePVC(ctx context.Context, inst *hermesv1.HermesInstance) error {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.PVCName(inst),
		Namespace: inst.Namespace,
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
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.ConfigMapName(inst),
		Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildConfigMap(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Data = desired.Data
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) reconcileService(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.ServiceName(inst),
		Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildService(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
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

func (r *HermesInstanceReconciler) reconcileStatefulSet(ctx context.Context, inst *hermesv1.HermesInstance) error {
	obj := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.StatefulSetName(inst),
		Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		desired := resources.BuildStatefulSet(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, operatorLabelPrefix)
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, r.Scheme)
	})
	return err
}

func (r *HermesInstanceReconciler) updateStatus(ctx context.Context, inst *hermesv1.HermesInstance) error {
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		return err
	}
	ready := sts.Status.ReadyReplicas > 0 && sts.Status.ReadyReplicas == sts.Status.Replicas
	phase := "Pending"
	if ready {
		phase = "Ready"
	}
	if inst.Status.Phase != phase || inst.Status.ObservedGeneration != inst.Generation {
		inst.Status.Phase = phase
		inst.Status.ObservedGeneration = inst.Generation
		return r.Status().Update(ctx, inst)
	}
	return nil
}

// SetupWithManager wires watches.
func (r *HermesInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesInstance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Named("hermesinstance").
		Complete(r)
}
