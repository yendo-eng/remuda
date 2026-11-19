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
		Agent:               "claude",
		Prompt:              "ignored",
		Detached:            true,
		ExistingWorkspace:   workspace,
		Yolo:                true,
		Model:               "claude-opus-4-8",
		ReasoningLevel:      "high",
		AgentArgs:           []string{"--extra-arg"},
		UsePromptIDs:        []string{"make-pr"},
		UsePromptsPosition:  "after",
		Container:           true,
		ContainerName:       "manifest-image",
		ContainerOpts:       []string{"--cpus=2"},
		ContainerInheritEnv: []string{"MANIFEST_TOKEN"},
	})
	require.NoError(t, err)

	manifest, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.True(t, ok, "expected a .remuda.json manifest to be written")
	require.Equal(t, SessionManifest{
		SchemaVersion:  SessionManifestSchemaVersion,
		Agent:          "claude",
		Model:          "claude-opus-4-8",
		ReasoningLevel: "high",
		Yolo:           true,
		AgentCmd:       "",
		AgentArgs:      []string{"--extra-arg"},
		UsePrompts:     []string{"make-pr"},
		UsePosition:    "after",
		Container: SessionManifestContainer{
			Enabled:    true,
			Image:      "manifest-image",
			Opts:       []string{"--cpus=2"},
			InheritEnv: []string{"MANIFEST_TOKEN"},
		},
	}, manifest)
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
