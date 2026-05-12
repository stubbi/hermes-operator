/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// SelfConfigFieldManager is the SSA field manager string the operator uses
// when applying HermesSelfConfig-driven mutations to HermesInstance and to
// the workspace ConfigMap. Any other manager that writes the same path
// produces an SSA conflict, which is exactly what we want — GitOps tools
// keep their fields, this manager keeps its own.
const SelfConfigFieldManager = "hermes.agent/selfconfig"

// ForceOwnershipAnnotation, when set to "true" on a HermesSelfConfig,
// causes the reconciler to call client.Apply with client.ForceOwnership.
// Default behaviour (no annotation, or "false") is collaborative — SSA
// conflicts are surfaced as a Denied status and reported via an Event.
const ForceOwnershipAnnotation = "hermes.agent/force-ownership"

// newPartialInstance returns a HermesInstance carrying only the apiVersion +
// kind + identity fields. Callers populate exactly the spec fields they
// intend to own. SSA semantics: an empty/zero field is NOT claimed.
func newPartialInstance(parent *hermesv1.HermesInstance) *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: hermesv1.GroupVersion.String(),
			Kind:       "HermesInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      parent.Name,
			Namespace: parent.Namespace,
		},
	}
}

// buildSkillsPatch returns a partial HermesInstance whose .spec.skills holds
// only the entries from sc.Spec.AddSkills. SSA merges these into the existing
// slice by listMapKey=source.
func buildSkillsPatch(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *hermesv1.HermesInstance {
	p := newPartialInstance(parent)
	for _, s := range sc.Spec.AddSkills {
		p.Spec.Skills = append(p.Spec.Skills, hermesv1.InstanceSkill{
			Source:  s.Source,
			Version: s.Version,
		})
	}
	return p
}

// buildEnvVarsPatch returns a partial HermesInstance whose .spec.env holds
// only the entries from sc.Spec.AddEnvVars. SSA merges by listMapKey=name.
func buildEnvVarsPatch(parent *hermesv1.HermesInstance, sc *hermesv1.HermesSelfConfig) *hermesv1.HermesInstance {
	p := newPartialInstance(parent)
	for _, ev := range sc.Spec.AddEnvVars {
		out := corev1.EnvVar{Name: ev.Name, Value: ev.Value}
		if ev.ValueFrom != nil {
			out.Value = ""
			vf := &corev1.EnvVarSource{}
			if ev.ValueFrom.SecretKeyRef != nil {
				vf.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.SecretKeyRef.Name},
					Key:                  ev.ValueFrom.SecretKeyRef.Key,
				}
			}
			if ev.ValueFrom.ConfigMapKeyRef != nil {
				vf.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.ConfigMapKeyRef.Name},
					Key:                  ev.ValueFrom.ConfigMapKeyRef.Key,
				}
			}
			out.ValueFrom = vf
		}
		p.Spec.Env = append(p.Spec.Env, out)
	}
	return p
}

func formatAppliedFieldEnv(name string) string { return fmt.Sprintf("spec.env[name=%s]", name) }
func formatAppliedFieldSkill(source string) string {
	return fmt.Sprintf("spec.skills[source=%s]", source)
}
func formatAppliedFieldFile(path string) string {
	return fmt.Sprintf("workspace-configmap.data[path=%s]", path)
}
