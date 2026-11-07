package session

import (
	"errors"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type aggregateMultiplexer struct {
	createTarget Multiplexer
	backends     []Multiplexer
	logger       zerolog.Logger
}

func NewAggregateMultiplexer(createTarget Multiplexer, logger zerolog.Logger, backends ...Multiplexer) Multiplexer {
	return &aggregateMultiplexer{
		createTarget: createTarget,
		backends:     backends,
		logger:       logger,
	}
}

func (m *aggregateMultiplexer) Name() string {
	return m.createTarget.Name()
}

func (m *aggregateMultiplexer) Start(start AgentStart) error {
	if err := m.ensureSessionDoesNotExist(start.SessionName); err != nil {
		return err
	}
	return m.createTarget.Start(start)
}

func (m *aggregateMultiplexer) List() ([]SessionInfo, error) {
	var sessions []SessionInfo
	for _, backend := range m.backends {
		found, err := backend.List()
		if errors.Is(err, ErrMultiplexerUnavailable) {
			m.logUnavailableBackend(backend, err)
			continue
		}
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "list %s sessions", backend.Name())
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

func (m *aggregateMultiplexer) ensureSessionDoesNotExist(name string) error {
	_, _, err := m.findBackend(name)
	if err == nil {
		return SessionAlreadyExistsError{Name: name}
	}
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	return err
}

func (m *aggregateMultiplexer) findBackend(name string) (Multiplexer, SessionInfo, error) {
	for _, backend := range m.backends {
		info, err := backend.Find(name)
		switch {
		case err == nil:
			return backend, info, nil
		case errors.Is(err, ErrMultiplexerUnavailable):
			m.logUnavailableBackend(backend, err)
		case !errors.Is(err, ErrSessionNotFound):
			return nil, SessionInfo{}, pkgerrors.Wrapf(err, "find session in %s", backend.Name())
		}
	}
	return nil, SessionInfo{}, ErrSessionNotFound
}

func (m *aggregateMultiplexer) logUnavailableBackend(backend Multiplexer, err error) {
	m.logger.Debug().Err(err).Str("multiplexer", backend.Name()).Msg("skipping unavailable multiplexer")
}
