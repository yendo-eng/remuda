package internal

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
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
		IsolationRuntime:    "smolvm",
		Container:           true,
		ContainerName:       "manifest-image",
		ContainerOpts:       []string{"--cpus=2"},
		ContainerInheritEnv: []string{"MANIFEST_TOKEN"},
	})
	require.NoError(t, err)

	manifest, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.True(t, ok, "expected a .remuda.json manifest to be written")
	agent := "claude"
	model := "claude-opus-4-8"
	reasoningLevel := "high"
	agentCmd := ""
	yolo := true
	usePrompts := []string{"make-pr"}
	usePromptsPosition := "after"
	containerEnabled := true
	containerImage := "manifest-image"
	containerOpts := []string{"--cpus=2"}
	containerInheritEnv := []string{"MANIFEST_TOKEN"}
	isolationRuntime := "smolvm"
	require.Equal(t, configfile.DefaultsV1{
		Agent:              &agent,
		Model:              &model,
		ReasoningLevel:     &reasoningLevel,
		AgentCmd:           &agentCmd,
		AgentArgs:          map[string][]string{"claude": {"--extra-arg"}},
		UsePrompts:         &usePrompts,
		UsePromptsPosition: &usePromptsPosition,
		Yolo:               &yolo,
		Isolation: &configfile.IsolationV1{
			Enabled:    &containerEnabled,
			Runtime:    &isolationRuntime,
			Image:      &containerImage,
			Opts:       &containerOpts,
			InheritEnv: &containerInheritEnv,
		},
	}, manifest.Defaults)
}

func TestVibe_DoesNotOverwriteExistingManifest(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")

	workspace := t.TempDir()
	g := &fakeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}
	require.NoError(t, WriteSessionManifest(g, workspace, sessionManifestWithAgent("codex")))

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
	require.NotNil(t, manifest.Defaults.Agent)
	require.Equal(t, "codex", *manifest.Defaults.Agent, "existing manifest must not be clobbered")
}
