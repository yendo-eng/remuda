package agentlauncher_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
)

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"codex version 1.2.3", "v1.2.3"},
		{"v2.0.0-beta+build", "v2.0.0-beta"},
		{"OpenCode CLI 0.9.0", "v0.9.0"},
	}
	for _, c := range cases {
		got, err := agentlauncher.ParseSemver(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got)
	}

	_, err := agentlauncher.ParseSemver("no version here")
	require.Error(t, err)
}

func TestMeetsMinimum(t *testing.T) {
	// Empty inputs trivially pass.
	assert.True(t, agentlauncher.MeetsMinimum("", ""))
	assert.True(t, agentlauncher.MeetsMinimum("v1.0.0", ""))
	assert.True(t, agentlauncher.MeetsMinimum("", "v1.0.0"))

	assert.True(t, agentlauncher.MeetsMinimum("v1.2.3", "v1.2.3"))
	assert.True(t, agentlauncher.MeetsMinimum("v1.3.0", "v1.2.3"))
	assert.False(t, agentlauncher.MeetsMinimum("v1.2.2", "v1.2.3"))
}
