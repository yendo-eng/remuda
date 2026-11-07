package session_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/testutils"
)

func newMockMultiplexer(name string, sessions ...session.SessionInfo) *testutils.MockMultiplexer {
	mgr := &testutils.MockMultiplexer{MultiplexerName: name, ReadBuf: name}
	for _, info := range sessions {
		mgr.AddSession(info)
	}
	return mgr
}

func sendNames(calls []testutils.SendCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return names
}

func TestAggregateMultiplexerSkipsOnlyUnavailableBackends(t *testing.T) {
	tests := []struct {
		name string
		// herdr is the body of a herdr CLI stand-in, when one is installed.
		herdr string
		// wantErr is in the errors from List and Find, when they fail.
		wantErr string
	}{
		{name: "not installed"},
		{
			name: "herdr server not running",
			// herdr 0.9.5 answers this way and starts no server.
			herdr: `printf '%s\n' '{"id":"cli:workspace:list","error":{"code":"server_not_running","message":"no herdr server is running at /home/remuda/.config/herdr/sessions/default/herdr.sock"}}' >&2
exit 1
`,
		},
		{
			name: "herdr failing otherwise",
			herdr: `printf '%s\n' '{"id":"cli:workspace:list","error":{"code":"internal_error","message":"workspace list failed"}}' >&2
exit 1
`,
			wantErr: "herdr-tui",
		},
		{
			name: "herdr reporting an unknown agent status",
			herdr: `printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","agent_status":"paused","tokens":{"remuda":"1","org":"org","repo":"repo","folder":"held"}}]}}'
`,
			wantErr: `session "org/repo/held" (herdr-tui address w1) has unknown agent status "paused"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No backend CLI is on PATH or where the Rex app installs one, and no
			// herdr-core core is listening.
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("HOME", dir)
			t.Setenv("REX_BIN_DIR", "")
			t.Setenv("HC_SOCKET", filepath.Join(dir, "hc.sock"))
			if tt.herdr != "" {
				writeHerdrStub(t, dir, tt.herdr)
			}
			ownerSessions := []session.SessionInfo{{Name: "org/repo/owned"}}
			owner := newMockMultiplexer("owner", ownerSessions...)
			backends := make([]session.Multiplexer, 0, len(enums.ValidMultiplexers)+1)
			for _, name := range enums.ValidMultiplexers {
				backends = append(backends, session.NewMultiplexer(session.SupportedMultiplexer(name), zerolog.Nop()))
			}
			mgr := session.NewAggregateMultiplexer(owner, logging.DefaultLogger(), append(backends, owner)...)

			sessions, listErr := mgr.List()
			info, findErr := mgr.Find("org/repo/owned")
			if tt.wantErr != "" {
				require.ErrorContains(t, listErr, tt.wantErr)
				require.ErrorContains(t, findErr, tt.wantErr)
				return
			}
			require.NoError(t, listErr)
			require.Equal(t, ownerSessions, sessions)
			require.NoError(t, findErr)
			require.Equal(t, ownerSessions[0], info)
		})
	}
}

func TestAggregateMultiplexerDispatchesToSessionOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(session.Multiplexer) error
		got  func(*testutils.MockMultiplexer) []string
	}{
		{name: "attach", run: func(m session.Multiplexer) error { return m.Attach("org/repo/owned") }, got: func(b *testutils.MockMultiplexer) []string { return b.Attached }},
		{name: "read", run: func(m session.Multiplexer) error { _, err := m.ReadBuffer("org/repo/owned", 20); return err }, got: func(b *testutils.MockMultiplexer) []string { return b.ReadNames }},
		{name: "send", run: func(m session.Multiplexer) error { return m.Send("org/repo/owned", "hello", true) }, got: func(b *testutils.MockMultiplexer) []string { return sendNames(b.Sends) }},
		{name: "kill", run: func(m session.Multiplexer) error { return m.Kill("org/repo/owned") }, got: func(b *testutils.MockMultiplexer) []string { return b.Killed }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			unavailable := &testutils.MockMultiplexer{MultiplexerName: "tmux", ListErr: session.ErrMultiplexerUnavailable}
			owner := newMockMultiplexer("herdr-tui", session.SessionInfo{Name: "org/repo/owned"})
			mgr := session.NewAggregateMultiplexer(unavailable, logging.DefaultLogger(), unavailable, owner)

			require.NoError(t, tt.run(mgr))
			require.Equal(t, []string{"org/repo/owned"}, tt.got(owner))
		})
	}
}

func TestAggregateMultiplexerReturnsBackendErrors(t *testing.T) {
	t.Parallel()

	backendErr := errors.New("unexpected end of JSON input")
	tests := []struct {
		name string
		run  func(session.Multiplexer) error
	}{
		{name: "list", run: func(m session.Multiplexer) error { _, err := m.List(); return err }},
		{name: "find", run: func(m session.Multiplexer) error { _, err := m.Find("org/repo/held"); return err }},
		{name: "attach", run: func(m session.Multiplexer) error { return m.Attach("org/repo/held") }},
		{name: "read", run: func(m session.Multiplexer) error { _, err := m.ReadBuffer("org/repo/held", 20); return err }},
		{name: "send", run: func(m session.Multiplexer) error { return m.Send("org/repo/held", "hello", true) }},
		{name: "kill", run: func(m session.Multiplexer) error { return m.Kill("org/repo/held") }},
		{name: "start", run: func(m session.Multiplexer) error {
			return m.Start(session.AgentStart{SessionName: "org/repo/held", Command: "command"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			createTarget := newMockMultiplexer("tmux")
			failing := &testutils.MockMultiplexer{MultiplexerName: "herdr-core", ListErr: backendErr}
			failing.AddSession(session.SessionInfo{Name: "org/repo/held"})
			mgr := session.NewAggregateMultiplexer(createTarget, logging.DefaultLogger(), createTarget, failing)

			err := tt.run(mgr)

			require.ErrorIs(t, err, backendErr)
			require.ErrorContains(t, err, "herdr-core")
			require.Empty(t, createTarget.Starts)
		})
	}
}

func TestAggregateMultiplexerRoutesCreation(t *testing.T) {
	t.Parallel()

	t.Run("rejects duplicate from another backend", func(t *testing.T) {
		t.Parallel()

		createTarget := newMockMultiplexer("tmux")
		owner := newMockMultiplexer("herdr-tui", session.SessionInfo{Name: "org/repo/existing"})
		mgr := session.NewAggregateMultiplexer(createTarget, logging.DefaultLogger(), createTarget, owner)

		err := mgr.Start(session.AgentStart{SessionName: "org/repo/existing", Command: "command"})

		var duplicate session.SessionAlreadyExistsError
		require.ErrorAs(t, err, &duplicate)
		require.Empty(t, createTarget.Starts)
	})

	t.Run("delegates name and start", func(t *testing.T) {
		t.Parallel()

		createTarget := newMockMultiplexer("herdr-tui")
		mgr := session.NewAggregateMultiplexer(createTarget, logging.DefaultLogger(), createTarget)
		start := session.AgentStart{SessionName: "org/repo/agent", Command: "command", Env: []string{"KEY=value"}, ForwardEnv: []string{"KEY"}}

		require.Equal(t, "herdr-tui", mgr.Name())
		require.NoError(t, mgr.Start(start))
		require.Equal(t, []session.AgentStart{start}, createTarget.Starts)
	})
}
