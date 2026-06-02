package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionShellFallsBackToHostShellForNonContainerSession(t *testing.T) {
	t.Parallel()
	dock := &docker.Mock{Running: true}
	h := testutils.NewHarness(t, testutils.WithDocker(dock))

	remoteURL := testutils.InitTestRemote(t)
	h.RunOK(
		"vibe",
		"--name", "feature",
		"--repo-url", remoteURL,
		"--agent-cmd", "echo ",
		"--no-container",
		"prompt",
	)

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	workspacePath := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, "feature")
	require.DirExists(t, workspacePath)
	sessionName := session.SessionNameFromWorkspaceName(workspacePath)

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "shell.sh")
	markerPath := filepath.Join(scriptDir, "pwd.txt")

	require.NoError(t, os.WriteFile(scriptPath, []byte(`#!/bin/sh
set -eu
pwd > "$MARKER"
`), 0o755))

	h.SetEnv("SHELL", scriptPath)
	h.SetEnv("MARKER", markerPath)

	h.RunOK("session", "shell", "--name", sessionName)

	got, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	gotPath := strings.TrimSpace(string(got))
	require.True(t, filepath.IsAbs(gotPath), "expected absolute path from test shell script, got %q", gotPath)
	wantInfo, err := os.Stat(workspacePath)
	require.NoError(t, err)
	//nolint:gosec // G703: gotPath comes from our test-controlled shell script output.
	gotInfo, err := os.Stat(gotPath)
	require.NoError(t, err)
	require.True(t, wantInfo.IsDir())
	require.True(t, gotInfo.IsDir())
	require.True(t, os.SameFile(wantInfo, gotInfo), "expected %q and %q to refer to the same directory", workspacePath, gotPath)
	require.Empty(t, dock.Execs)
}
