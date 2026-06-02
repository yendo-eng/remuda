package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestApplyCloneHooksFromConfig_AppendsAfterRegisteredHooks(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()
	orderFile := filepath.Join(worktreeDir, "hook-order.txt")

	registry := internal.NewCloneHookRegistry()
	registry.Register("acme", "rocket", internal.NewCloneHook("base", func(_ internal.CloneHookContext) error {
		f, err := os.OpenFile(orderFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()
		_, err = f.WriteString("base\n")
		return err
	}))

	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: t.TempDir()},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
		internal.WithCloneHooks(registry),
		internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}),
	)
	kctx := Context{Remuda: k}

	cfg := &configfile.V1{
		PerRepo: map[string]configfile.OverlayV1{
			"acme/rocket": {
				CloneHooks: []configfile.CloneHookV1{
					{Name: "config", Argv: []string{"/bin/sh", "-c", "echo config >> hook-order.txt"}},
				},
			},
		},
	}

	applyCloneHooksFromConfig(&kctx, cfg)
	err := kctx.Remuda.CloneHooks.RunCloneHooks(internal.CloneHookContext{
		Org:         "acme",
		Repo:        "rocket",
		WorktreeDir: worktreeDir,
		CacheDir:    t.TempDir(),
	})
	require.NoError(t, err)

	data, err := os.ReadFile(orderFile)
	require.NoError(t, err)
	require.Equal(t, "base\nconfig\n", string(data))
}

func TestApplyCloneHooksFromConfig_ReplacesAndClearsConfigHooks(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()
	markerFile := filepath.Join(worktreeDir, "marker.txt")

	registry := internal.NewCloneHookRegistry()
	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: t.TempDir()},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
		internal.WithCloneHooks(registry),
		internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}),
	)
	kctx := Context{Remuda: k}

	cfgA := &configfile.V1{
		PerRepo: map[string]configfile.OverlayV1{
			"acme/rocket": {
				CloneHooks: []configfile.CloneHookV1{
					{Argv: []string{"/bin/sh", "-c", "echo first > marker.txt"}},
				},
			},
		},
	}
	applyCloneHooksFromConfig(&kctx, cfgA)
	require.NoError(t, kctx.Remuda.CloneHooks.RunCloneHooks(internal.CloneHookContext{
		Org:         "acme",
		Repo:        "rocket",
		WorktreeDir: worktreeDir,
		CacheDir:    t.TempDir(),
	}))
	data, err := os.ReadFile(markerFile)
	require.NoError(t, err)
	require.Equal(t, "first\n", string(data))

	cfgB := &configfile.V1{
		PerRepo: map[string]configfile.OverlayV1{
			"acme/rocket": {
				CloneHooks: []configfile.CloneHookV1{
					{Argv: []string{"/bin/sh", "-c", "echo second > marker.txt"}},
				},
			},
		},
	}
	applyCloneHooksFromConfig(&kctx, cfgB)
	require.NoError(t, kctx.Remuda.CloneHooks.RunCloneHooks(internal.CloneHookContext{
		Org:         "acme",
		Repo:        "rocket",
		WorktreeDir: worktreeDir,
		CacheDir:    t.TempDir(),
	}))
	data, err = os.ReadFile(markerFile)
	require.NoError(t, err)
	require.Equal(t, "second\n", string(data))

	applyCloneHooksFromConfig(&kctx, nil)
	require.NoError(t, os.Remove(markerFile))
	require.NoError(t, kctx.Remuda.CloneHooks.RunCloneHooks(internal.CloneHookContext{
		Org:         "acme",
		Repo:        "rocket",
		WorktreeDir: worktreeDir,
		CacheDir:    t.TempDir(),
	}))
	_, err = os.Stat(markerFile)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestApplyCloneHooksFromConfig_GeneratesDefaultHookNames(t *testing.T) {
	t.Parallel()
	registry := internal.NewCloneHookRegistry()
	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: t.TempDir()},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
		internal.WithCloneHooks(registry),
		internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}),
	)
	kctx := Context{Remuda: k}

	cfg := &configfile.V1{
		PerRepo: map[string]configfile.OverlayV1{
			"acme/rocket": {
				CloneHooks: []configfile.CloneHookV1{
					{Argv: nil},
				},
			},
		},
	}
	applyCloneHooksFromConfig(&kctx, cfg)

	err := kctx.Remuda.CloneHooks.RunCloneHooks(internal.CloneHookContext{
		Org:  "acme",
		Repo: "rocket",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "hook config-hook-1 failed")
}
