package session_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/session"
)

type aggregateBackend struct {
	name       string
	sessions   []session.SessionInfo
	listErr    error
	started    []string
	attached   []string
	read       []string
	sent       []string
	killed     []string
	agentStart []session.AgentStart
	env        []string
	loggerSet  bool
}

func (b *aggregateBackend) Name() string { return b.name }
func (b *aggregateBackend) Start(name, _ string) error {
	b.started = append(b.started, name)
	return nil
}
func (b *aggregateBackend) StartWithEnv(name, _ string, env []string) error {
	b.started = append(b.started, name)
	b.env = append([]string(nil), env...)
	return nil
}
func (b *aggregateBackend) StartAgent(start session.AgentStart) error {
	b.agentStart = append(b.agentStart, start)
	return nil
}
func (b *aggregateBackend) List() ([]session.SessionInfo, error) {
	return b.sessions, b.listErr
}
func (b *aggregateBackend) Find(name string) (session.SessionInfo, error) {
	if b.listErr != nil {
		return session.SessionInfo{}, b.listErr
	}
	for _, info := range b.sessions {
		if info.Name == name {
			return info, nil
		}
	}
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (b *aggregateBackend) Attach(name string) error {
	b.attached = append(b.attached, name)
	return nil
}
func (b *aggregateBackend) ReadBuffer(name string, _ int) (string, error) {
	b.read = append(b.read, name)
	return b.name, nil
}
func (b *aggregateBackend) Send(name, _ string, _ bool) error {
	b.sent = append(b.sent, name)
	return nil
}
func (b *aggregateBackend) Kill(name string) error {
	b.killed = append(b.killed, name)
	return nil
}
func (b *aggregateBackend) SetLogger(zerolog.Logger) { b.loggerSet = true }

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
			owner := &aggregateBackend{name: "owner", sessions: []session.SessionInfo{{Name: "org/repo/owned"}}}
			backends := make([]session.Multiplexer, 0, len(enums.ValidMultiplexers)+1)
			for _, name := range enums.ValidMultiplexers {
				backends = append(backends, session.NewMultiplexerWithLogger(session.SupportedMultiplexer(name), zerolog.Nop()))
			}
			mgr := session.NewAggregateMultiplexer(owner, append(backends, owner)...)

			sessions, listErr := mgr.List()
			info, findErr := mgr.Find("org/repo/owned")
			if tt.wantErr != "" {
				require.ErrorContains(t, listErr, tt.wantErr)
				require.ErrorContains(t, findErr, tt.wantErr)
				return
			}
			require.NoError(t, listErr)
			require.Equal(t, owner.sessions, sessions)
			require.NoError(t, findErr)
			require.Equal(t, owner.sessions[0], info)
		})
	}
}

func TestAggregateMultiplexerDispatchesToSessionOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(session.Multiplexer) error
		got  func(*aggregateBackend) []string
	}{
		{name: "attach", run: func(m session.Multiplexer) error { return m.Attach("org/repo/owned") }, got: func(b *aggregateBackend) []string { return b.attached }},
		{name: "read", run: func(m session.Multiplexer) error { _, err := m.ReadBuffer("org/repo/owned", 20); return err }, got: func(b *aggregateBackend) []string { return b.read }},
		{name: "send", run: func(m session.Multiplexer) error { return m.Send("org/repo/owned", "hello", true) }, got: func(b *aggregateBackend) []string { return b.sent }},
		{name: "kill", run: func(m session.Multiplexer) error { return m.Kill("org/repo/owned") }, got: func(b *aggregateBackend) []string { return b.killed }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			unavailable := &aggregateBackend{name: "tmux", listErr: session.ErrMultiplexerUnavailable}
			owner := &aggregateBackend{name: "herdr-tui", sessions: []session.SessionInfo{{Name: "org/repo/owned"}}}
			mgr := session.NewAggregateMultiplexer(unavailable, unavailable, owner)

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
		{name: "start", run: func(m session.Multiplexer) error { return m.Start("org/repo/held", "command") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			createTarget := &aggregateBackend{name: "tmux"}
			failing := &aggregateBackend{name: "herdr-core", sessions: []session.SessionInfo{{Name: "org/repo/held"}}, listErr: backendErr}
			mgr := session.NewAggregateMultiplexer(createTarget, createTarget, failing)

			err := tt.run(mgr)

			require.ErrorIs(t, err, backendErr)
			require.ErrorContains(t, err, "herdr-core")
			require.Empty(t, createTarget.started)
		})
	}
}

func TestAggregateMultiplexerRoutesCreation(t *testing.T) {
	t.Parallel()

	t.Run("rejects duplicate from another backend", func(t *testing.T) {
		t.Parallel()

		createTarget := &aggregateBackend{name: "tmux"}
		owner := &aggregateBackend{name: "herdr-tui", sessions: []session.SessionInfo{{Name: "org/repo/existing"}}}
		mgr := session.NewAggregateMultiplexer(createTarget, createTarget, owner)

		err := mgr.Start("org/repo/existing", "command")

		var duplicate session.SessionAlreadyExistsError
		require.ErrorAs(t, err, &duplicate)
		require.Empty(t, createTarget.started)
	})

	t.Run("delegates name and start variants", func(t *testing.T) {
		t.Parallel()

		createTarget := &aggregateBackend{name: "herdr-tui"}
		mgr := session.NewAggregateMultiplexer(createTarget, createTarget)
		envStarter, ok := mgr.(session.EnvStarter)
		require.True(t, ok)
		agentStarter, ok := mgr.(session.AgentStarter)
		require.True(t, ok)

		require.Equal(t, "herdr-tui", mgr.Name())
		require.NoError(t, mgr.Start("org/repo/basic", "command"))
		require.NoError(t, envStarter.StartWithEnv("org/repo/env", "command", []string{"KEY=value"}))
		require.NoError(t, agentStarter.StartAgent(session.AgentStart{SessionName: "org/repo/agent"}))
		require.Equal(t, []string{"org/repo/basic", "org/repo/env"}, createTarget.started)
		require.Equal(t, []string{"KEY=value"}, createTarget.env)
		require.Equal(t, "org/repo/agent", createTarget.agentStart[0].SessionName)
	})
}

func TestAggregateMultiplexerSetsLoggerOnAllBackends(t *testing.T) {
	t.Parallel()

	tmux := &aggregateBackend{name: "tmux"}
	herdrTUI := &aggregateBackend{name: "herdr-tui"}
	mgr := session.NewAggregateMultiplexer(tmux, tmux, herdrTUI)
	setter, ok := mgr.(session.LoggerSetter)
	require.True(t, ok)

	setter.SetLogger(zerolog.Nop())

	require.True(t, tmux.loggerSet)
	require.True(t, herdrTUI.loggerSet)
}
