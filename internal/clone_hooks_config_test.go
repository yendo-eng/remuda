package internal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestConfigCloneHookRunsWithWorktreeCWD(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()

	hook := internal.NewConfigCloneHook("write-cwd", []string{"/bin/sh", "-c", "pwd > hook-cwd.txt"})
	ctx := internal.CloneHookContext{
		RepoURL:     "https://github.com/acme/rocket.git",
		Org:         "acme",
		Repo:        "rocket",
		CacheDir:    t.TempDir(),
		WorktreeDir: worktreeDir,
	}

	require.NoError(t, hook.Run(ctx))

	data, err := os.ReadFile(filepath.Join(worktreeDir, "hook-cwd.txt"))
	require.NoError(t, err)

	actualCWD := strings.TrimSpace(string(data))
	expectedRealPath, err := filepath.EvalSymlinks(worktreeDir)
	require.NoError(t, err)
	actualRealPath, err := filepath.EvalSymlinks(actualCWD)
	require.NoError(t, err)
	require.Equal(t, expectedRealPath, actualRealPath)
}

func TestConfigCloneHookInjectsRemudaEnvVars(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()
	cacheDir := t.TempDir()

	hook := internal.NewConfigCloneHook("dump-env", []string{
		"/bin/sh",
		"-c",
		`printf "%s\n" "$REMUDA_REPO_URL|$REMUDA_REPO_ORG|$REMUDA_REPO_NAME|$REMUDA_REPO_SLUG|$REMUDA_CACHE_DIR|$REMUDA_WORKTREE_DIR|$CUSTOM_VALUE" > hook-env.txt`,
	})
	ctx := internal.CloneHookContext{
		RepoURL:     "https://github.com/Acme/Rocket.git",
		Org:         "Acme",
		Repo:        "Rocket",
		CacheDir:    cacheDir,
		WorktreeDir: worktreeDir,
		Env: env.StaticProvider{Values: map[string]string{
			"REMUDA_REPO_URL": "stale-value",
			"CUSTOM_VALUE":    "present",
		}},
	}

	require.NoError(t, hook.Run(ctx))

	data, err := os.ReadFile(filepath.Join(worktreeDir, "hook-env.txt"))
	require.NoError(t, err)
	require.Equal(
		t,
		"https://github.com/Acme/Rocket.git|Acme|Rocket|acme/rocket|"+cacheDir+"|"+worktreeDir+"|present",
		strings.TrimSpace(string(data)),
	)
}

func TestConfigCloneHookEmptyArgvFailsAtRuntime(t *testing.T) {
	t.Parallel()

	hook := internal.NewConfigCloneHook("bad", nil)
	err := hook.Run(internal.CloneHookContext{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "argv is empty")
}

func TestCloneHookRegistrySetConfigHooksReplacesPriorConfigHooks(t *testing.T) {
	t.Parallel()

	registry := internal.NewCloneHookRegistry()
	calls := []string{}
	base := internal.NewCloneHook("base", func(_ internal.CloneHookContext) error {
		calls = append(calls, "base")
		return nil
	})
	configA := internal.NewCloneHook("config-a", func(_ internal.CloneHookContext) error {
		calls = append(calls, "config-a")
		return nil
	})
	configB := internal.NewCloneHook("config-b", func(_ internal.CloneHookContext) error {
		calls = append(calls, "config-b")
		return nil
	})

	registry.Register("acme", "rocket", base)
	registry.SetConfigHooks(map[string][]internal.CloneHook{
		"acme/rocket": {configA},
	})

	require.NoError(t, registry.RunCloneHooks(internal.CloneHookContext{Org: "acme", Repo: "rocket"}))
	require.Equal(t, []string{"base", "config-a"}, calls)

	calls = nil
	registry.SetConfigHooks(map[string][]internal.CloneHook{
		"acme/rocket": {configB},
	})
	require.NoError(t, registry.RunCloneHooks(internal.CloneHookContext{Org: "acme", Repo: "rocket"}))
	require.Equal(t, []string{"base", "config-b"}, calls)
}
