package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestAggregateMultiplexerListAndCompletionFromConfig(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	require.NoError(t, backends[session.MultiplexerTmux].Start("org/repo/tmux-session", "command"))
	require.NoError(t, backends[session.MultiplexerHerdr].Start("org/repo/herdr-session", "command"))

	list := h.RunOK("session", "list")
	require.Equal(t, "org/repo/tmux-session\norg/repo/herdr-session\n", list.Stdout)

	completion := h.RunOK("__complete", "--session-manager", "herdr", "session", "attach", "--name", "")
	require.Contains(t, completion.Stdout, "org/repo/tmux-session\n")
	require.Contains(t, completion.Stdout, "org/repo/herdr-session\n")
}

func TestAggregateMultiplexerListJSONIncludesBackend(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	backends[session.MultiplexerTmux].AddSessionWithBuffer("org/repo/tmux-session", "")
	backends[session.MultiplexerZellij].AddSessionWithBuffer("org/repo/zellij-session", "")
	backends[session.MultiplexerHerdr].AddSessionWithBuffer("org/repo/herdr-session", "")

	list := h.RunOK("session", "list", "--json")

	require.JSONEq(t, `[
  {
    "Name": "org/repo/tmux-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "tmux"
  },
  {
    "Name": "org/repo/zellij-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "zellij"
  },
  {
    "Name": "org/repo/herdr-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "herdr"
  }
]`, list.Stdout)
}

func TestAggregateMultiplexerRoutesAndRejectsCrossBackendDuplicate(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	remoteURL := testutils.InitTestRemote(t)
	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)
	sessionName := session.SessionNameFromWorkspaceName(filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, "wk"))
	backends[session.MultiplexerHerdr].AddSessionWithBuffer(sessionName, "herdr output")

	read := h.RunOK("session", "readbuf", "--name", sessionName)
	require.Equal(t, "herdr output", read.Stdout)
	require.Equal(t, sessionName, backends[session.MultiplexerHerdr].LastReadName)
	require.Empty(t, backends[session.MultiplexerTmux].LastReadName)

	start := h.Run(
		"vibe",
		"--name", "wk",
		"--repo-url", remoteURL,
		"--agent-cmd", "echo",
		"--no-container",
		"prompt",
	)
	require.ErrorContains(t, start.Err, "session \""+sessionName+"\" already exists")
	require.Nil(t, backends[session.MultiplexerTmux].FindSession(sessionName))
	require.NotNil(t, backends[session.MultiplexerHerdr].FindSession(sessionName))

	h.RunOK("session", "kill", "--name", sessionName)
	require.Nil(t, backends[session.MultiplexerHerdr].FindSession(sessionName))
}

func newAggregateMultiplexerHarness(t *testing.T) (*testutils.Harness, map[session.SupportedMultiplexer]*testutils.MockMultiplexer) {
	t.Helper()

	backends := map[session.SupportedMultiplexer]*testutils.MockMultiplexer{
		session.MultiplexerTmux:   {Backend: session.MultiplexerTmux},
		session.MultiplexerZellij: {Backend: session.MultiplexerZellij},
		session.MultiplexerHerdr:  {Backend: session.MultiplexerHerdr},
	}
	h := testutils.NewHarness(t, testutils.WithMultiplexerFactory(
		func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
			backend, ok := backends[name]
			require.True(t, ok)
			return backend
		},
	))
	configPath := filepath.Join(h.HomeDir, ".config", "remuda", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("version: 1\ndefaults:\n  experiments:\n    - aggregate-multiplexer\n"), 0o644))
	return h, backends
}
