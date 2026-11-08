package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

func TestSelfConfigAuthorization_OptimisticParentConflictRequeuesButOwnershipConflictIsDenied(t *testing.T) {
	for _, ownership := range []bool{false, true} {
		t.Run(fmt.Sprintf("fieldOwnership=%t", ownership), func(t *testing.T) {
			ctx := context.Background()
			scheme, parent, sc := authorizationFixture(t)
			parent.Spec.SelfConfigure.AllowedActions = []hermesv1.SelfConfigAction{hermesv1.ActionEnvVars}
			sc.Spec.PatchConfig = nil
			sc.Spec.AddEnvVars = []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}}
			sc.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(parent, hermesv1.GroupVersion.WithKind("HermesInstance"))}
			c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sc).WithObjects(parent, sc).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, underlying client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if _, ok := obj.(*hermesv1.HermesInstance); ok {
							err := apierrors.NewConflict(schema.GroupResource{Group: "hermes.agent", Resource: "hermesinstances"}, parent.Name, fmt.Errorf("object changed"))
							if ownership {
								err.ErrStatus.Details.Causes = []metav1.StatusCause{{Type: metav1.CauseTypeFieldManagerConflict, Field: "spec.env"}}
							}
							return err
						}
						return underlying.Patch(ctx, obj, patch, opts...)
					},
				}).Build()
			r := &HermesSelfConfigReconciler{Client: c, Scheme: scheme}
			key := types.NamespacedName{Name: sc.Name, Namespace: sc.Namespace}
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			got := &hermesv1.HermesSelfConfig{}
			require.NoError(t, c.Get(ctx, key, got))
			if ownership {
				require.NoError(t, err)
				assert.Equal(t, hermesv1.SelfConfigPhaseDenied, got.Status.Phase)
			} else {
				assert.True(t, apierrors.IsConflict(err), "requeue to read the current parent policy")
				assert.NotEqual(t, hermesv1.SelfConfigPhaseDenied, got.Status.Phase, "a status-only parent update must not permanently deny the request")
			}
		})
	}
}

func TestSelfConfigAuthorization_SequentialParentWritesUseTheLastSuccessfulRevision(t *testing.T) {
	ctx := context.Background()
	scheme, parent, sc := authorizationFixture(t)
	parent.Spec.SelfConfigure.AllowedActions = []hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionEnvVars}
	sc.Spec.PatchConfig = nil
	sc.Spec.AddSkills = []hermesv1.SelfConfigSkill{{Source: "git+fixture"}}
	sc.Spec.AddEnvVars = []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}}
	sc.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(parent, hermesv1.GroupVersion.WithKind("HermesInstance"))}
	// The fake client has no SSA engine. This models only revision preconditions
	// and responses, and does not qualify real SSA field-ownership persistence.
	var revisions []string
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sc).WithObjects(parent, sc).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, underlying client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				p, ok := obj.(*hermesv1.HermesInstance)
				if !ok {
					return underlying.Patch(ctx, obj, patch, opts...)
				}
				current := &hermesv1.HermesInstance{}
				if err := underlying.Get(ctx, client.ObjectKeyFromObject(p), current); err != nil {
					return err
				}
				if p.ResourceVersion != current.ResourceVersion {
					return apierrors.NewConflict(schema.GroupResource{Group: "hermes.agent", Resource: "hermesinstances"}, p.Name, fmt.Errorf("stale parent revision"))
				}
				revisions = append(revisions, p.ResourceVersion)
				if len(p.Spec.Skills) > 0 {
					current.Spec.Skills = p.Spec.Skills
				}
				if len(p.Spec.Env) > 0 {
					current.Spec.Env = p.Spec.Env
				}
				if err := underlying.Update(ctx, current); err != nil {
					return err
				}
				p.ResourceVersion = current.ResourceVersion
				return nil
			},
		}).Build()
	r := &HermesSelfConfigReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(sc)})
	require.NoError(t, err)
	require.Len(t, revisions, 2)
	assert.NotEqual(t, revisions[0], revisions[1], "each write must use the previous successful response revision")
}

func authorizationFixture(t *testing.T) (*runtime.Scheme, *hermesv1.HermesInstance, *hermesv1.HermesSelfConfig) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, hermesv1.AddToScheme(scheme))
	enabled := true
	parent := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "agents", UID: "original-parent"},
		Spec: hermesv1.HermesInstanceSpec{SelfConfigure: hermesv1.SelfConfigureSpec{
			Enabled: &enabled, AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionConfig},
		}},
	}
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "patch", Namespace: parent.Namespace, Generation: 1},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: parent.Name,
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{"memory":{"provider":"hindsight"}}`)},
		},
	}
	return scheme, parent, sc
}

func TestSelfConfigAuthorization_ConcurrentSpecChangeCannotBypassPolicyDuringBinding(t *testing.T) {
	ctx := context.Background()
	scheme, parent, sc := authorizationFixture(t)
	parent.Spec.SelfConfigure.ProtectedKeys = []string{"model.**"}
	key := types.NamespacedName{Name: sc.Name, Namespace: sc.Namespace}
	injected := false
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sc).WithObjects(parent, sc).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, underlying client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if _, ok := obj.(*hermesv1.HermesSelfConfig); ok && !injected {
					injected = true
					updated := &hermesv1.HermesSelfConfig{}
					require.NoError(t, underlying.Get(ctx, key, updated))
					updated.Spec.PatchConfig = &apiextensionsv1.JSON{Raw: []byte(`{"model":{"provider":"unauthorized"}}`)}
					updated.Generation++
					require.NoError(t, underlying.Update(ctx, updated))
				}
				return underlying.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := &HermesSelfConfigReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.True(t, injected)
	require.True(t, apierrors.IsConflict(err), "a new spec must be requeued for revalidation: %v", err)
	got := &hermesv1.HermesSelfConfig{}
	require.NoError(t, c.Get(ctx, key, got))
	assert.NotEqual(t, hermesv1.SelfConfigPhaseApplied, got.Status.Phase)
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, key, got))
	assert.Equal(t, hermesv1.SelfConfigPhaseDenied, got.Status.Phase)
	assert.Contains(t, got.Status.DenyReason, "protected")
}

func TestSelfConfigAuthorization_BindsNewRequestAndKeepsAuthorizedReplayIdempotent(t *testing.T) {
	ctx := context.Background()
	scheme, parent, sc := authorizationFixture(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sc).WithObjects(parent, sc).Build()
	r := &HermesSelfConfigReconciler{Client: c, Scheme: scheme}
	key := types.NamespacedName{Name: sc.Name, Namespace: sc.Namespace}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	got := &hermesv1.HermesSelfConfig{}
	require.NoError(t, c.Get(ctx, key, got))
	require.Equal(t, hermesv1.SelfConfigPhaseApplied, got.Status.Phase)
	owner := metav1.GetControllerOf(got)
	require.NotNil(t, owner, "authorization must record the parent identity")
	assert.Equal(t, parent.UID, owner.UID)
	assert.Equal(t, parent.Name, owner.Name)
	patches, err := (&HermesInstanceReconciler{Client: c}).resolveSelfConfigPatches(ctx, parent)
	require.NoError(t, err)
	require.Len(t, patches, 1)
	first := got.DeepCopy()
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, key, got))
	assert.Equal(t, first.ResourceVersion, got.ResourceVersion, "authorized terminal replay must not rewrite status")
	assert.Equal(t, first.Status.AppliedAt, got.Status.AppliedAt)
}

func TestSelfConfigAuthorization_RevokesAppliedRequestsBeforeReplay(t *testing.T) {
	for _, scenario := range []string{"replacement enabled", "replacement disabled", "disabled", "action removed", "protected key", "legacy unbound"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			scheme, parent, sc := authorizationFixture(t)
			sc.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(parent, hermesv1.GroupVersion.WithKind("HermesInstance"))}
			sc.Status.Phase, sc.Status.ObservedGeneration = hermesv1.SelfConfigPhaseApplied, sc.Generation
			switch scenario {
			case "replacement enabled":
				parent.UID = "replacement-parent"
			case "replacement disabled":
				parent.UID = "replacement-parent"
				*parent.Spec.SelfConfigure.Enabled = false
			case "disabled":
				*parent.Spec.SelfConfigure.Enabled = false
			case "action removed":
				parent.Spec.SelfConfigure.AllowedActions = nil
			case "protected key":
				parent.Spec.SelfConfigure.ProtectedKeys = []string{"memory.**"}
			case "legacy unbound":
				sc.OwnerReferences = nil
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(sc).WithObjects(parent, sc).Build()
			patches, err := (&HermesInstanceReconciler{Client: c}).resolveSelfConfigPatches(ctx, parent)
			require.NoError(t, err)
			assert.Empty(t, patches, "instance reconcile must reject stale authorization before status is refreshed")
			r := &HermesSelfConfigReconciler{Client: c, Scheme: scheme}
			key := types.NamespacedName{Name: sc.Name, Namespace: sc.Namespace}
			_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			require.NoError(t, err)
			got := &hermesv1.HermesSelfConfig{}
			require.NoError(t, c.Get(ctx, key, got))
			assert.Equal(t, hermesv1.SelfConfigPhaseDenied, got.Status.Phase)
			assert.NotEmpty(t, got.Status.DenyReason)
			firstVersion := got.ResourceVersion
			_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			require.NoError(t, err)
			require.NoError(t, c.Get(ctx, key, got))
			assert.Equal(t, firstVersion, got.ResourceVersion, "denial must not create a reconcile loop")
		})
	}
}
