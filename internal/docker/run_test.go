package docker_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
)

func TestBuildRunCommand_ComposesExpectedPieces(t *testing.T) {
	ws := "/abs/path with spaces/repos/org/repo/repo_1"
	img := "vibe-dev"
	opts := []string{"--gpus all", "--network host"}
	agent := "codex 'Do the thing'"

	containerName := "session-container"
	cmd := docker.BuildRunCommand(ws, img, opts, agent, false, containerName)
	containerWS := docker.ContainerWorkspacePath(ws)

	checks := []string{
		"docker run --rm -it",
		"--name session-container",
		fmt.Sprintf("-v %q", ws+":"+containerWS),
		"-w " + containerWS,
		"-e OPENAI_API_KEY",
		"-e REMUDA_AGENT",
		"-e REMUDA_MODEL",
		"-e GH_TOKEN",
		"-e GITHUB_TOKEN",
		"-e GIT_TERMINAL_PROMPT=0",
		"-e GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=accept-new'",
		"--gpus all",
		"--network host",
		"vibe-dev",
		"credential.helper",
		"store --file=/root/.git-credentials",
		"/root/.git-credentials",
		"gh auth setup-git -h github.com",
		`git config --global url."git@github.com:".insteadOf "https://github.com/"`,
		"bash -lc '",
		"codex",
		"Do the thing",
	}
	for _, want := range checks {
		require.Contains(t, cmd, want, "docker command missing %q. Got:\n%s", want, cmd)
	}
	require.NotContains(t, cmd, `bash -lc "`)
}

func TestBuildRunCommand_BackticksAreNotExecutedByOuterShell(t *testing.T) {
	t.Parallel()

	ws := "/abs/path/repos/org/repo/repo_1"
	img := "vibe-dev"
	agent := "echo 'Migration:\n\n```\ncreate table payment_methods(\n  status text not null default '\\''active'\\''\n);\n```'"

	cmd := docker.BuildRunCommand(ws, img, nil, agent, false, "")

	binDir := t.TempDir()
	capturePath := filepath.Join(binDir, "docker-args.txt")
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%q\\n' \"$@\" > \"$CAPTURE_ARGS\"\n"
	require.NoError(t, os.WriteFile(dockerPath, []byte(script), 0o755))

	//nolint:gosec // G204: this test intentionally executes the generated shell command to verify quoting behavior.
	run := exec.CommandContext(context.Background(), "bash", "-c", cmd)
	run.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CAPTURE_ARGS="+capturePath,
	)
	out, err := run.CombinedOutput()
	require.NoError(t, err, "command should not fail from shell substitution:\n%s", string(out))

	argsDump, err := os.ReadFile(capturePath)
	require.NoError(t, err)
	require.Contains(t, string(argsDump), "create table payment_methods(")
	require.Contains(t, string(argsDump), "```")
}
