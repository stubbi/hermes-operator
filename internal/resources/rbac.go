package resources

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// ServiceAccountName returns the deterministic name of the operator-created
// ServiceAccount. Distinct from ServiceAccountNameFor (which honors the
// spec.security.rbac.serviceAccountName override and tells the pod which SA to
// use).
func ServiceAccountName(inst *hermesv1.HermesInstance) string { return inst.Name }

// RoleName returns the deterministic Role name.
func RoleName(inst *hermesv1.HermesInstance) string { return inst.Name }

// RoleBindingName returns the deterministic RoleBinding name.
func RoleBindingName(inst *hermesv1.HermesInstance) string { return inst.Name }

// BuildServiceAccount returns the per-instance SA. AutomountServiceAccountToken
// is false unless spec.selfConfigure.enabled is true.
func BuildServiceAccount(inst *hermesv1.HermesInstance) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceAccountName(inst),
			Namespace:   inst.Namespace,
			Labels:      LabelsForInstance(inst),
			Annotations: inst.Spec.Security.RBAC.Annotations,
		},
		AutomountServiceAccountToken: Ptr(BoolValue(inst.Spec.SelfConfigure.Enabled)),
	}
}

// BuildRole returns the per-instance Role. Base ruleset: read own ConfigMap +
// own gateway-token Secret. When selfConfigure.enabled is true, additional
// verbs are added on hermesinstances and hermesselfconfigs.
func BuildRole(inst *hermesv1.HermesInstance) *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{ConfigMapName(inst), WorkspaceConfigMapName(inst)},
			Verbs:         []string{"get", "watch"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{GatewayTokenSecretName(inst)},
			Verbs:         []string{"get", "watch"},
		},
	}
	if BoolValue(inst.Spec.SelfConfigure.Enabled) {
		rules = append(rules,
			rbacv1.PolicyRule{
				APIGroups:     []string{"hermes.agent"},
				Resources:     []string{"hermesinstances"},
				ResourceNames: []string{inst.Name},
				Verbs:         []string{"get"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"hermes.agent"},
				Resources: []string{"hermesselfconfigs"},
				Verbs:     []string{"create", "get", "list"},
			},
		)
	}
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Rules: rules,
	}
}

// BuildRoleBinding binds the per-instance SA to the per-instance Role.
func BuildRoleBinding(inst *hermesv1.HermesInstance) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleBindingName(inst),
			Namespace: inst.Namespace,
			Labels:    LabelsForInstance(inst),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName(inst),
				Namespace: inst.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     RoleName(inst),
		},
	}
}
