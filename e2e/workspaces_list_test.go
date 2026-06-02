package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestWorkspacesList(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*testutils.Harness, string, string, string, string) {
		t.Helper()

		h := testutils.NewHarness(t)
		h.SetEnv("REMUDA_CONTAINER", "false")

		remoteURL := testutils.InitTestRemote(t)
		org, repo, _ := github.ParseRepo(remoteURL)
		baseDir := h.RemudaConfig.ReposBaseDir

		return h, remoteURL, baseDir, org, repo
	}

	t.Run("lists active and inactive remuda workspaces", func(t *testing.T) {
		h, remoteURL, baseDir, org, repo := setup(t)

		activeName := "active-workspace"
		inactiveName := "inactive-workspace"
		activePath := filepath.Join(baseDir, org, repo, activeName)
		inactivePath := filepath.Join(baseDir, org, repo, inactiveName)
		inactiveSessionName := session.SessionNameFromWorkspaceName(inactivePath)

		h.RunOK("vibe", "--name", activeName, "--repo-url", remoteURL)
		h.RunOK("vibe", "--name", inactiveName, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", inactiveSessionName)

		res := h.RunOK("workspaces", "list")
		lines := nonEmptyOutputLines(res.Stdout)

		require.ElementsMatch(t, []string{activePath, inactivePath}, lines)
		require.NotContains(t, res.Stdout, ".repo_cache")
	})

	t.Run("inactive flag lists only inactive workspaces", func(t *testing.T) {
		h, remoteURL, baseDir, org, repo := setup(t)

		activeName := "active-workspace"
		inactiveName := "inactive-workspace"
		activePath := filepath.Join(baseDir, org, repo, activeName)
		inactivePath := filepath.Join(baseDir, org, repo, inactiveName)
		inactiveSessionName := session.SessionNameFromWorkspaceName(inactivePath)

		h.RunOK("vibe", "--name", activeName, "--repo-url", remoteURL)
		h.RunOK("vibe", "--name", inactiveName, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", inactiveSessionName)

		res := h.RunOK("workspaces", "list", "--inactive")
		require.Equal(t, []string{inactivePath}, nonEmptyOutputLines(res.Stdout))
		require.NotContains(t, res.Stdout, activePath)
	})

	t.Run("active flag lists only active workspaces", func(t *testing.T) {
		h, remoteURL, baseDir, org, repo := setup(t)

		activeName := "active-workspace"
		inactiveName := "inactive-workspace"
		activePath := filepath.Join(baseDir, org, repo, activeName)
		inactivePath := filepath.Join(baseDir, org, repo, inactiveName)
		inactiveSessionName := session.SessionNameFromWorkspaceName(inactivePath)

		h.RunOK("vibe", "--name", activeName, "--repo-url", remoteURL)
		h.RunOK("vibe", "--name", inactiveName, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", inactiveSessionName)

		res := h.RunOK("workspaces", "list", "--active")
		require.Equal(t, []string{activePath}, nonEmptyOutputLines(res.Stdout))
		require.NotContains(t, res.Stdout, inactivePath)
	})

	t.Run("configured workspaces ignore excludes matching workspaces", func(t *testing.T) {
		h, remoteURL, baseDir, org, repo := setup(t)

		keepName := "keep-workspace"
		ignoreName := "ignored-workspace"
		keepPath := filepath.Join(baseDir, org, repo, keepName)
		ignorePath := filepath.Join(baseDir, org, repo, ignoreName)
		keepSessionName := session.SessionNameFromWorkspaceName(keepPath)
		ignoreSessionName := session.SessionNameFromWorkspaceName(ignorePath)

		h.RunOK("vibe", "--name", keepName, "--repo-url", remoteURL)
		h.RunOK("vibe", "--name", ignoreName, "--repo-url", remoteURL)
		h.RunOK("session", "kill", "--name", keepSessionName)
		h.RunOK("session", "kill", "--name", ignoreSessionName)

		configPath := writeTempConfigFile(t, fmt.Sprintf(`
version: 1
workspaces:
  ignore:
    - %q
`, path.Join(org, repo, ignoreName)))
		h.SetEnv("REMUDA_CONFIG", configPath)

		res := h.RunOK("workspaces", "list")
		require.Equal(t, []string{keepPath}, nonEmptyOutputLines(res.Stdout))
		require.NotContains(t, res.Stdout, ignorePath)

		resInactive := h.RunOK("workspaces", "list", "--inactive")
		require.Equal(t, []string{keepPath}, nonEmptyOutputLines(resInactive.Stdout))
		require.NotContains(t, resInactive.Stdout, ignorePath)
	})

	t.Run("includes full clone workspaces", func(t *testing.T) {
		h, remoteURL, baseDir, org, repo := setup(t)

		name := "full-clone-workspace"
		fullClonePath := filepath.Join(baseDir, org, repo, name)

		h.RunOK("clone", "--name", name, "--full-clone", "--repo-url", remoteURL)

		res := h.RunOK("workspaces", "list")
		require.Contains(t, nonEmptyOutputLines(res.Stdout), fullClonePath)
	})

	t.Run("active and inactive flags are an invalid combination", func(t *testing.T) {
		h, _, _, _, _ := setup(t)

		goModCache := filepath.Join(h.RootDir, "go-mod-cache")
		goBuildCache := filepath.Join(h.RootDir, "go-build-cache")
		require.NoError(t, os.MkdirAll(goModCache, 0o755))
		require.NoError(t, os.MkdirAll(goBuildCache, 0o755))

		cmd := exec.CommandContext(
			t.Context(),
			"go",
			"run",
			"./cmd/remuda",
			"workspaces",
			"list",
			"--active",
			"--inactive",
		)
		cmd.Dir = ".."
		require.NoError(t, testutils.ApplyE2EEnvIsolationToCmd(
			cmd,
			map[string]string(h.Env),
			map[string]string{
				"GOFLAGS":    "-modcacherw",
				"GOMODCACHE": goModCache,
				"GOCACHE":    goBuildCache,
			},
		))

		out, err := cmd.CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(out), "flags --active and --inactive cannot be used together")
	})
}

func writeTempConfigFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func nonEmptyOutputLines(out string) []string {
	lines := strings.Split(out, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}
