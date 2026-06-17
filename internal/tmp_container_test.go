package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerMountDeniedHint(t *testing.T) {
	tmpBase := filepath.FromSlash("/var/folders/xy/remuda")
	tmpWS := filepath.Join(tmpBase, "org", "repo", "wk")
	reposWS := filepath.FromSlash("/Users/dev/.remuda/repos/org/repo/wk")

	deniedOutput := `docker: Error response from daemon: Mounts denied: ` +
		`The path /var/folders/xy/remuda/org/repo/wk is not shared from the host and is not known to Docker.`

	t.Run("tmp worktree mount-denied points at REMUDA_TMP_DIR", func(t *testing.T) {
		hint := dockerMountDeniedHint(deniedOutput, tmpWS, tmpBase)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "REMUDA_TMP_DIR")
		require.Contains(t, hint, "File Sharing")
	})

	t.Run("non-tmp workspace mount-denied omits REMUDA_TMP_DIR", func(t *testing.T) {
		hint := dockerMountDeniedHint(deniedOutput, reposWS, tmpBase)
		require.NotEmpty(t, hint)
		require.NotContains(t, hint, "REMUDA_TMP_DIR")
		require.Contains(t, hint, "File Sharing")
	})

	t.Run("unrelated docker error yields no hint", func(t *testing.T) {
		out := "docker: Error response from daemon: pull access denied for someimage"
		require.Empty(t, dockerMountDeniedHint(out, tmpWS, tmpBase))
	})

	t.Run("empty output yields no hint", func(t *testing.T) {
		require.Empty(t, dockerMountDeniedHint("", tmpWS, tmpBase))
	})
}

func TestTailBuffer_RetainsOnlyLastBytes(t *testing.T) {
	b := newTailBuffer(8)
	_, _ = b.Write([]byte("123456"))
	_, _ = b.Write([]byte("7890"))
	require.Equal(t, "34567890", b.String())

	// A single write larger than max keeps only the last max bytes.
	b2 := newTailBuffer(4)
	_, _ = b2.Write([]byte("abcdefgh"))
	require.Equal(t, "efgh", b2.String())
}

func TestRunDetachedContainerPreflightTranslatesMountDenied(t *testing.T) {
	tmpBase := filepath.FromSlash("/var/folders/xy/remuda")
	tmpWS := filepath.Join(tmpBase, "org", "repo", "wk")
	deniedOutput := `docker: Error response from daemon: Mounts denied: ` +
		`The path /var/folders/xy/remuda/org/repo/wk is not shared from the host and is not known to Docker.`

	k := Remuda{Config: Config{TmpBaseDir: tmpBase}}
	err := k.runDetachedContainerPreflight(
		"printf '%s\n' "+shellSingleQuote(deniedOutput)+" >&2; exit 1",
		tmpWS,
		nil,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "REMUDA_TMP_DIR")
	require.ErrorContains(t, err, "File Sharing")
}

func TestRunDetachedContainerPreflightSuccess(t *testing.T) {
	k := Remuda{}
	err := k.runDetachedContainerPreflight("true", t.TempDir(), nil)
	require.NoError(t, err)
}
