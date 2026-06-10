package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	repoRoot := repoRootDir(t)
	baseEnv := testutils.ProcessEnvMap()

	t.Run("local source build reports a non-empty version", func(t *testing.T) {
		t.Parallel()

		binPath := filepath.Join(t.TempDir(), "remuda-local")
		buildRemudaBinary(t, repoRoot, binPath, baseEnv, false)

		version := runRemudaVersion(t, repoRoot, binPath, baseEnv)
		require.NotEmpty(t, version)
	})

	t.Run("ldflags stamped build overrides build info", func(t *testing.T) {
		t.Parallel()

		binPath := filepath.Join(t.TempDir(), "remuda-stamped")
		buildRemudaBinary(t, repoRoot, binPath, baseEnv, true)

		version := runRemudaVersion(t, repoRoot, binPath, baseEnv)
		require.Equal(t, "v0.1.0-test", version)
	})
}

func repoRootDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	return filepath.Clean(filepath.Join(wd, ".."))
}

func buildRemudaBinary(t *testing.T, repoRoot, outputPath string, baseEnv map[string]string, stamped bool) {
	t.Helper()

	var cmd *exec.Cmd
	if stamped {
		//nolint:gosec // Test intentionally invokes local `go build` against fixed args to produce a test binary.
		cmd = exec.CommandContext(
			t.Context(),
			"go",
			"build",
			"-o",
			outputPath,
			"-buildvcs=false",
			"-ldflags=-X main.buildVersion=v0.1.0-test",
			"./cmd/remuda",
		)
	} else {
		//nolint:gosec // Test intentionally invokes local `go build` against fixed args to produce a test binary.
		cmd = exec.CommandContext(
			t.Context(),
			"go",
			"build",
			"-o",
			outputPath,
			"./cmd/remuda",
		)
	}
	cmd.Dir = repoRoot
	require.NoError(t, testutils.ApplyE2EEnvIsolationToCmd(cmd, baseEnv, nil))

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func runRemudaVersion(t *testing.T, repoRoot, binPath string, baseEnv map[string]string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), binPath, "--version")
	cmd.Dir = repoRoot
	require.NoError(t, testutils.ApplyE2EEnvIsolationToCmd(cmd, baseEnv, nil))

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	return strings.TrimSpace(string(output))
}
