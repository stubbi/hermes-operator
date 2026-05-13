package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestBuildServiceAccount_NameAndAnnotations(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Security: hermesv1.SecuritySpec{
				RBAC: hermesv1.RBACSpec{
					CreateServiceAccount: Ptr(true),
					Annotations: map[string]string{
						"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/hermes",
					},
				},
			},
		},
	}
	sa := BuildServiceAccount(inst)
	assert.Equal(t, "demo", sa.Name)
	assert.Equal(t, "agents", sa.Namespace)
	assert.Equal(t, "arn:aws:iam::1:role/hermes", sa.Annotations["eks.amazonaws.com/role-arn"])
	assert.NotNil(t, sa.AutomountServiceAccountToken)
	assert.False(t, *sa.AutomountServiceAccountToken)
}

func TestBuildServiceAccount_AutomountTokenWhenSelfConfigureEnabled(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true)},
		},
	}
	sa := BuildServiceAccount(inst)
	assert.NotNil(t, sa.AutomountServiceAccountToken)
	assert.True(t, *sa.AutomountServiceAccountToken)
}

func TestBuildRole_BaseRulesAndSelfConfigure(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	r := BuildRole(inst)
	assert.Equal(t, "demo", r.Name)
	require := false
	for _, rule := range r.Rules {
		for _, res := range rule.Resources {
			if res == "configmaps" {
				require = true
			}
		}
	}
	assert.True(t, require, "base Role must grant configmap reads")

	inst2 := &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo"},
		Spec: hermesv1.HermesInstanceSpec{
			SelfConfigure: hermesv1.SelfConfigureSpec{Enabled: Ptr(true)},
		},
	}
	r2 := BuildRole(inst2)
	var sawHermesSelfConfig bool
	for _, rule := range r2.Rules {
		for _, g := range rule.APIGroups {
			if g == "hermes.agent" {
				for _, res := range rule.Resources {
					if res == "hermesselfconfigs" {
						sawHermesSelfConfig = true
					}
				}
			}
		}
	}
	assert.True(t, sawHermesSelfConfig, "selfConfigure=true must add hermesselfconfigs verbs")
}

func TestBuildRoleBinding_Matches(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	rb := BuildRoleBinding(inst)
	assert.Equal(t, "demo", rb.Name)
	assert.Equal(t, "agents", rb.Namespace)
	assert.Equal(t, "demo", rb.RoleRef.Name)
	assert.Equal(t, "Role", rb.RoleRef.Kind)
	assert.Len(t, rb.Subjects, 1)
	assert.Equal(t, "ServiceAccount", rb.Subjects[0].Kind)
	assert.Equal(t, "demo", rb.Subjects[0].Name)
	assert.Equal(t, "agents", rb.Subjects[0].Namespace)
}

func TestRBACNames(t *testing.T) {
	t.Parallel()
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	assert.Equal(t, "demo", ServiceAccountName(inst))
	assert.Equal(t, "demo", RoleName(inst))
	assert.Equal(t, "demo", RoleBindingName(inst))
}
