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
	"time"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
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
	assert.Equal(t, "finance-creds", vf.SecretKeyRef.Name)
	assert.Equal(t, "apiKey", vf.SecretKeyRef.Key)
	assert.Empty(t, patch.Spec.Skills, "must not touch skills when only env requested")
}

func TestAppliedFieldsFormat(t *testing.T) {
	assert.Equal(t, "spec.env[name=FINANCE_TZ]", formatAppliedFieldEnv("FINANCE_TZ"))
	assert.Equal(t, "spec.skills[source=git+https://github.com/foo/skill@v1]",
		formatAppliedFieldSkill("git+https://github.com/foo/skill@v1"))
}

func TestBuildWorkspaceFilesPatch_NestedPaths(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{
				{Path: "notes/finance.md", Content: "# Finance"},
				{Path: "flat.md", Content: "hello"},
			},
		},
	}
	cm := buildWorkspaceFilesPatch(parent, sc)
	assert.Equal(t, "my-hermes-workspace", cm.Name)
	assert.Equal(t, "agents", cm.Namespace)
	assert.Equal(t, "# Finance", cm.Data["notes__finance.md"])
	assert.Equal(t, "hello", cm.Data["flat.md"])
	assert.Equal(t, "v1", cm.APIVersion)
	assert.Equal(t, "ConfigMap", cm.Kind)
}

func TestBuildPatchConfigPayload_WritesSelfConfigYaml(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			PatchConfig: &apiextensionsv1.JSON{
				Raw: []byte(`{"schedules":{"morning-brief":"0 8 * * *"}}`),
			},
		},
	}
	cm := buildPatchConfigPayload(parent, sc)
	assert.Equal(t, "my-hermes-workspace", cm.Name)
	got := cm.Data["selfconfig.yaml"]
	assert.JSONEq(t, `{"schedules":{"morning-brief":"0 8 * * *"}}`, got)
}

func TestBuildPatchConfigPayload_NilPatch(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{}
	cm := buildPatchConfigPayload(parent, sc)
	assert.Empty(t, cm.Data)
}

func TestMergeConfigMapPatches_CombinesKeys(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{"x":1}`)},
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{
				{Path: "a.md", Content: "x"},
			},
		},
	}
	cm := mergeConfigMapPatches(
		buildPatchConfigPayload(parent, sc),
		buildWorkspaceFilesPatch(parent, sc),
	)
	assert.Equal(t, `{"x":1}`, cm.Data["selfconfig.yaml"])
	assert.Equal(t, "x", cm.Data["a.md"])
}

func TestMergeConfigMapPatches_NilHandling(t *testing.T) {
	parent := parentInstance()
	sc := &hermesv1.HermesSelfConfig{Spec: hermesv1.HermesSelfConfigSpec{
		AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{{Path: "a.md", Content: "x"}},
	}}
	right := buildWorkspaceFilesPatch(parent, sc)
	assert.Same(t, right, mergeConfigMapPatches(nil, right))
	assert.Same(t, right, mergeConfigMapPatches(right, nil))
}

func TestBuildProfileSnapshotPayload_NilWhenEmpty(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{}
	assert.Nil(t, buildProfileSnapshotPayload(parentInstance(), sc, time.Now()))
}

func TestBuildProfileSnapshotPayload_PopulatesJob(t *testing.T) {
	sc := &hermesv1.HermesSelfConfig{
		Spec: hermesv1.HermesSelfConfigSpec{
			AddProfileSnapshot: &hermesv1.SelfConfigProfileSnapshot{
				ProfileID: "user-42",
				Data:      "some-payload",
			},
		},
	}
	job := buildProfileSnapshotPayload(parentInstance(), sc, time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC))
	assert.NotNil(t, job)
	assert.Equal(t, "my-hermes-snapshot-user-42-20260512080000", job.Name)
}
