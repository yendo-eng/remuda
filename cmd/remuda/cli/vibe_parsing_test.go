package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal/enums"
)

func TestVibe_ContainerOptsEnvVar(t *testing.T) {
	t.Parallel()
	t.Run("remuda vibe --container-opt should default to an environment variable", func(t *testing.T) {
		env := cli.EnvMap{"REMUDA_CONTAINER_OPTS": "--network host,--gpus all"}
		parser, parsed, _ := newParserWithEnv(t, env)

		_, err := parser.Parse([]string{
			"vibe",
			"--name", "wk",
			"--agent-cmd", "true",
			"--container",
			"hello",
		})
		require.NoError(t, err, "expected parsing to succeed with env defaults")
		require.Equal(t, []string{"--network host", "--gpus all"}, parsed.Vibe.ContainerOpt)
	})

	t.Run("remuda vibe --container-opt should override the env var if present", func(t *testing.T) {
		env := cli.EnvMap{"REMUDA_CONTAINER_OPTS": "--network host,--gpus all"}
		parser, parsed, _ := newParserWithEnv(t, env)

		_, err := parser.Parse([]string{
			"vibe",
			"--name", "wk",
			"--agent-cmd", "true",
			"--container",
			"--container-opt=--rm",
			"hello",
		})
		require.NoError(t, err, "expected parsing to succeed with env defaults")
		require.Equal(t, []string{"--rm"}, parsed.Vibe.ContainerOpt)
	})
}

func TestVibe_ContainerInheritEnv_EnvVar(t *testing.T) {
	t.Parallel()
	t.Run("remuda vibe --container-inherit-env should default to an environment variable", func(t *testing.T) {
		env := cli.EnvMap{"REMUDA_CONTAINER_INHERIT_ENVS": "AWS_REGION,FOO_BAR"}
		parser, parsed, _ := newParserWithEnv(t, env)

		_, err := parser.Parse([]string{
			"vibe",
			"--name", "wk",
			"--agent-cmd", "true",
			"--container",
			"hello",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"AWS_REGION", "FOO_BAR"}, parsed.Vibe.ContainerInheritEnv)
	})

	t.Run("remuda vibe --container-inherit-env should reject invalid env var names", func(t *testing.T) {
		parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

		_, err := parser.Parse([]string{
			"vibe",
			"--name", "wk",
			"--agent-cmd", "true",
			"--container",
			"--container-inherit-env", "BAD=NOPE",
			"hello",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "--container-inherit-env")
	})
}

func TestVibe_NegatableContainerFlag(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{"REMUDA_CONTAINER": "true"}
	parser, parsed, _ := newParserWithEnv(t, env)

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--agent-cmd", "true",
		"--no-container",
		"hello",
	})
	require.NoError(t, err, "expected kong to accept --no-container")
	require.False(t, parsed.Vibe.Container, "--no-container should disable container mode")
}

func TestVibeStart_NegatableYoloFlag(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{"REMUDA_YOLO": "true"}
	parser, parsed, _ := newParserWithEnv(t, env)

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--agent-cmd", "true",
		"--no-yolo",
		"hello",
	})
	require.NoError(t, err, "expected kong to accept --no-yolo")
	require.False(t, parsed.Vibe.Yolo, "--no-yolo should disable yolo mode despite the env var")
}

func TestVibe_RemoteFlag_ParsesPresenceOnly(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--remote",
		"--agent-cmd", "true",
		"hello",
	})
	require.NoError(t, err)
	require.True(t, parsed.Vibe.Remote)
}

func TestVibe_RemoteFlag_DoesNotConsumeFollowingPromptToken(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--remote", "hello prompt",
		"--agent-cmd", "true",
	})
	require.NoError(t, err)
	require.True(t, parsed.Vibe.Remote)
	require.Equal(t, "hello prompt", parsed.Vibe.Prompt)
}

func TestVibe_RemoteFlag_RejectsAttachedValue(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--remote=my-session",
		"--agent-cmd", "true",
		"prompt",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "--remote")
}

// Ensure Kong parsing accepts --repo utils for vibe start.
func TestVibe_KongEnum_AllowsUtils(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	// Parse without running: just validate flags/enum.
	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--repo", "utils",
		"hello prompt",
	})
	require.NoError(t, err, "expected kong to accept --repo utils")
	require.NotNil(t, parsed.Vibe.Repo)
	require.Equal(t, "utils", *parsed.Vibe.Repo)
}

func TestVibe_RepoURLExpandsGithubShorthand(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--repo-url", "github.com/acme/tools",
		"hello prompt",
	})
	require.NoError(t, err)
	require.NotNil(t, parsed.Vibe.RepoURL)
	require.Equal(t, "https://github.com/acme/tools.git", *parsed.Vibe.RepoURL)
}

func TestAgentEnum_AcceptsCanonicalAgentsForVibeAndVibeCheck(t *testing.T) {
	t.Parallel()
	for _, agent := range enums.ValidAgents {
		agent := agent
		t.Run(agent, func(t *testing.T) {
			t.Parallel()

			vibeParser, _, _ := newParserWithEnv(t, cli.EnvMap{})
			_, err := vibeParser.Parse([]string{
				"vibe",
				"--name", "wk",
				"--agent", agent,
				"--agent-cmd", "true",
				"hello",
			})
			require.NoError(t, err)

			vibeCheckParser, _, _ := newParserWithEnv(t, cli.EnvMap{})
			_, err = vibeCheckParser.Parse([]string{
				"vibe-check",
				"--agent", agent,
				"--agent-cmd", "true",
				"main",
			})
			require.NoError(t, err)
		})
	}
}

func TestVibe_MissingNameSucceedsParse(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{"vibe", "--agent-cmd", "true", "hello"})
	require.NoError(t, err)
	require.Empty(t, parsed.Vibe.Name, "name is generated at runtime, not during parse")
}

func TestVibe_InFlagAllowsMissingName(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})
	workspace := t.TempDir()

	_, err := parser.Parse([]string{
		"vibe",
		"--in", workspace,
		"--agent-cmd", "true",
		"prompt",
	})
	require.NoError(t, err)
	require.Empty(t, parsed.Vibe.Name)
}

func TestVibe_InFlagRejectsExplicitName(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})
	workspace := t.TempDir()

	_, err := parser.Parse([]string{
		"vibe",
		"--in", workspace,
		"--name", "wk",
		"prompt",
	})
	require.ErrorContains(t, err, "--name cannot be combined with --in")
}

func TestVibeUseUnknownFailsParse(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

	args := []string{"vibe", "--use", "does-not-exist", "--name", "wk", "hi"}
	_, err := parser.Parse(args)
	require.ErrorContains(t, err, "unknown prompt: does-not-exist")
}

func TestVibeNoUseUnknownFailsParse(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

	args := []string{"vibe", "--no-use", "does-not-exist", "--name", "wk", "hi"}
	_, err := parser.Parse(args)
	require.ErrorContains(t, err, "unknown prompt: does-not-exist")
}

func TestVibeCheckNoUseUnknownFailsParse(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

	args := []string{"vibe-check", "--no-use", "does-not-exist", "main"}
	_, err := parser.Parse(args)
	require.ErrorContains(t, err, "unknown prompt: does-not-exist")
}

func TestVibeCheck_NoCloneHooksFlagParses(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{"vibe-check", "--no-clone-hooks", "main"})
	require.NoError(t, err)
	require.True(t, parsed.VibeCheck.NoCloneHooks)
}

func TestVibe_BranchFlagParses(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--branch", "feature/x",
		"--agent-cmd", "true",
		"prompt",
	})
	require.NoError(t, err)
	require.Equal(t, "feature/x", parsed.Vibe.Branch)
}

func TestVibe_BranchFlagRejectedWithIn(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})
	workspace := t.TempDir()

	_, err := parser.Parse([]string{
		"vibe",
		"--in", workspace,
		"--branch", "feature/x",
		"--agent-cmd", "true",
		"prompt",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "--branch cannot be combined with --in")
}
