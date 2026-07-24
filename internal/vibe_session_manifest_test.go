package internal

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
)

func TestVibe_WritesSessionManifestWhenEnabled(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	workspace := t.TempDir()
	g := &fakeExcludeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}
	k := Remuda{
		Session: &captureSessionManager{},
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
		Git:     g,
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:                  "bash",
		Prompt:                 "ignored",
		Detached:               true,
		ExistingWorkspace:      workspace,
		Yolo:                   true,
		Model:                  "some-model",
		SessionManifestEnabled: true,
	})
	require.NoError(t, err)

	manifest, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.True(t, ok, "expected a .remuda.json manifest to be written")
	require.Equal(t, "bash", manifest.Agent)
	require.True(t, manifest.Yolo)
}

func TestVibe_SkipsSessionManifestWhenDisabled(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")

	workspace := t.TempDir()
	k := Remuda{
		Session: &captureSessionManager{},
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "bash",
		Prompt:            "ignored",
		Detached:          true,
		ExistingWorkspace: workspace,
	})
	require.NoError(t, err)

	_, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.False(t, ok, "manifest should not be written when the experiment is disabled")
}

func TestVibe_RefusesToOverwriteExistingManifest(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")

	workspace := t.TempDir()
	g := &fakeExcludeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}
	require.NoError(t, WriteSessionManifest(g, workspace, SessionManifest{Agent: "codex"}))

	k := Remuda{
		Session: &captureSessionManager{},
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
		Git:     g,
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:                  "bash",
		Prompt:                 "ignored",
		Detached:               true,
		ExistingWorkspace:      workspace,
		SessionManifestEnabled: true,
	})
	require.Error(t, err)

	manifest, ok, readErr := ReadSessionManifest(workspace)
	require.NoError(t, readErr)
	require.True(t, ok)
	require.Equal(t, "codex", manifest.Agent, "existing manifest must not be clobbered")
}
