package oci

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeRegistry_ListTags(t *testing.T) {
	fake := NewFake()
	fake.SetTags("ghcr.io/stubbi/hermes-agent", []string{"1.0.0", "1.0.1", "1.1.0", "2.0.0-rc1"})
	tags, err := fake.ListTags(context.Background(), "ghcr.io/stubbi/hermes-agent")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"1.0.0", "1.0.1", "1.1.0", "2.0.0-rc1"}, tags)
}

func TestHighestMatching_BasicSemver(t *testing.T) {
	tags := []string{"1.0.0", "1.0.1", "1.1.0", "2.0.0-rc1", "not-a-semver"}
	best, err := HighestMatching(tags, "1.x")
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", best)
}

func TestHighestMatching_SameMajorDefault(t *testing.T) {
	tags := []string{"1.0.0", "1.5.3", "2.0.0"}
	best, err := HighestMatching(tags, ">=1.0.0 <2.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.5.3", best)
}

func TestHighestMatching_NoMatch(t *testing.T) {
	tags := []string{"1.0.0"}
	_, err := HighestMatching(tags, "3.x")
	assert.ErrorIs(t, err, ErrNoMatchingTag)
}

func TestHighestMatching_SkipPrerelease(t *testing.T) {
	tags := []string{"1.0.0", "1.1.0-rc1"}
	best, err := HighestMatching(tags, "1.x")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", best)
}
