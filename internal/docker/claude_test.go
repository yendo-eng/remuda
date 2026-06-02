package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
)

func TestBuildClaudeStateMountOpts_BothPathsPresent(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	claudeJSON := filepath.Join(tmp, ".claude.json")

	require.NoError(t, os.MkdirAll(claudeDir, 0o700))
	require.NoError(t, os.WriteFile(claudeJSON, []byte(`{"token":"redacted"}`), 0o600))

	opts := buildClaudeStateMountOpts(logging.NewDisabledLogger(), tmp)
	require.Equal(
		t,
		[]string{
			fmt.Sprintf("-v %q:%q:rw", claudeDir, "/root/.claude"),
			fmt.Sprintf("-v %q:%q:rw", claudeJSON, "/root/.claude.json"),
		},
		opts,
	)
}

func TestBuildClaudeStateMountOptsWithProvider_Permutations(t *testing.T) {
	tcs := []struct {
		name      string
		withDir   bool
		withFile  bool
		expectNil bool
	}{
		{name: "both", withDir: true, withFile: true},
		{name: "dir only", withDir: true},
		{name: "file only", withFile: true},
		{name: "missing", expectNil: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			claudeDir := filepath.Join(tmp, ".claude")
			claudeJSON := filepath.Join(tmp, ".claude.json")

			if tc.withDir {
				require.NoError(t, os.MkdirAll(claudeDir, 0o700))
			}
			if tc.withFile {
				require.NoError(t, os.WriteFile(claudeJSON, []byte(`{"token":"redacted"}`), 0o600))
			}

			opts := BuildClaudeStateMountOptsWithProvider(env.StaticProvider{HomeDir: tmp})
			if tc.expectNil {
				require.Nil(t, opts)
				return
			}

			var want []string
			if tc.withDir {
				want = append(want, fmt.Sprintf("-v %q:%q:rw", claudeDir, "/root/.claude"))
			}
			if tc.withFile {
				want = append(want, fmt.Sprintf("-v %q:%q:rw", claudeJSON, "/root/.claude.json"))
			}
			require.Equal(t, want, opts)
		})
	}
}

func TestBuildClaudeStateMountOptsWithProvider_IgnoresUnexpectedNodeTypes(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".claude"), []byte("not-a-dir"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".claude.json"), 0o700))

	opts := BuildClaudeStateMountOptsWithProvider(env.StaticProvider{HomeDir: tmp})
	require.Nil(t, opts)
}

func TestBuildClaudeStateMountOptsWithProvider_BlankHome(t *testing.T) {
	opts := BuildClaudeStateMountOptsWithProvider(env.StaticProvider{HomeDir: "   "})
	require.Nil(t, opts)
}

func TestBuildClaudeStateMountOptsWithProvider_HomeUnavailable(t *testing.T) {
	provider := env.StaticProvider{HomeErr: errors.New("no home")}
	opts := BuildClaudeStateMountOptsWithProvider(provider)
	require.Nil(t, opts)
}
