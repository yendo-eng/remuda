package testutils

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHarness_SetsHomeAndXDG(t *testing.T) {
	h := NewHarness(t)

	require.Equal(t, h.HomeDir, h.Getenv("HOME"))
	require.Equal(t, filepath.Join(h.HomeDir, ".config"), h.Getenv("XDG_CONFIG_HOME"))
	require.Equal(t, filepath.Join(h.HomeDir, ".cache"), h.Getenv("XDG_CACHE_HOME"))
	require.Equal(t, filepath.Join(h.HomeDir, ".local", "state"), h.Getenv("XDG_STATE_HOME"))
	require.Equal(t, filepath.Join(h.HomeDir, ".local", "share"), h.Getenv("XDG_DATA_HOME"))
	require.Equal(t, GitConfigGlobalPath(h.HomeDir), h.Getenv("GIT_CONFIG_GLOBAL"))
	require.Equal(t, GitConfigSystemPath(h.HomeDir), h.Getenv("GIT_CONFIG_SYSTEM"))
}

func TestHarness_ClearsIsolationPrefixes(t *testing.T) {
	baseEnv := map[string]string{
		"REMUDA_USE_PROMPTS": "host-value",
		"GIT_CONFIG_KEY_0":   "user.name",
		"GIT_CONFIG_VALUE_0": "Host User",
		"GIT_ATTR_FOO":       "bar",
	}

	h := newHarness(t, baseEnv)

	_, ok := h.Env.LookupEnv("REMUDA_USE_PROMPTS")
	require.False(t, ok)
	_, ok = h.Env.LookupEnv("GIT_CONFIG_KEY_0")
	require.False(t, ok)
	_, ok = h.Env.LookupEnv("GIT_CONFIG_VALUE_0")
	require.False(t, ok)
	_, ok = h.Env.LookupEnv("GIT_ATTR_FOO")
	require.False(t, ok)

	// Forced vars should still be applied.
	require.Equal(t, "1", h.Getenv("GIT_ATTR_NOSYSTEM"))
	require.Equal(t, h.HomeDir, h.Getenv("HOME"))
}

func TestHarness_Run_CapturesOutput(t *testing.T) {
	h := NewHarness(t)
	res := h.RunOK("repo", "list", "--json")
	var decoded []map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &decoded), res.String())
	require.Empty(t, res.Stderr, res.String())
}
