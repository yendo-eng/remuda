package internal

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func TestVibe_WritesSessionManifest(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	workspace := t.TempDir()
	g := &fakeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}
	k := Remuda{
		Multiplexer: &testutils.MockMultiplexer{MultiplexerName: "tmux"},
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
		Git:         g,
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "bash",
		Prompt:            "ignored",
		Detached:          true,
		ExistingWorkspace: workspace,
		Yolo:              true,
		Model:             "some-model",
	})
	require.NoError(t, err)

	manifest, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.True(t, ok, "expected a .remuda.json manifest to be written")
	require.Equal(t, "bash", manifest.Agent)
	require.True(t, manifest.Yolo)
}

func TestVibe_DoesNotOverwriteExistingManifest(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")

	workspace := t.TempDir()
	g := &fakeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}
	require.NoError(t, WriteSessionManifest(g, workspace, SessionManifest{Agent: "codex"}))

	k := Remuda{
		Multiplexer: &testutils.MockMultiplexer{MultiplexerName: "tmux"},
		Docker:      &docker.Mock{Running: true},
		IO:          DefaultIO(),
		Git:         g,
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "bash",
		Prompt:            "ignored",
		Detached:          true,
		ExistingWorkspace: workspace,
	})
	require.NoError(t, err)

	manifest, ok, readErr := ReadSessionManifest(workspace)
	require.NoError(t, readErr)
	require.True(t, ok)
	require.Equal(t, "codex", manifest.Agent, "existing manifest must not be clobbered")
}
