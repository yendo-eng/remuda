package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
)

func TestComposeLaunchCommand_ForwardsContainerInheritEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	// Keep BuildGoCacheMountOpts deterministic + confined to tmp.
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Container:           true,
		ContainerName:       "vibe-dev",
		ContainerInheritEnv: []string{"AWS_REGION", "FOO_BAR", "GOPRIVATE"},
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launchCmd, "'-e' 'AWS_REGION'")
	require.Contains(t, launchCmd, "'-e' 'FOO_BAR'")
	require.Contains(t, launchCmd, "'-e' 'GOPRIVATE'")
}

func TestComposeLaunchCommand_InvalidContainerInheritEnvFails(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Container:           true,
		ContainerName:       "vibe-dev",
		ContainerInheritEnv: []string{"BAD=NOPE"},
	}

	_, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid env var name")
}

func TestComposeLaunchCommand_ContainerRequiresExplicitImage(t *testing.T) {
	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Container: true,
	}

	_, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.Error(t, err)
	require.ErrorContains(t, err, "container mode requires an explicit image")
}

func TestComposeLaunchCommand_BashAgentIncludesCodexStateMounts(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	require.NoError(t, os.WriteFile(claudeJSON, []byte("{}"), 0o600))

	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	// Keep BuildGoCacheMountOpts deterministic + confined to tmp.
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Agent:         "bash",
		Container:     true,
		ContainerName: "vibe-dev",
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launchCmd, "'-e' 'ANTHROPIC_API_KEY'")
	require.Contains(t, launchCmd, ":/root/.codex/history.jsonl:rw")
	require.Contains(t, launchCmd, ":/root/.codex/sessions:rw")
	require.Contains(t, launchCmd, ":/root/.local/share/opencode:rw")
	require.Contains(t, launchCmd, claudeDir+":/root/.claude:rw")
	require.Contains(t, launchCmd, claudeJSON+":/root/.claude.json:rw")
}

func TestComposeLaunchCommand_ClaudeAgentIncludesClaudeStateMountsAndEnv(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	require.NoError(t, os.WriteFile(claudeJSON, []byte("{}"), 0o600))

	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	// Keep BuildGoCacheMountOpts deterministic + confined to tmp.
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Agent:         "claude",
		Container:     true,
		ContainerName: "vibe-dev",
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launchCmd, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, launchCmd, "IS_SANDBOX")
	require.Contains(t, launchCmd, claudeDir+":/root/.claude:rw")
	require.Contains(t, launchCmd, claudeJSON+":/root/.claude.json:rw")
}

func TestComposeLaunchCommand_ClaudeYoloSetsSandboxEnv(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	require.NoError(t, os.WriteFile(claudeJSON, []byte("{}"), 0o600))

	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	// Keep BuildGoCacheMountOpts deterministic + confined to tmp.
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Agent:         "claude",
		Yolo:          true,
		Container:     true,
		ContainerName: "vibe-dev",
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launchCmd, "'-e' 'ANTHROPIC_API_KEY'")
	require.Contains(t, launchCmd, "'-e' 'IS_SANDBOX'")
}

func TestComposeLaunchCommand_CodexAgentOmitsClaudeStateMountsAndEnv(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	require.NoError(t, os.WriteFile(claudeJSON, []byte("{}"), 0o600))

	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")

	// Keep BuildGoCacheMountOpts deterministic + confined to tmp.
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	k := Remuda{
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Agent:         "codex",
		Container:     true,
		ContainerName: "vibe-dev",
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.NotContains(t, launchCmd, "ANTHROPIC_API_KEY")
	require.NotContains(t, launchCmd, "/root/.claude")
	require.NotContains(t, launchCmd, "/root/.claude.json")
}
