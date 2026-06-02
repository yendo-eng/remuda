package e2e_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/jira"
)

func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	var runErr error
	func() {
		defer func() {
			_ = w.Close()
			os.Stderr = orig
		}()
		runErr = fn()
	}()

	out, readErr := io.ReadAll(r)
	_ = r.Close()
	require.NoError(t, readErr)

	return string(out), runErr
}

func TestShellCommandOutputHiddenUnlessVerbose(t *testing.T) {
	remoteURL := testutils.InitTestRemote(t)

	t.Run("default (quiet)", func(t *testing.T) {
		runDir := t.TempDir()
		k := internal.NewRemuda(
			internal.Config{ReposBaseDir: runDir},
			git.NewShellGit(),
			&testutils.MockSessionManager{},
			jira.Mock{},
			nil,
			nil,
		)
		kctx := cli.NewContext(t.Context(), k, cli.Stdout(&bytes.Buffer{}), cli.Stderr(&bytes.Buffer{}))

		stderr, err := captureStderr(t, func() error {
			return cli.Run(kctx, []string{"clone", "--name", "wk", "--repo-url", remoteURL})
		})
		require.NoError(t, err)
		require.NotContains(t, stderr, "Cloning into", "expected git output to be suppressed without -v")
		require.Equal(t, "", stderr, "expected no direct stderr writes without -v")
	})

	t.Run("with -v", func(t *testing.T) {
		runDir := t.TempDir()
		k := internal.NewRemuda(
			internal.Config{ReposBaseDir: runDir},
			git.NewShellGit(),
			&testutils.MockSessionManager{},
			jira.Mock{},
			nil,
			nil,
		)
		kctx := cli.NewContext(t.Context(), k, cli.Stdout(&bytes.Buffer{}), cli.Stderr(&bytes.Buffer{}))

		stderr, err := captureStderr(t, func() error {
			return cli.Run(kctx, []string{"-v", "clone", "--name", "wk2", "--repo-url", remoteURL})
		})
		require.NoError(t, err)
		require.Contains(t, stderr, "Cloning into", "expected git output to be streamed with -v")
	})
}
