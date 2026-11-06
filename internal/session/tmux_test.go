package session_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util/shell"
)

// writeStubTmux creates a stub tmux that simulates various behaviors for list-sessions.
func writeStubTmux(t *testing.T, dir string, stderr string, exitCode int) string {
	t.Helper()
	name := "tmux"
	if runtime.GOOS == "windows" {
		name = "tmux.bat"
	}
	path := filepath.Join(dir, name)
	script := "" +
		"#!/bin/sh\n" +
		"cmd=\"$1\"\n" +
		"if [ \"$cmd\" = \"list-sessions\" ]; then\n" +
		"  # Simulate tmux server absent\n" +
		"  >&2 echo \"" + stderr + "\"\n" +
		"  exit " + func() string {
		if exitCode == 0 {
			return "0"
		}
		return "1"
	}() + "\n" +
		"else\n" +
		"  echo \"ok\"\n" +
		"fi\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func TestTmuxListHandlesNoServerMessage(t *testing.T) {
	tmp := t.TempDir()
	// Typical tmux message on macOS/Linux when no server is running.
	_ = writeStubTmux(t, tmp, "no server running on /tmp/tmux-123/default", 1)
	// Prepend stub to PATH
	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)

	got, err := session.NewTmux(logging.DefaultLogger()).List()
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestTmuxListHandlesExitCode1WithoutCanonicalMessage(t *testing.T) {
	tmp := t.TempDir()
	// Some tmux builds output different wording.
	_ = writeStubTmux(t, tmp, "failed to connect to server", 1)
	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)

	got, err := session.NewTmux(logging.DefaultLogger()).List()
	// We expect no error and an empty list even if the wording differs.
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestTmuxListParsesSpaceDelimitedFormat(t *testing.T) {
	tmp := t.TempDir()

	name := "tmux"
	if runtime.GOOS == "windows" {
		name = "tmux.bat"
	}
	path := filepath.Join(tmp, name)
	script := "" +
		"#!/bin/sh\n" +
		"cmd=\"$1\"\n" +
		"if [ \"$cmd\" = \"list-sessions\" ]; then\n" +
		"  # The production format is: \"#{session_name} #{?session_attached,1,0} #{session_created}\"\n" +
		"  printf '%s\\n' 'org/repo/work 1 1710000000'\n" +
		"  printf '%s\\n' 'other session 0 1710000123'\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"ok\"\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)

	got, err := session.NewTmux(logging.DefaultLogger()).List()
	require.NoError(t, err)
	require.Equal(t, []session.SessionInfo{
		{Name: "org/repo/work", Attached: true, CreatedAt: time.Unix(1710000000, 0).UTC(), Multiplexer: "tmux"},
		{Name: "other session", Attached: false, CreatedAt: time.Unix(1710000123, 0).UTC(), Multiplexer: "tmux"},
	}, got)
}

func TestTmuxListParsesMissingOrMalformedCreated(t *testing.T) {
	tmp := t.TempDir()

	name := "tmux"
	if runtime.GOOS == "windows" {
		name = "tmux.bat"
	}
	path := filepath.Join(tmp, name)
	script := "" +
		"#!/bin/sh\n" +
		"cmd=\"$1\"\n" +
		"if [ \"$cmd\" = \"list-sessions\" ]; then\n" +
		"  printf '%s\\n' 'org/repo/work 1 not_a_number'\n" +
		"  printf '%s\\n' 'other 0'\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"ok\"\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)

	got, err := session.NewTmux(logging.DefaultLogger()).List()
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "org/repo/work", got[0].Name)
	require.True(t, got[0].Attached)
	require.True(t, got[0].CreatedAt.IsZero())
	require.Equal(t, "other", got[1].Name)
	require.False(t, got[1].Attached)
	require.True(t, got[1].CreatedAt.IsZero())
}

func TestTmuxStartWithEnvSetsPaneEnvWithExistingServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires tmux")
	}

	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	tmp := t.TempDir()
	tmuxEnvDir := filepath.Join(tmp, "tmux env'files")
	require.NoError(t, os.MkdirAll(tmuxEnvDir, 0o755))
	t.Setenv("TMPDIR", tmuxEnvDir)
	socketName := fmt.Sprintf("remuda-test-%d", time.Now().UnixNano())
	wrapperPath := filepath.Join(tmp, "tmux")
	wrapper := "#!/bin/sh\nexec \"$REMUDA_REAL_TMUX\" -L \"$REMUDA_TMUX_SOCKET\" \"$@\"\n"
	require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0o755))

	pathEnv := tmp + string(os.PathListSeparator) + os.Getenv("PATH")
	baseEnv := filteredEnvWithout("PATH", "TMUX", "TMUX_PANE", "TERM")
	baseEnv = append(baseEnv,
		"PATH="+pathEnv,
		"REMUDA_REAL_TMUX="+realTmux,
		"REMUDA_TMUX_SOCKET="+socketName,
	)
	seedEnv := append(append([]string{}, baseEnv...), "STALE_SECRET=stale-value")

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		//nolint:gosec,nolintlint // G204: wrapperPath is a test-owned temp wrapper script written by this test, not external input.
		cmd := exec.CommandContext(ctx, wrapperPath, "kill-server")
		cmd.Env = baseEnv
		_ = cmd.Run()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	//nolint:gosec // G204: wrapperPath is a test-owned temp wrapper script written by this test, not external input.
	cmd := exec.CommandContext(ctx, wrapperPath, "new-session", "-d", "-s", "seed", "sleep 30")
	cmd.Env = seedEnv
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	outputPath := filepath.Join(tmp, "pane-env")
	startEnv := append([]string{}, baseEnv...)
	startEnv = append(startEnv, "REMUDA_TEST_PANE_ENV=tmux-secret value")

	mgr := session.NewTmux(logging.DefaultLogger())
	starter, ok := mgr.(session.EnvStarter)
	require.True(t, ok)
	err = starter.StartWithEnv(
		"env-check",
		"printf '%s|%s|%s|%s|%s' \"$REMUDA_TEST_PANE_ENV\" \"${STALE_SECRET-unset}\" \"${TERM:+set}\" \"${TMUX:+set}\" \"${TMUX_PANE:+set}\" > "+shell.SingleQuote(outputPath),
		startEnv,
	)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(outputPath)
		return readErr == nil && string(data) == "tmux-secret value|unset|set|set|set"
	}, 2*time.Second, 50*time.Millisecond)
}

func TestTmuxStartWithEnvSurfacesStderrOnDuplicateSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires tmux")
	}

	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	tmp := t.TempDir()
	socketName := fmt.Sprintf("remuda-test-%d", time.Now().UnixNano())
	wrapperPath := filepath.Join(tmp, "tmux")
	wrapper := "#!/bin/sh\nexec \"$REMUDA_REAL_TMUX\" -L \"$REMUDA_TMUX_SOCKET\" \"$@\"\n"
	require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0o755))

	pathEnv := tmp + string(os.PathListSeparator) + os.Getenv("PATH")
	baseEnv := filteredEnvWithout("PATH", "TMUX", "TMUX_PANE")
	baseEnv = append(baseEnv,
		"PATH="+pathEnv,
		"REMUDA_REAL_TMUX="+realTmux,
		"REMUDA_TMUX_SOCKET="+socketName,
	)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		//nolint:gosec,nolintlint // G204: wrapperPath is a test-owned temp wrapper script written by this test, not external input.
		cmd := exec.CommandContext(ctx, wrapperPath, "kill-server")
		cmd.Env = baseEnv
		_ = cmd.Run()
	}()

	mgr := session.NewTmux(logging.DefaultLogger())
	starter, ok := mgr.(session.EnvStarter)
	require.True(t, ok)

	require.NoError(t, starter.StartWithEnv("dup-session", "sleep 30", baseEnv))

	err = starter.StartWithEnv("dup-session", "sleep 30", baseEnv)
	require.Error(t, err)
	require.ErrorContains(t, err, "tmux")
	require.ErrorContains(t, err, "dup-session")
	require.ErrorContains(t, err, "duplicate session")
}

func TestTmuxStartWithEnvKeepsValuesOffArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script stub")
	}

	t.Setenv("TMPDIR", t.TempDir())
	tmp := t.TempDir()
	argsPath := filepath.Join(tmp, "tmux-args")
	tmuxPath := filepath.Join(tmp, "tmux")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$TMUX_ARGS_FILE\"\n"
	require.NoError(t, os.WriteFile(tmuxPath, []byte(script), 0o755))

	pathEnv := tmp + string(os.PathListSeparator) + os.Getenv("PATH")
	secret := "openai-secret-value"
	env := []string{
		"PATH=" + pathEnv,
		"TMUX_ARGS_FILE=" + argsPath,
		"OPENAI_API_KEY=" + secret,
	}

	starter, ok := session.NewTmux(logging.DefaultLogger()).(session.EnvStarter)
	require.True(t, ok)
	require.NoError(t, starter.StartWithEnv("argv-check", "true", env))

	args, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	require.NotContains(t, string(args), secret)
	require.NotContains(t, string(args), "OPENAI_API_KEY="+secret)
}

func TestTmuxStartWithEnvBoundsArgvForLargeEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script stub")
	}

	t.Setenv("TMPDIR", t.TempDir())
	tmp := t.TempDir()
	argsPath := filepath.Join(tmp, "tmux-args")
	tmuxPath := filepath.Join(tmp, "tmux")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$TMUX_ARGS_FILE\"\n"
	require.NoError(t, os.WriteFile(tmuxPath, []byte(script), 0o755))

	pathEnv := tmp + string(os.PathListSeparator) + os.Getenv("PATH")
	env := []string{
		"PATH=" + pathEnv,
		"TMUX_ARGS_FILE=" + argsPath,
	}
	for i := 0; i < 256; i++ {
		env = append(env, fmt.Sprintf("SYNTHETIC_%03d=%s", i, strings.Repeat("x", 1024)))
	}

	starter, ok := session.NewTmux(logging.DefaultLogger()).(session.EnvStarter)
	require.True(t, ok)
	require.NoError(t, starter.StartWithEnv("large-env", "true", env))

	args, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	require.Less(t, len(args), 4096)
}

func TestTmuxReadBufferReadsFirstPane(t *testing.T) {
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	tests := []struct {
		name string
		conf string
	}{
		{name: "windows and panes numbered from 0"},
		{name: "windows and panes numbered from 1", conf: "set -g base-index 1\nsetw -g pane-base-index 1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// tmux is run by ReadBuffer with the process env, so the server is
			// isolated there with t.Setenv, and t.Parallel can't be used. TMUX
			// is cleared so that an enclosing session's server isn't used. The
			// socket path has to fit macOS's 103-byte cap, which t.TempDir can
			// exceed.
			tmuxDir, err := os.MkdirTemp("", "tmux")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(tmuxDir) })
			t.Setenv("TMUX_TMPDIR", tmuxDir)
			t.Setenv("TMUX", "")
			confPath := filepath.Join(t.TempDir(), "tmux.conf")
			require.NoError(t, os.WriteFile(confPath, []byte(tt.conf), 0o600))
			tmux := func(ctx context.Context, args ...string) (string, error) {
				//nolint:gosec // G204: realTmux is the tmux on PATH, and args are test-owned.
				out, err := exec.CommandContext(ctx, realTmux, args...).CombinedOutput()
				return string(out), err
			}
			out, err := tmux(t.Context(), "-f", confPath, "new-session", "-d", "-s", "org/repo/agent", "echo agent pane; exec cat")
			require.NoError(t, err, out)
			t.Cleanup(func() { _, _ = tmux(context.Background(), "kill-server") })
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				out, err := tmux(t.Context(), "capture-pane", "-p", "-t", "=org/repo/agent:")
				assert.NoError(c, err, out)
				assert.Contains(c, out, "agent pane")
			}, 10*time.Second, 50*time.Millisecond)

			// As when a shell is opened next to the agent, a second window is
			// made current.
			out, err = tmux(t.Context(), "new-window", "-t", "=org/repo/agent:", "exec cat")
			require.NoError(t, err, out)

			buf, err := session.NewTmux(logging.DefaultLogger()).ReadBuffer("org/repo/agent", 10)
			require.NoError(t, err)
			require.Contains(t, buf, "agent pane")
		})
	}
}

func TestTmuxPreviewMatchesSessionNameExactly(t *testing.T) {
	t.Parallel()

	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	// tmux is run under `bash -lc` in the preview, where PATH can be reset by
	// the login profile, so the server is isolated with TMUX_TMPDIR rather
	// than a -L wrapper on PATH. Its socket path has to fit macOS's 103-byte
	// cap, which t.TempDir can exceed. HOME is a temp directory so that no
	// user profile is sourced there.
	tmuxDir, err := os.MkdirTemp("", "tmux")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmuxDir) })
	env := append(filteredEnvWithout("HOME", "TMUX", "TMUX_PANE", "TMUX_TMPDIR"), "HOME="+t.TempDir(), "TMUX_TMPDIR="+tmuxDir)
	tmux := func(ctx context.Context, args ...string) (string, error) {
		//nolint:gosec // G204: realTmux is the tmux on PATH, and args are test-owned.
		cmd := exec.CommandContext(ctx, realTmux, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := tmux(t.Context(), "-f", "/dev/null", "new-session", "-d", "-s", "org/repo/rm-uumo_4", "echo rm-uumo_4 pane; exec cat")
	require.NoError(t, err, out)
	t.Cleanup(func() { _, _ = tmux(context.Background(), "kill-server") })
	require.Eventually(t, func() bool {
		out, err := tmux(t.Context(), "capture-pane", "-p", "-t", "org/repo/rm-uumo_4:")
		return err == nil && strings.Contains(out, "rm-uumo_4 pane")
	}, 2*time.Second, 50*time.Millisecond)

	preview := session.FZFPreviewCommand(session.NewTmux(logging.DefaultLogger()))
	tests := []struct {
		name string
		row  string
		want string
	}{
		{name: "exact tmux session", row: "org/repo/rm-uumo_4", want: "rm-uumo_4 pane"},
		{name: "prefix of a tmux session", row: "org/repo/rm-uumo", want: "<no buffer>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// As in fzf, {} is replaced with the single-quoted row, and the
			// preview is run with $SHELL -c.
			//nolint:gosec // G204: the preview is built by FZFPreviewCommand, and the row is test-owned.
			cmd := exec.CommandContext(t.Context(), "sh", "-c", strings.ReplaceAll(preview, "{}", shell.SingleQuote(tt.row)))
			cmd.Env = env
			out, err := cmd.Output()
			require.NoError(t, err)
			require.Contains(t, string(out), tt.want)
		})
	}
}

func filteredEnvWithout(keys ...string) []string {
	skip := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		skip[key] = struct{}{}
	}

	var out []string
	for _, kv := range os.Environ() {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, shouldSkip := skip[key]; shouldSkip {
			continue
		}
		out = append(out, kv)
	}
	return out
}
