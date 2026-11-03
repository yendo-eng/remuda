package e2e_test

import (
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestAggregateMultiplexerListAndCompletion(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	require.NoError(t, backends[session.MultiplexerTmux].Start("org/repo/tmux-session", "command"))
	require.NoError(t, backends[session.MultiplexerHerdrTUI].Start("org/repo/herdr-tui-session", "command"))

	list := h.RunOK("session", "list")
	require.Equal(t, "org/repo/tmux-session\norg/repo/herdr-tui-session\n", list.Stdout)

	completion := h.RunOK("__complete", "--session-manager", "herdr-tui", "session", "attach", "--name", "")
	require.Contains(t, completion.Stdout, "org/repo/tmux-session\n")
	require.Contains(t, completion.Stdout, "org/repo/herdr-tui-session\n")
}

func TestAggregateMultiplexerRetiredExperimentOnlyWarns(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	writeConfigFile(t, h, `
version: 1
defaults:
  experiments:
    - aggregate-multiplexer
`)
	backends[session.MultiplexerTmux].AddSessionWithBuffer("org/repo/tmux-session", "")
	backends[session.MultiplexerHerdrTUI].AddSessionWithBuffer("org/repo/herdr-tui-session", "")

	list := h.RunOK("session", "list")

	require.Equal(t, "org/repo/tmux-session\norg/repo/herdr-tui-session\n", list.Stdout)
	require.Equal(t, "warning: experiment \"aggregate-multiplexer\" was mainlined and is now a no-op; remove it\n", list.Stderr)
}

func TestAggregateMultiplexerListJSONIncludesBackend(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	backends[session.MultiplexerTmux].AddSessionWithBuffer("org/repo/tmux-session", "")
	backends[session.MultiplexerZellij].AddSessionWithBuffer("org/repo/zellij-session", "")
	backends[session.MultiplexerHerdrTUI].AddSessionWithBuffer("org/repo/herdr-tui-session", "")
	herdrCore := backends[session.MultiplexerHerdrCore]
	herdrCore.AddSessionWithBuffer("org/repo/herdr-core-session", "")
	reported := herdrCore.FindSession("org/repo/herdr-core-session")
	reported.AgentStatus = session.AgentStatusBlocked
	reported.Address = "t1"

	list := h.RunOK("session", "list", "--json")

	require.JSONEq(t, `[
  {
    "Name": "org/repo/tmux-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "tmux",
    "AgentStatus": "",
    "Address": ""
  },
  {
    "Name": "org/repo/zellij-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "zellij",
    "AgentStatus": "",
    "Address": ""
  },
  {
    "Name": "org/repo/herdr-tui-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "herdr-tui",
    "AgentStatus": "",
    "Address": ""
  },
  {
    "Name": "org/repo/herdr-core-session",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "herdr-core",
    "AgentStatus": "blocked",
    "Address": "t1"
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
	backends[session.MultiplexerHerdrTUI].AddSessionWithBuffer(sessionName, "herdr-tui output")

	read := h.RunOK("session", "readbuf", "--name", sessionName)
	require.Equal(t, "herdr-tui output", read.Stdout)
	require.Equal(t, sessionName, backends[session.MultiplexerHerdrTUI].LastReadName)
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
	require.NotNil(t, backends[session.MultiplexerHerdrTUI].FindSession(sessionName))

	h.RunOK("session", "kill", "--name", sessionName)
	require.Nil(t, backends[session.MultiplexerHerdrTUI].FindSession(sessionName))
}

func TestAggregateMultiplexerSkipsUnavailableBackends(t *testing.T) {
	t.Parallel()
	h, backends := newAggregateMultiplexerHarness(t)
	backends[session.MultiplexerTmux].ListErr = session.ErrMultiplexerUnavailable
	backends[session.MultiplexerZellij].ListErr = session.ErrMultiplexerUnavailable
	backends[session.MultiplexerHerdrTUI].AddSessionWithBuffer("org/repo/herdr-tui-session", "herdr-tui output")
	backends[session.MultiplexerRex].ListErr = session.ErrRexNotFound
	backends[session.MultiplexerHerdrCore].ListErr = session.ErrHerdrCoreNotRunning

	list := h.RunOK("session", "list")
	require.Equal(t, "org/repo/herdr-tui-session\n", list.Stdout)

	read := h.RunOK("session", "readbuf", "--name", "org/repo/herdr-tui-session")
	require.Equal(t, "herdr-tui output", read.Stdout)
}

func TestAggregateMultiplexerFailsWhenABackendFails(t *testing.T) {
	t.Parallel()

	backendErr := session.UnsupportedHerdrCoreGenerationError{Generation: 2}
	tests := []struct {
		name string
		// cloned is whether the workspace is cloned before the command runs.
		cloned bool
		args   func(remoteURL, workspace string) []string
	}{
		{name: "session list", args: func(_, _ string) []string { return []string{"session", "list"} }},
		{name: "workspaces remove", cloned: true, args: func(_, workspace string) []string { return []string{"workspaces", "remove", workspace} }},
		{name: "session resume", cloned: true, args: func(_, workspace string) []string { return []string{"session", "resume", workspace} }},
		{name: "vibe", args: func(remoteURL, _ string) []string {
			return []string{"vibe", "--name", "wk", "--repo-url", remoteURL, "--agent-cmd", "echo", "--no-container", "prompt"}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, backends := newAggregateMultiplexerHarness(t)
			h.SetEnv("REMUDA_CONTAINER", "false")
			remoteURL := testutils.InitTestRemote(t)
			org, repo, err := github.ParseRepo(remoteURL)
			require.NoError(t, err)
			workspace := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, "wk")
			if tt.cloned {
				h.RunOK("clone", "--name", "wk", "--repo-url", remoteURL)
			}
			// The workspace's session is held by a backend that fails to list it.
			sessionName := session.SessionNameFromWorkspaceName(workspace)
			backends[session.MultiplexerHerdrCore].AddSessionWithBuffer(sessionName, "")
			backends[session.MultiplexerHerdrCore].ListErr = backendErr

			res := h.Run(tt.args(remoteURL, workspace)...)

			require.ErrorIs(t, res.Err, backendErr)
			require.Empty(t, res.Stdout)
			if tt.cloned {
				require.DirExists(t, workspace)
			}
			require.Nil(t, backends[session.MultiplexerTmux].FindSession(sessionName))
		})
	}
}

func newAggregateMultiplexerHarness(t *testing.T) (*testutils.Harness, map[session.SupportedMultiplexer]*testutils.MockMultiplexer) {
	t.Helper()

	backends := map[session.SupportedMultiplexer]*testutils.MockMultiplexer{
		session.MultiplexerTmux:      {Backend: session.MultiplexerTmux},
		session.MultiplexerZellij:    {Backend: session.MultiplexerZellij},
		session.MultiplexerHerdrTUI:  {Backend: session.MultiplexerHerdrTUI},
		session.MultiplexerRex:       {Backend: session.MultiplexerRex},
		session.MultiplexerHerdrCore: {Backend: session.MultiplexerHerdrCore},
	}
	h := testutils.NewHarness(t, testutils.WithMultiplexerFactory(
		func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
			backend, ok := backends[name]
			require.True(t, ok)
			return backend
		},
	))
	return h, backends
}
