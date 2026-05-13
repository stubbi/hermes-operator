package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func TestDetermineActions(t *testing.T) {
	t.Run("skills only", func(t *testing.T) {
		sc := &hermesv1.HermesSelfConfig{
			Spec: hermesv1.HermesSelfConfigSpec{
				AddSkills: []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			},
		}
		got := DetermineActions(sc)
		assert.Equal(t, []hermesv1.SelfConfigAction{hermesv1.ActionSkills}, got)
	})
	t.Run("multiple actions", func(t *testing.T) {
		sc := &hermesv1.HermesSelfConfig{Spec: hermesv1.HermesSelfConfigSpec{
			AddSkills:         []hermesv1.SelfConfigSkill{{Source: "x"}},
			AddEnvVars:        []hermesv1.SelfConfigEnvVar{{Name: "X", Value: "y"}},
			AddWorkspaceFiles: []hermesv1.SelfConfigWorkspaceFile{{Path: "a.md", Content: "x"}},
		}}
		got := DetermineActions(sc)
		assert.ElementsMatch(t,
			[]hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionEnvVars, hermesv1.ActionWorkspaceFiles},
			got)
	})
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, DetermineActions(&hermesv1.HermesSelfConfig{}))
	})
}

func TestCheckAllowedActions(t *testing.T) {
	allowed := []hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionConfig}
	t.Run("all allowed", func(t *testing.T) {
		denied := CheckAllowedActions([]hermesv1.SelfConfigAction{hermesv1.ActionConfig}, allowed)
		assert.Empty(t, denied)
	})
	t.Run("some denied", func(t *testing.T) {
		denied := CheckAllowedActions(
			[]hermesv1.SelfConfigAction{hermesv1.ActionSkills, hermesv1.ActionEnvVars, hermesv1.ActionProfiles},
			allowed,
		)
		assert.ElementsMatch(t,
			[]hermesv1.SelfConfigAction{hermesv1.ActionEnvVars, hermesv1.ActionProfiles},
			denied)
	})
	t.Run("none allowed = all denied", func(t *testing.T) {
		denied := CheckAllowedActions([]hermesv1.SelfConfigAction{hermesv1.ActionSkills}, nil)
		assert.Equal(t, []hermesv1.SelfConfigAction{hermesv1.ActionSkills}, denied)
	})
}

func TestCheckProtectedPaths(t *testing.T) {
	protected := []string{"provider.apiKey", "*.secret*", "gateways.telegram.token"}
	t.Run("clean patch passes", func(t *testing.T) {
		raw := []byte(`{"schedules":{"morning":"0 8 * * *"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Empty(t, hit)
	})
	t.Run("exact-match protection", func(t *testing.T) {
		raw := []byte(`{"provider":{"apiKey":"sk-xxx"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "provider.apiKey", hit)
	})
	t.Run("glob protection by suffix", func(t *testing.T) {
		raw := []byte(`{"db":{"secretKey":"x"}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "db.secretKey", hit, "matched against *.secret*")
	})
	t.Run("nested gateway token", func(t *testing.T) {
		raw := []byte(`{"gateways":{"telegram":{"token":"x"}}}`)
		hit, err := CheckProtectedPaths(raw, protected)
		assert.NoError(t, err)
		assert.Equal(t, "gateways.telegram.token", hit)
	})
	t.Run("invalid JSON errors", func(t *testing.T) {
		_, err := CheckProtectedPaths([]byte(`{invalid`), protected)
		assert.Error(t, err)
	})
}
