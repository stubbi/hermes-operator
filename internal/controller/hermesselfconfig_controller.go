/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesSelfConfigReconciler reconciles HermesSelfConfig resources by
// applying the requested mutations to the parent HermesInstance and/or the
// workspace ConfigMap via Server-Side Apply.
//
// SSA mechanics in this controller:
//  1. Never call client.Update on the parent HermesInstance — that reconciler
//     owns the lifecycle of those objects; we only patch fields.
//  2. Every write is `client.Patch(ctx, partial, client.Apply, ...)`
//     with FieldOwner=SelfConfigFieldManager. SSA records ownership per
//     field; other managers (Flux, Argo, kubectl users) keep theirs.
//  3. The partial object contains ONLY the fields we want to own. A
//     zero/empty field is not claimed.
//  4. ForceOwnership is opt-in per HermesSelfConfig via the
//     "hermes.agent/force-ownership: true" annotation. Default is
//     collaborative — conflicts become Denied status entries.
type HermesSelfConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesselfconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesinstances,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the controller-runtime entrypoint.
func (r *HermesSelfConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("hermesselfconfig", req.NamespacedName)

	var sc hermesv1.HermesSelfConfig
	if err := r.Get(ctx, req.NamespacedName, &sc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Idempotency canary: if we've already processed this generation and the
	// request is in a terminal phase, requeue for cleanup only.
	if sc.Status.ObservedGeneration == sc.Generation &&
		(sc.Status.Phase == hermesv1.SelfConfigPhaseApplied ||
			sc.Status.Phase == hermesv1.SelfConfigPhaseDenied) {
		return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
	}

	parent := &hermesv1.HermesInstance{}
	parentKey := types.NamespacedName{Name: sc.Spec.InstanceRef, Namespace: sc.Namespace}
	if err := r.Get(ctx, parentKey, parent); err != nil {
		if apierrors.IsNotFound(err) {
			return r.deny(ctx, &sc, nil, fmt.Sprintf("parent HermesInstance %q not found", sc.Spec.InstanceRef))
		}
		return ctrl.Result{}, err
	}

	// Gate 1: selfConfigure must be enabled on the parent.
	if !boolValue(parent.Spec.SelfConfigure.Enabled) {
		return r.deny(ctx, &sc, parent, "selfconfig disabled on parent")
	}

	// Gate 2: every requested action must be in allowedActions.
	requested := DetermineActions(&sc)
	if len(requested) == 0 {
		return r.deny(ctx, &sc, parent, "request contains no mutations")
	}
	if denied := CheckAllowedActions(requested, parent.Spec.SelfConfigure.AllowedActions); len(denied) > 0 {
		msg := fmt.Sprintf("actions %v not in allowedActions=%v", denied, parent.Spec.SelfConfigure.AllowedActions)
		return r.deny(ctx, &sc, parent, msg)
	}

	// Gate 3: protected paths.
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		hit, err := CheckProtectedPaths(sc.Spec.PatchConfig.Raw, parent.Spec.SelfConfigure.ProtectedKeys)
		if err != nil {
			return r.deny(ctx, &sc, parent, fmt.Sprintf("invalid patchConfig: %v", err))
		}
		if hit != "" {
			return r.deny(ctx, &sc, parent, fmt.Sprintf("patchConfig path %q is protected", hit))
		}
	}

	applied, err := r.applyAll(ctx, parent, &sc)
	if err != nil {
		logger.Error(err, "SSA apply failed")
		return r.deny(ctx, &sc, parent, fmt.Sprintf("apply failed: %v", err))
	}

	return r.markApplied(ctx, &sc, parent, applied)
}

// applyAll runs every action category via SSA and returns the list of
// dotted-path field identifiers we touched.
func (r *HermesSelfConfigReconciler) applyAll(ctx context.Context, parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) ([]string, error) {
	var applied []string

	if len(sc.Spec.AddSkills) > 0 {
		patch := buildSkillsPatch(parent, sc)
		if err := r.applySSA(ctx, patch, sc); err != nil {
			return applied, fmt.Errorf("skills SSA: %w", err)
		}
		for _, s := range sc.Spec.AddSkills {
			applied = append(applied, formatAppliedFieldSkill(s.Source))
		}
	}

	if len(sc.Spec.AddEnvVars) > 0 {
		patch := buildEnvVarsPatch(parent, sc)
		if err := r.applySSA(ctx, patch, sc); err != nil {
			return applied, fmt.Errorf("envVars SSA: %w", err)
		}
		for _, ev := range sc.Spec.AddEnvVars {
			applied = append(applied, formatAppliedFieldEnv(ev.Name))
		}
	}

	var cmPatch *corev1.ConfigMap
	if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
		cmPatch = buildPatchConfigPayload(parent, sc)
	}
	if len(sc.Spec.AddWorkspaceFiles) > 0 {
		cmPatch = mergeConfigMapPatches(cmPatch, buildWorkspaceFilesPatch(parent, sc))
	}
	if cmPatch != nil {
		if err := r.applySSA(ctx, cmPatch, sc); err != nil {
			return applied, fmt.Errorf("workspace CM SSA: %w", err)
		}
		if sc.Spec.PatchConfig != nil && len(sc.Spec.PatchConfig.Raw) > 0 {
			applied = append(applied, "workspace-configmap.data[key=selfconfig.yaml]")
		}
		for _, f := range sc.Spec.AddWorkspaceFiles {
			applied = append(applied, formatAppliedFieldFile(f.Path))
		}
	}

	if sc.Spec.AddProfileSnapshot != nil {
		if !boolValue(parent.Spec.ProfileStore.Honcho.Enabled) {
			return applied, fmt.Errorf("addProfileSnapshot requires .spec.profileStore.honcho.enabled=true")
		}
		job := buildProfileSnapshotPayload(parent, sc, time.Now())
		if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) { // reconcile-guard:allow
			return applied, fmt.Errorf("snapshot Job create: %w", err)
		}
		applied = append(applied, fmt.Sprintf("job[name=%s]", job.Name))
	}

	return applied, nil
}

func (r *HermesSelfConfigReconciler) patchOptions(sc *hermesv1.HermesSelfConfig) []client.PatchOption {
	opts := []client.PatchOption{client.FieldOwner(SelfConfigFieldManager)}
	if sc.Annotations[ForceOwnershipAnnotation] == "true" {
		opts = append(opts, client.ForceOwnership)
	}
	return opts
}

func (r *HermesSelfConfigReconciler) applySSA(ctx context.Context, obj client.Object, sc *hermesv1.HermesSelfConfig) error {
	return r.Patch(ctx, obj, client.Apply, r.patchOptions(sc)...)
}

func (r *HermesSelfConfigReconciler) deny(ctx context.Context, sc *hermesv1.HermesSelfConfig, parent *hermesv1.HermesInstance, reason string) (ctrl.Result, error) {
	sc.Status.Phase = hermesv1.SelfConfigPhaseDenied
	sc.Status.DenyReason = reason
	sc.Status.ObservedGeneration = sc.Generation
	now := metav1.Now()
	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               string(hermesv1.SelfConfigConditionDenied),
		Status:             metav1.ConditionTrue,
		Reason:             "PolicyViolation",
		Message:            reason,
		LastTransitionTime: now,
	})
	emitSelfConfigEvent(r.Recorder, sc, parent, corev1.EventTypeWarning, EventReasonSelfConfigDenied, reason)
	incSelfConfigDenied(parent, reason)
	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HermesSelfConfigReconciler) markApplied(ctx context.Context, sc *hermesv1.HermesSelfConfig, parent *hermesv1.HermesInstance, applied []string) (ctrl.Result, error) {
	sc.Status.Phase = hermesv1.SelfConfigPhaseApplied
	sc.Status.DenyReason = ""
	sc.Status.AppliedFields = applied
	now := metav1.Now()
	sc.Status.AppliedAt = &now
	sc.Status.ObservedGeneration = sc.Generation
	meta.SetStatusCondition(&sc.Status.Conditions, metav1.Condition{
		Type:               string(hermesv1.SelfConfigConditionApplied),
		Status:             metav1.ConditionTrue,
		Reason:             "SSASuccess",
		Message:            fmt.Sprintf("applied %d fields", len(applied)),
		LastTransitionTime: now,
	})
	emitSelfConfigEvent(r.Recorder, sc, parent, corev1.EventTypeNormal, EventReasonSelfConfigApplied, fmt.Sprintf("applied %d fields", len(applied)))
	for _, a := range DetermineActions(sc) {
		incSelfConfigApplied(parent, string(a))
	}
	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil
}

func (r *HermesSelfConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesSelfConfig{}).
		Named("hermesselfconfig").
		Complete(r)
}

func boolValue(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
