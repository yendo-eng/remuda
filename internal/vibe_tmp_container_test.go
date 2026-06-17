package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
)

// When a --tmp worktree (under the OS-temp root) runs in a container, the git
// cache mount must point at the REAL persistent cache under ReposBaseDir, not a
// non-existent sibling of the temp worktree.
func TestComposeLaunchCommand_TmpWorktreeMountsRealCache(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	reposBase := filepath.Join(root, "repos")
	tmpBase := filepath.Join(root, "tmp-repos")

	require.NoError(t, os.MkdirAll(home, 0o755))

	t.Setenv("HOME", home)
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOCACHE", filepath.Join(home, "gocache"))
	t.Setenv("GOMODCACHE", filepath.Join(home, "gomodcache"))

	// Persistent cache under ReposBaseDir; temp worktree under TmpBaseDir.
	cacheDir := filepath.Join(reposBase, "org", "repo", ".repo_cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	tmpWorkspace := filepath.Join(tmpBase, "org", "repo", "wk")
	require.NoError(t, os.MkdirAll(tmpWorkspace, 0o755))

	k := Remuda{
		Config: Config{ReposBaseDir: reposBase, TmpBaseDir: tmpBase},
		Docker: &docker.Mock{Running: true},
	}

	cmd := VibeCommand{
		Container:     true,
		ContainerName: "vibe-dev",
	}

	launchCmd, _, err := k.composeLaunchCommand(cmd, tmpWorkspace, "echo hi", "org/repo/wk", "cont", k.envProvider())
	require.NoError(t, err)

	// The mount target is the persistent cache path, mounted at the identical
	// absolute path inside the container.
	require.Contains(t, launchCmd, fmt.Sprintf("-v %q:%q", cacheDir, cacheDir))
	// It must NOT try to mount a (non-existent) cache beside the temp worktree.
	require.NotContains(t, launchCmd, filepath.Join(tmpBase, "org", "repo", ".repo_cache"))
}

func TestVibeDetachedTmpContainerPreflightFailsBeforeSessionStart(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	reposBase := filepath.Join(root, "repos")
	tmpBase := filepath.Join(root, "tmp-repos")
	binDir := filepath.Join(root, "bin")
	tmpWorkspace := filepath.Join(tmpBase, "org", "repo", "wk")
	fakeDocker := filepath.Join(binDir, "docker")
	script := `#!/usr/bin/env bash
echo "docker: Error response from daemon: Mounts denied: The path is not shared from the host and is not known to Docker." >&2
exit 1
`

	require.NoError(t, os.MkdirAll(home, 0o755))
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.MkdirAll(tmpWorkspace, 0o755))
	require.NoError(t, os.WriteFile(fakeDocker, []byte(script), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: reposBase, TmpBaseDir: tmpBase},
		Session: mgr,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
		Env: env.StaticProvider{
			Values: map[string]string{
				"PATH":     binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"GH_TOKEN": "test-token",
			},
			HomeDir: home,
		},
	}

	err := k.Vibe(context.Background(), VibeCommand{
		ExistingWorkspace: tmpWorkspace,
		Agent:             "bash",
		AgentCmd:          "echo should-not-run",
		Detached:          true,
		Container:         true,
		ContainerName:     "vibe-dev",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "REMUDA_TMP_DIR")
	require.ErrorContains(t, err, "File Sharing")
	require.Empty(t, mgr.started)
}
