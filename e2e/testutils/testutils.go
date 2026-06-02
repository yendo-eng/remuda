package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal/util"
)

func RemudaParser(t *testing.T) (*kong.Kong, *cli.CLI) {
	c := cli.CLI{}
	parser, err := kong.New(&c, kong.Name("remuda"), kong.Bind(&cli.Context{}))
	require.NoError(t, err)
	return parser, &c
}

func RunGit(t *testing.T, dir string, args ...string) string {
	return RunGitWithOverrides(t, dir, nil, args...)
}

func RunGitWithOverrides(t *testing.T, dir string, overrides map[string]string, args ...string) string {
	return RunGitWithEnv(t, ProcessEnvMap(), dir, overrides, args...)
}

func RunGitWithEnv(t *testing.T, baseEnv map[string]string, dir string, overrides map[string]string, args ...string) string {
	t.Helper()
	t.Logf("git %s (dir %s)", strings.Join(args, " "), dir)
	cmd := util.Cmd("git", args...)
	require.NoError(t, ApplyE2EEnvIsolationToCmd(cmd, baseEnv, overrides))
	cmd.Dir = dir
	output := mustRun(t, cmd)
	if output != "" {
		t.Logf("git output:\n%s", output)
	}
	return output
}

func mustRun(t *testing.T, cmd *exec.Cmd) string {
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	require.NoError(t, err, "command %v failed", cmd.Args)
	return buf.String()
}

// InitTestRemote initialises a bare git repo and returns its file:// URL.
// If modify callback is provided, it is invoked with a working clone path before final push.
func InitTestRemote(t *testing.T) string {
	t.Helper()

	temp := t.TempDir()
	remotePath := filepath.Join(temp, "remote.git")
	RunGit(t, temp, "init", "--bare", remotePath)

	workDir := filepath.Join(temp, "work")
	RunGit(t, temp, "clone", remotePath, workDir)

	// Configure git user for commits.
	RunGit(t, workDir, "config", "user.email", "test@example.com")
	RunGit(t, workDir, "config", "user.name", "Test User")
	RunGit(t, workDir, "config", "commit.gpgsign", "false")

	require.NoError(t, os.WriteFile(filepath.Join(workDir, "README.md"), []byte("hello"), 0o644))
	RunGit(t, workDir, "add", "README.md")
	RunGit(t, workDir, "commit", "-m", "initial commit")
	// Ensure branch name is main for consistency.
	RunGit(t, workDir, "branch", "-M", "main")
	RunGit(t, workDir, "push", "-u", "origin", "main")

	// Ensure bare repo's HEAD points to main to avoid git pull errors on master.
	RunGit(t, ".", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	// Return absolute path + .git suffix already included.
	return remotePath
}

func RequireDirExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.NoError(t, err, "expected directory %s to exist", path)
}

func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}

	return val
}
