package agentlauncher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeLauncher_Command_DefaultModel(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), "", false)
	require.NoError(t, err)
	require.Equal(t, DefaultModel(string(AgentOpenCode)), resolved)
	require.Equal(
		t,
		"opencode --model '"+DefaultModel(string(AgentOpenCode))+"' --prompt 'line 1\nline 2'",
		l.Command("line 1\nline 2"),
	)
}

func TestOpenCodeLauncher_Command_AgentDefaultOmitsModelFlag(t *testing.T) {
	l, resolved, err := Parse(string(AgentOpenCode), ModelAgentDefault, false)
	require.NoError(t, err)
	require.Equal(t, ModelAgentDefault, resolved)
	require.Equal(t, "opencode --prompt 'line 1\nline 2'", l.Command("line 1\nline 2"))
}

func TestOpenCodeLauncher_Command_WithModelAndQuotes(t *testing.T) {
	l := OpenCode("gpt-4o")
	got := l.Command("don't stop")
	require.Equal(t, "opencode --model 'gpt-4o' --prompt 'don'\\''t stop'", got)
}

func TestOpenCodeLauncher_Command_ExtraArgsBeforePrompt(t *testing.T) {
	l := OpenCode("gpt-4o")
	got := l.Command("hello", "--foo", "--bar")
	require.Equal(t, "opencode --model 'gpt-4o' '--foo' '--bar' --prompt 'hello'", got)
}

func TestOpenCodeLauncher_Arguments(t *testing.T) {
	l := OpenCode("openai/gpt-5")
	require.Equal(t, []string{
		"--model", "openai/gpt-5",
		"--foo",
		"--prompt", "fix it",
	}, l.Arguments("fix it", "--foo"))
}

func TestBuildOpenCodeStateMountOpts_LinuxPath(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".local", "share", "opencode")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	opts := buildOpenCodeStateMountOpts(zerolog.Nop(), "linux", tmp)
	require.Equal(t, []string{"-v", stateDir + ":/root/.local/share/opencode:rw"}, opts)
}

func TestBuildOpenCodeStateMountOpts_MacPath(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, "Library", "Application Support", "opencode")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	opts := buildOpenCodeStateMountOpts(zerolog.Nop(), "darwin", tmp)
	require.Equal(t, []string{"-v", stateDir + ":/root/.local/share/opencode:rw"}, opts)
}

func TestBuildOpenCodeStateMountOpts_CreatesPreferredDir(t *testing.T) {
	t.Run("linux", func(t *testing.T) {
		tmp := t.TempDir()
		stateDir := filepath.Join(tmp, ".local", "share", "opencode")

		opts := buildOpenCodeStateMountOpts(zerolog.Nop(), "linux", tmp)
		require.Equal(t, []string{"-v", stateDir + ":/root/.local/share/opencode:rw"}, opts)
		require.DirExists(t, stateDir)
	})

	t.Run("darwin", func(t *testing.T) {
		tmp := t.TempDir()
		stateDir := filepath.Join(tmp, "Library", "Application Support", "opencode")

		opts := buildOpenCodeStateMountOpts(zerolog.Nop(), "darwin", tmp)
		require.Equal(t, []string{"-v", stateDir + ":/root/.local/share/opencode:rw"}, opts)
		require.DirExists(t, stateDir)
	})
}
