package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launch.shell, "'-e' 'AWS_REGION'")
	require.Contains(t, launch.shell, "'-e' 'FOO_BAR'")
	require.Contains(t, launch.shell, "'-e' 'GOPRIVATE'")
}

// TestComposeLaunchCommand_SplitsMultiTokenContainerOpt verifies that a raw
// --container-opt value carrying more than one docker CLI token (eg. "-v
// /a:/b", typed by the operator as a single flag value) is field-split into
// separate argv elements before docker's own quoting boundary, matching the
// word-splitting a shell would previously have done. Executes the built
// command through bash with a fake `docker` on PATH so the assertion is on
// the actual argv docker receives, not on the intermediate string shape.
func TestComposeLaunchCommand_SplitsMultiTokenContainerOpt(t *testing.T) {
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
		Container:     true,
		ContainerName: "vibe-dev",
		ContainerOpts: []string{"-v /a:/b"},
	}

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)

	binDir := t.TempDir()
	capturePath := filepath.Join(binDir, "docker-args.txt")
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"$CAPTURE_ARGS\"\n"
	require.NoError(t, os.WriteFile(dockerPath, []byte(script), 0o755))

	//nolint:gosec // G204: this test intentionally executes the generated shell command to verify quoting behavior.
	run := exec.CommandContext(context.Background(), "bash", "-c", launch.shell)
	run.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CAPTURE_ARGS="+capturePath,
	)
	out, runErr := run.CombinedOutput()
	require.NoError(t, runErr, "command should not fail:\n%s", string(out))

	argsDump, err := os.ReadFile(capturePath)
	require.NoError(t, err)
	argv := strings.Split(strings.TrimRight(string(argsDump), "\n"), "\n")

	found := false
	for i, tok := range argv {
		if tok == "-v" && i+1 < len(argv) && argv[i+1] == "/a:/b" {
			found = true
			break
		}
	}
	require.True(t, found, "expected docker to receive \"-v\" and \"/a:/b\" as separate args, got: %v", argv)
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

	_, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
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

	_, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
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

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launch.shell, "'-e' 'ANTHROPIC_API_KEY'")
	require.Contains(t, launch.shell, ":/root/.codex/history.jsonl:rw")
	require.Contains(t, launch.shell, ":/root/.codex/sessions:rw")
	require.Contains(t, launch.shell, ":/root/.local/share/opencode:rw")
	require.Contains(t, launch.shell, claudeDir+":/root/.claude:rw")
	require.Contains(t, launch.shell, claudeJSON+":/root/.claude.json:rw")
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

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launch.shell, "'-e' 'ANTHROPIC_API_KEY'")
	require.NotContains(t, launch.shell, "IS_SANDBOX")
	require.Contains(t, launch.shell, claudeDir+":/root/.claude:rw")
	require.Contains(t, launch.shell, claudeJSON+":/root/.claude.json:rw")
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

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.Contains(t, launch.shell, "'-e' 'ANTHROPIC_API_KEY'")
	require.Contains(t, launch.shell, "'-e' 'IS_SANDBOX'")
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

	launch, err := k.composeLaunchCommand(cmd, "/tmp/ws", "echo hi", "sess", "cont", k.envProvider())
	require.NoError(t, err)
	require.NotContains(t, launch.shell, "ANTHROPIC_API_KEY")
	require.NotContains(t, launch.shell, "/root/.claude")
	require.NotContains(t, launch.shell, "/root/.claude.json")
}
