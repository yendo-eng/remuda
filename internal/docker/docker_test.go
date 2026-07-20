package docker_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
)

func TestBuildContainerAuthOpts_ProducesMountsWhenAvailable(t *testing.T) {
	tmp := t.TempDir()

	// Simulate HOME with gh config, gitconfig, and .ssh
	ghDir := filepath.Join(tmp, ".config", "gh")
	require.NoError(t, os.MkdirAll(ghDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte("test: token"), 0o600))

	gitconfig := filepath.Join(tmp, ".gitconfig")
	require.NoError(t, os.WriteFile(gitconfig, []byte("[user]\nname = test\n"), 0o644))

	sshDir := filepath.Join(tmp, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte("github.com ssh-rsa AAAA"), 0o600))

	// Mock SSH agent socket path (used on non-darwin)
	sock := filepath.Join(tmp, "agent.sock")
	require.NoError(t, os.WriteFile(sock, []byte{}, 0o600))

	// Override HOME and SSH_AUTH_SOCK for this test
	oldHome := os.Getenv("HOME")
	oldSock := os.Getenv("SSH_AUTH_SOCK")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("SSH_AUTH_SOCK", oldSock)
	})
	require.NoError(t, os.Setenv("HOME", tmp))
	require.NoError(t, os.Setenv("SSH_AUTH_SOCK", sock))

	opts := docker.BuildContainerAuthOpts()

	// On Windows, docker path quoting/volume semantics differ; our CLI targets macOS/Linux.
	if runtime.GOOS == "windows" {
		t.Skip("Windows path semantics not covered")
	}

	require.Contains(t, opts, ghDir+":/root/.config/gh:ro")
	// We intentionally do NOT mount host ~/.gitconfig to keep container writable.
	require.Contains(t, opts, sshDir+":/root/.ssh:ro")
	// Darwin uses Docker Desktop magic socket path; others use SSH_AUTH_SOCK
	if runtime.GOOS == "darwin" {
		require.Contains(t, opts, "/run/host-services/ssh-auth.sock:/ssh-agent")
	} else {
		require.Contains(t, opts, sock+":/ssh-agent")
	}
	require.Contains(t, opts, "SSH_AUTH_SOCK=/ssh-agent")
}

func TestExtraGitMountForWorktree_ReturnsMountForCacheDir(t *testing.T) {
	tmp := t.TempDir()

	// Simulate repos/<org>/<repo> structure
	baseDir := filepath.Join(tmp, "repos", "org", "repo")
	workspace := filepath.Join(baseDir, "feature_branch")
	cacheDir := filepath.Join(baseDir, ".repo_cache")

	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))

	// Absolute path expected by helper
	absWS, err := filepath.Abs(workspace)
	require.NoError(t, err)

	mount, ok := docker.ExtraGitMountForWorktree(absWS)
	require.True(t, ok, "expected an extra mount when cache dir exists")

	// It should mount the cache dir to the identical path inside the container
	absCache, err := filepath.Abs(cacheDir)
	require.NoError(t, err)
	require.Equal(t, absCache+":"+absCache, mount)
}

func TestExtraGitMountForWorktree_NoCache_NoMount(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "repos", "org", "repo")
	workspace := filepath.Join(baseDir, "feature_branch")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	absWS, err := filepath.Abs(workspace)
	require.NoError(t, err)

	mount, ok := docker.ExtraGitMountForWorktree(absWS)
	require.False(t, ok)
	require.Equal(t, "", mount)
}

func TestContainerNameFromSession(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"acme/widgets/feature_task-1", "acme-widgets-feature_task-1"},
		{"solo", "solo"},
		{"Leading/Spaces /And@Invalid", "Leading-Spaces-And-Invalid"},
		{"123/abc", "123-abc"},
		// Dots are converted to underscores to match tmux's behavior (tmux converts dots
		// to underscores in session names, so we must do the same to ensure the container
		// name derived at launch matches the name derived later from the tmux session).
		{"acme/remuda/5.4-per-repo-overrides", "acme-remuda-5_4-per-repo-overrides"},
		{"org/repo/v1.2.3-feature", "org-repo-v1_2_3-feature"},
	}
	for _, c := range cases {
		got := docker.ContainerNameFromSession(c.name)
		require.Equal(t, c.want, got, "ContainerNameFromSession(%q) = %q; want %q", c.name, got, c.want)
	}
}

func TestBuildGoCacheMountOpts_UsesGoEnv(t *testing.T) {
	tmp := t.TempDir()
	hostCache := filepath.Join(tmp, "cache")
	hostMod := filepath.Join(tmp, "mod")
	// ensure directories do not exist before helper runs to exercise MkdirAll path
	require.NoError(t, os.RemoveAll(hostCache))
	require.NoError(t, os.RemoveAll(hostMod))

	oldCache := os.Getenv("GOCACHE")
	oldMod := os.Getenv("GOMODCACHE")
	t.Cleanup(func() {
		_ = os.Setenv("GOCACHE", oldCache)
		_ = os.Setenv("GOMODCACHE", oldMod)
	})

	require.NoError(t, os.Setenv("GOCACHE", hostCache))
	require.NoError(t, os.Setenv("GOMODCACHE", hostMod))

	opts := docker.BuildGoCacheMountOpts()
	require.Contains(t, opts, hostCache+":/root/.cache/go-build")
	require.Contains(t, opts, hostMod+":/go/pkg/mod")

	// helper should create directories if they were missing
	_, err := os.Stat(hostCache)
	require.NoError(t, err)
	_, err = os.Stat(hostMod)
	require.NoError(t, err)
}
