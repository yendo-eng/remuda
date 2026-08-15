package session

import (
	"errors"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
)

type aggregateMultiplexer struct {
	createTarget Multiplexer
	backends     []Multiplexer
	logger       zerolog.Logger
}

func NewAggregateMultiplexer(createTarget Multiplexer, backends ...Multiplexer) Multiplexer {
	return NewAggregateMultiplexerWithLogger(createTarget, logging.DefaultLogger(), backends...)
}

func NewAggregateMultiplexerWithLogger(createTarget Multiplexer, logger zerolog.Logger, backends ...Multiplexer) Multiplexer {
	return &aggregateMultiplexer{
		createTarget: createTarget,
		backends:     backends,
		logger:       logger,
	}
}

func (m *aggregateMultiplexer) Name() string {
	return m.createTarget.Name()
}

func (m *aggregateMultiplexer) Start(name, command string) error {
	if err := m.ensureSessionDoesNotExist(name); err != nil {
		return err
	}
	return m.createTarget.Start(name, command)
}

func (m *aggregateMultiplexer) StartWithEnv(name, command string, env []string) error {
	if err := m.ensureSessionDoesNotExist(name); err != nil {
		return err
	}
	if starter, ok := m.createTarget.(EnvStarter); ok {
		return starter.StartWithEnv(name, command, env)
	}
	return m.createTarget.Start(name, command)
}

func (m *aggregateMultiplexer) StartAgent(start AgentStart) error {
	if err := m.ensureSessionDoesNotExist(start.SessionName); err != nil {
		return err
	}
	if starter, ok := m.createTarget.(AgentStarter); ok {
		return starter.StartAgent(start)
	}
	if starter, ok := m.createTarget.(EnvStarter); ok {
		return starter.StartWithEnv(start.SessionName, start.Command, start.Env)
	}
	return m.createTarget.Start(start.SessionName, start.Command)
}

func (m *aggregateMultiplexer) List() ([]SessionInfo, error) {
	var sessions []SessionInfo
	for _, backend := range m.backends {
		found, err := backend.List()
		if err != nil {
			m.logUnavailableBackend(backend, err)
			continue
		}
		sessions = append(sessions, found...)
	}
	return sessions, nil
}

func (m *aggregateMultiplexer) Find(name string) (SessionInfo, error) {
	_, info, err := m.findBackend(name)
	return info, err
}

func (m *aggregateMultiplexer) Attach(name string) error {
	backend, _, err := m.findBackend(name)
	if err != nil {
		return err
	}
	return backend.Attach(name)
}

func (m *aggregateMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	backend, _, err := m.findBackend(name)
	if err != nil {
		return "", err
	}
	return backend.ReadBuffer(name, lines)
}

func (m *aggregateMultiplexer) Send(name, payload string, appendNewline bool) error {
	backend, _, err := m.findBackend(name)
	if err != nil {
		return err
	}
	return backend.Send(name, payload, appendNewline)
}

func (m *aggregateMultiplexer) Kill(name string) error {
	backend, _, err := m.findBackend(name)
	if err != nil {
		return err
	}
	return backend.Kill(name)
}

func (m *aggregateMultiplexer) SetLogger(logger zerolog.Logger) {
	m.logger = logger
	for _, backend := range m.backends {
		if setter, ok := backend.(LoggerSetter); ok {
			setter.SetLogger(logger)
		}
	}
}

func (m *aggregateMultiplexer) ensureSessionDoesNotExist(name string) error {
	if _, _, err := m.findBackend(name); err == nil {
		return SessionAlreadyExistsError{Name: name}
	}
	return nil
}

func (m *aggregateMultiplexer) findBackend(name string) (Multiplexer, SessionInfo, error) {
	for _, backend := range m.backends {
		info, err := backend.Find(name)
		if err == nil {
			return backend, info, nil
		}
		if !errors.Is(err, ErrSessionNotFound) {
			m.logUnavailableBackend(backend, err)
		}
	}
	return nil, SessionInfo{}, ErrSessionNotFound
}

func (m *aggregateMultiplexer) logUnavailableBackend(backend Multiplexer, err error) {
	m.logger.Debug().Err(err).Str("multiplexer", backend.Name()).Msg("skipping unavailable multiplexer")
}
