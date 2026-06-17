package testutils

import (
	"errors"
	"time"

	"github.com/yendo-eng/remuda/internal/session"
)

type MockSessionManager struct {
	sessions      []Session
	ReadBuf       string            // Default buffer for all sessions
	ReadBufs      map[string]string // Per-session buffers (keyed by session name)
	LastReadName  string
	LastReadLines int
	LastSendName  string
	LastSendInput string
	LastSendEnter bool
}

type Session struct {
	session.SessionInfo
	CommandRan string
	StartEnv   []string
}

func (f *MockSessionManager) FindSession(name string) *Session {
	for i := range f.sessions {
		if f.sessions[i].Name == name {
			return &f.sessions[i]
		}
	}

	return nil
}

func (MockSessionManager) Name() string {
	return "mock"
}

func (f *MockSessionManager) Start(sessionName, command string) error {
	f.sessions = append(f.sessions, Session{
		SessionInfo: session.SessionInfo{
			Name:      sessionName,
			CreatedAt: time.Now(),
		},
		CommandRan: command,
	})
	return nil
}

func (f *MockSessionManager) StartWithEnv(sessionName, command string, env []string) error {
	f.sessions = append(f.sessions, Session{
		SessionInfo: session.SessionInfo{
			Name:      sessionName,
			CreatedAt: time.Now(),
		},
		CommandRan: command,
		StartEnv:   append([]string{}, env...),
	})
	return nil
}

func (f *MockSessionManager) List() ([]session.SessionInfo, error) {
	var infos []session.SessionInfo
	for _, sess := range f.sessions {
		infos = append(infos, sess.SessionInfo)
	}
	return infos, nil
}

func (f *MockSessionManager) Find(name string) (session.SessionInfo, error) {
	for _, sess := range f.sessions {
		if sess.Name == name {
			return sess.SessionInfo, nil
		}
	}
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (f *MockSessionManager) Attach(name string) error {
	for _, s := range f.sessions {
		if s.Name == name {
			return nil
		}
	}

	return errCantFindSession(name)
}

func (f *MockSessionManager) ReadBuffer(name string, lines int) (string, error) {
	f.LastReadName = name
	f.LastReadLines = lines
	// Check per-session buffer first
	if f.ReadBufs != nil {
		if buf, ok := f.ReadBufs[name]; ok {
			return buf, nil
		}
	}
	return f.ReadBuf, nil
}

func (f *MockSessionManager) Send(name string, payload string, appendNewline bool) error {
	for _, s := range f.sessions {
		if s.Name == name {
			f.LastSendName = name
			f.LastSendInput = payload
			f.LastSendEnter = appendNewline
			return nil
		}
	}
	return errCantFindSession(name)
}

func (f *MockSessionManager) Kill(name string) error {
	for i, s := range f.sessions {
		if s.Name == name {
			f.sessions = append(f.sessions[:i], f.sessions[i+1:]...)
			return nil
		}
	}

	return errCantFindSession(name)
}

func errCantFindSession(name string) error {
	return errors.New("can't find session: " + name)
}

// AddSessionWithBuffer adds a session with a specific buffer content.
func (f *MockSessionManager) AddSessionWithBuffer(name, buffer string) {
	f.sessions = append(f.sessions, Session{
		SessionInfo: session.SessionInfo{
			Name: name,
		},
	})
	if f.ReadBufs == nil {
		f.ReadBufs = make(map[string]string)
	}
	f.ReadBufs[name] = buffer
}
