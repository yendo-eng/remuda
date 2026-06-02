package session_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
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

	got, err := session.NewTmuxManager().List()
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestTmuxListHandlesExitCode1WithoutCanonicalMessage(t *testing.T) {
	tmp := t.TempDir()
	// Some tmux builds output different wording.
	_ = writeStubTmux(t, tmp, "failed to connect to server", 1)
	old := os.Getenv("PATH")
	t.Setenv("PATH", tmp+string(os.PathListSeparator)+old)

	got, err := session.NewTmuxManager().List()
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

	got, err := session.NewTmuxManager().List()
	require.NoError(t, err)
	require.Equal(t, []session.SessionInfo{
		{Name: "org/repo/work", Attached: true, CreatedAt: time.Unix(1710000000, 0).UTC()},
		{Name: "other session", Attached: false, CreatedAt: time.Unix(1710000123, 0).UTC()},
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

	got, err := session.NewTmuxManager().List()
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "org/repo/work", got[0].Name)
	require.True(t, got[0].Attached)
	require.True(t, got[0].CreatedAt.IsZero())
	require.Equal(t, "other", got[1].Name)
	require.False(t, got[1].Attached)
	require.True(t, got[1].CreatedAt.IsZero())
}
