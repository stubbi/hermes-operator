/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parentInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		TypeMeta:   metav1.TypeMeta{APIVersion: hermesv1.GroupVersion.String(), Kind: "HermesInstance"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-hermes", Namespace: "agents"},
	}
}

func TestBuildSkillsPatch_ContainsOnlySkills(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			InstanceRef: "my-hermes",
			AddSkills: []hermesv1.SelfConfigSkill{
				{Source: "git+https://github.com/foo/skill@v1"},
				{Source: "git+https://github.com/bar/other@v2", Version: "2.0"},
			},
		},
	}
	patch := buildSkillsPatch(parentInstance(), sc)

	assert.Equal(t, "my-hermes", patch.Name)
	assert.Equal(t, "agents", patch.Namespace)
	assert.Equal(t, hermesv1.GroupVersion.String(), patch.APIVersion)
	assert.Equal(t, "HermesInstance", patch.Kind)

	assert.Len(t, patch.Spec.Skills, 2)
	assert.Equal(t, "git+https://github.com/foo/skill@v1", patch.Spec.Skills[0].Source)
	assert.Equal(t, "git+https://github.com/bar/other@v2", patch.Spec.Skills[1].Source)
	assert.Equal(t, "2.0", patch.Spec.Skills[1].Version, "Version must propagate from SelfConfigSkill to InstanceSkill")
	assert.Empty(t, patch.Spec.Env, "must not touch env when only Skills is requested")
	assert.Empty(t, patch.Spec.Image.Repository, "must not touch image — Flux owns that")
}

func TestBuildEnvVarsPatch_LiteralAndValueFrom(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddEnvVars: []hermesv1.SelfConfigEnvVar{
				{Name: "FINANCE_TZ", Value: "Europe/Berlin"},
				{Name: "API_KEY", ValueFrom: &hermesv1.SelfConfigEnvVarSource{
					SecretKeyRef: &hermesv1.SelfConfigKeySelector{Name: "finance-creds", Key: "apiKey"},
				}},
			},
		},
	}
	patch := buildEnvVarsPatch(parentInstance(), sc)
	assert.Len(t, patch.Spec.Env, 2)
	assert.Equal(t, "FINANCE_TZ", patch.Spec.Env[0].Name)
	assert.Equal(t, "Europe/Berlin", patch.Spec.Env[0].Value)

	assert.Equal(t, "API_KEY", patch.Spec.Env[1].Name)
	vf := patch.Spec.Env[1].ValueFrom
	assert.NotNil(t, vf)
	assert.NotNil(t, vf.SecretKeyRef)
	assert.Equal(t, "finance-creds", vf.SecretKeyRef.LocalObjectReference.Name)
	assert.Equal(t, "apiKey", vf.SecretKeyRef.Key)
	assert.Empty(t, patch.Spec.Skills, "must not touch skills when only env requested")
}

func TestAppliedFieldsFormat(t *testing.T) {
	assert.Equal(t, "spec.env[name=FINANCE_TZ]", formatAppliedFieldEnv("FINANCE_TZ"))
	assert.Equal(t, "spec.skills[source=git+https://github.com/foo/skill@v1]",
		formatAppliedFieldSkill("git+https://github.com/foo/skill@v1"))
}
