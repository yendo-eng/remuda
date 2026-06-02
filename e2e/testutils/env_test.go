package testutils

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2EEnvIsolationContract_SanitizeSubprocessEnv(t *testing.T) {
	tempHome := t.TempDir()
	contract := DefaultE2EEnvIsolationContract(tempHome)

	parent := []string{
		"PATH=/bin:/usr/bin",
		"REMUDA_USE_PROMPTS=host-value",
		"GIT_CONFIG_KEY_0=user.name",
		"GIT_CONFIG_VALUE_0=Host User",
		"HOME=/host/home",
		"TERM=xterm-256color",
		"UNRELATED=host-leak",
	}

	env := contract.SanitizeSubprocessEnv(parent, map[string]string{
		"TERM": "dumb",
	})

	envMap := parseEnv(env)

	require.Equal(t, "/bin:/usr/bin", envMap["PATH"])
	require.Equal(t, "dumb", envMap["TERM"])
	require.NotContains(t, envMap, "UNRELATED")
	require.NotContains(t, envMap, "REMUDA_USE_PROMPTS")

	// Forced vars always present and override parent values.
	require.NotEqual(t, "/host/home", envMap["HOME"])
	require.Equal(t, "UTC", envMap["TZ"])
	require.Equal(t, PreferredE2ELocale(), envMap["LANG"])
	require.Equal(t, PreferredE2ELocale(), envMap["LC_ALL"])
	require.Equal(t, "1", envMap["GIT_CONFIG_NOSYSTEM"])
	require.Equal(t, GitConfigGlobalPath(tempHome), envMap["GIT_CONFIG_GLOBAL"])

	// Ensure output is deterministic (sorted).
	require.NotEmpty(t, env)
	require.True(t, sort.StringsAreSorted(env), "env output must be sorted for determinism")

	// Sanity check: must be safe to use directly as exec.Cmd.Env.
	for _, kv := range env {
		require.True(t, strings.Contains(kv, "="), "bad env entry: %q", kv)
	}
}

func TestE2EEnvIsolationContract_SanitizeProcessEnv(t *testing.T) {
	tempHome := t.TempDir()
	contract := DefaultE2EEnvIsolationContract(tempHome)

	parent := []string{
		"REMUDA_USE_PROMPTS=host-value",
		"GIT_CONFIG_KEY_0=user.name",
		"GIT_CONFIG_VALUE_0=Host User",
		"HOME=/host/home",
		"UNRELATED=host-kept",
	}

	env := contract.SanitizeProcessEnv(parent, nil)
	envMap := parseEnv(env)

	require.NotContains(t, envMap, "REMUDA_USE_PROMPTS")
	require.NotContains(t, envMap, "GIT_CONFIG_KEY_0")
	require.NotContains(t, envMap, "GIT_CONFIG_VALUE_0")
	require.Equal(t, "host-kept", envMap["UNRELATED"])
	require.NotEqual(t, "/host/home", envMap["HOME"])
	require.Equal(t, GitConfigGlobalPath(tempHome), envMap["GIT_CONFIG_GLOBAL"])
}

func TestApplyE2EEnvIsolationToCmd(t *testing.T) {
	home := t.TempDir()
	baseEnv := map[string]string{
		"HOME":           home,
		"UNRELATED_LEAK": "host-leak",
	}

	cmd := exec.CommandContext(context.Background(), "echo", "ok")
	require.NoError(t, ApplyE2EEnvIsolationToCmd(cmd, baseEnv, nil))

	envMap := parseEnv(cmd.Env)
	require.NotContains(t, envMap, "UNRELATED_LEAK")
	require.Equal(t, home, envMap["HOME"])
	require.Equal(t, GitConfigGlobalPath(home), envMap["GIT_CONFIG_GLOBAL"])
}
