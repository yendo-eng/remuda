package docker_test

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

// runBuiltCommand executes cmd (as built by docker.BuildRunCommand) through
// bash, capturing the argv a real `docker` binary would receive.
func runBuiltCommand(t *testing.T, cmd string) []string {
	t.Helper()

	binDir := t.TempDir()
	capturePath := filepath.Join(binDir, "docker-args.txt")
	dockerPath := filepath.Join(binDir, "docker")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" > \"$CAPTURE_ARGS\"\n"
	require.NoError(t, os.WriteFile(dockerPath, []byte(script), 0o755))

	//nolint:gosec // G204: this test intentionally executes the generated shell command to verify quoting behavior.
	run := exec.CommandContext(context.Background(), "bash", "-c", cmd)
	run.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"CAPTURE_ARGS="+capturePath,
	)
	out, err := run.CombinedOutput()
	require.NoError(t, err, "command should not fail:\n%s", string(out))

	argsDump, err := os.ReadFile(capturePath)
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(argsDump), "\n"), "\n")
}

func TestBuildRunCommand_ComposesExpectedArgv(t *testing.T) {
	ws := "/abs/path with spaces/repos/org/repo/repo_1"
	img := "vibe-dev"
	opts := []string{"--gpus", "all", "--network", "host"}
	agent := "codex 'Do the thing'"
	containerName := "session-container"

	cmd := docker.BuildRunCommand(ws, img, opts, agent, false, containerName)
	containerWS := docker.ContainerWorkspacePath(ws)

	argv := runBuiltCommand(t, cmd)

	require.Equal(t, []string{
		"run", "--rm", "-it",
		"--name", containerName,
		"-v", ws + ":" + containerWS,
		"-w", containerWS,
		"-e", "OPENAI_API_KEY",
		"-e", "REMUDA_AGENT",
		"-e", "REMUDA_MODEL",
		"-e", "GH_TOKEN",
		"-e", "GITHUB_TOKEN",
		"-e", "GIT_HTTPS_USERNAME",
		"-e", "GIT_TERMINAL_PROMPT=0",
		"-e", "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new",
		"--gpus", "all",
		"--network", "host",
		img,
	}, argv[:len(argv)-3], "docker argv up to the bash -lc invocation")

	require.Equal(t, []string{"bash", "-lc"}, argv[len(argv)-3:len(argv)-1])

	innerScript := argv[len(argv)-1]
	require.Contains(t, innerScript, "credential.helper")
	require.Contains(t, innerScript, "store --file=/root/.git-credentials")
	require.Contains(t, innerScript, "gh auth setup-git -h github.com")
	require.Contains(t, innerScript, `git config --global url."git@github.com:".insteadOf "https://github.com/"`)
	require.Contains(t, innerScript, agent)
}

func TestBuildRunCommand_OmitsUnsetContainerName(t *testing.T) {
	ws := "/abs/path/repos/org/repo/repo_1"
	img := "vibe-dev"
	opts := []string{"--network", "host"}
	agent := "codex"

	cmd := docker.BuildRunCommand(ws, img, opts, agent, false, "")
	argv := runBuiltCommand(t, cmd)

	require.NotContains(t, argv, "--name")
	require.Contains(t, strings.Join(argv, " "), "--network host")
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
