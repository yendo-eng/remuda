package session_test

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
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

func TestAggregateMultiplexerListSkipsUnavailableBackends(t *testing.T) {
	t.Parallel()

	tmux := &aggregateBackend{name: "tmux", sessions: []session.SessionInfo{{Name: "org/repo/tmux", Multiplexer: "tmux"}}}
	zellij := &aggregateBackend{name: "zellij", listErr: errors.New("zellij unavailable")}
	herdr := &aggregateBackend{name: "herdr", sessions: []session.SessionInfo{{Name: "org/repo/herdr", Multiplexer: "herdr"}}}
	mgr := session.NewAggregateMultiplexer(tmux, tmux, zellij, herdr)

	got, err := mgr.List()

	require.NoError(t, err)
	require.Equal(t, []session.SessionInfo{
		{Name: "org/repo/tmux", Multiplexer: "tmux"},
		{Name: "org/repo/herdr", Multiplexer: "herdr"},
	}, got)
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

			unavailable := &aggregateBackend{name: "tmux", listErr: errors.New("tmux unavailable")}
			owner := &aggregateBackend{name: "herdr", sessions: []session.SessionInfo{{Name: "org/repo/owned"}}}
			mgr := session.NewAggregateMultiplexer(unavailable, unavailable, owner)

			require.NoError(t, tt.run(mgr))
			require.Equal(t, []string{"org/repo/owned"}, tt.got(owner))
		})
	}
}

func TestAggregateMultiplexerRoutesCreation(t *testing.T) {
	t.Parallel()

	t.Run("rejects duplicate from another backend", func(t *testing.T) {
		t.Parallel()

		createTarget := &aggregateBackend{name: "tmux"}
		owner := &aggregateBackend{name: "herdr", sessions: []session.SessionInfo{{Name: "org/repo/existing"}}}
		mgr := session.NewAggregateMultiplexer(createTarget, createTarget, owner)

		err := mgr.Start("org/repo/existing", "command")

		var duplicate session.SessionAlreadyExistsError
		require.ErrorAs(t, err, &duplicate)
		require.Empty(t, createTarget.started)
	})

	t.Run("delegates name and start variants", func(t *testing.T) {
		t.Parallel()

		createTarget := &aggregateBackend{name: "herdr"}
		mgr := session.NewAggregateMultiplexer(createTarget, createTarget)
		envStarter, ok := mgr.(session.EnvStarter)
		require.True(t, ok)
		agentStarter, ok := mgr.(session.AgentStarter)
		require.True(t, ok)

		require.Equal(t, "herdr", mgr.Name())
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
	herdr := &aggregateBackend{name: "herdr"}
	mgr := session.NewAggregateMultiplexer(tmux, tmux, herdr)
	setter, ok := mgr.(session.LoggerSetter)
	require.True(t, ok)

	setter.SetLogger(zerolog.Nop())

	require.True(t, tmux.loggerSet)
	require.True(t, herdr.loggerSet)
}
