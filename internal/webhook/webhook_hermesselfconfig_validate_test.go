package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func newSelfConfigValidator(t *testing.T, objs ...client.Object) *HermesSelfConfigValidator {
	t.Helper()
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	return &HermesSelfConfigValidator{Client: c}
}

func selfConfigParent(name string, profileEnabled bool) *hermesv1.HermesInstance {
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
	}
	if profileEnabled {
		t := true
		inst.Spec.ProfileStore.Honcho.Enabled = &t
	}
	return inst
}

func TestSCValidate_RejectsMissingInstance(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t)
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec:       hermesv1.HermesSelfConfigSpec{InstanceRef: "nope"},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instanceRef")
}

func TestSCValidate_RejectsEmptyInstanceRef(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t)
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instanceRef")
}

func TestSCValidate_AcceptsValidRequest(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t, selfConfigParent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
	assert.Empty(t, warns)
}

func TestSCValidate_WarnsOnMultipleMutations(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t, selfConfigParent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "X", Value: "y"}},
		},
	}
	warns, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
	require.NotEmpty(t, warns, "must warn: not deny: on multiple mutation fields")
	assert.Contains(t, warns[0], "atomic")
}

func TestSCValidate_RejectsInvalidJSONPatch(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t, selfConfigParent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{not-json`)},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "patchConfig")
}

func TestSCValidate_RejectsSnapshotWithoutHoncho(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t, selfConfigParent("my-hermes", false))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "u", Data: "d",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "honcho")
}

func TestSCValidate_AcceptsSnapshotWithHoncho(t *testing.T) {
	t.Parallel()
	v := newSelfConfigValidator(t, selfConfigParent("my-hermes", true))
	sc := &hermesv1.HermesSelfConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "u", Data: "d",
			},
		},
	}
	_, err := v.ValidateCreate(context.Background(), sc)
	require.NoError(t, err)
}
