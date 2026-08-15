package testutils

import (
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/session"
)

type MockMultiplexer struct {
	Backend       session.SupportedMultiplexer
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

func (f *MockMultiplexer) FindSession(name string) *Session {
	for i := range f.sessions {
		if f.sessions[i].Name == name {
			return &f.sessions[i]
		}
	}

	return nil
}

func (MockMultiplexer) Name() string {
	return "mock"
}

func (f *MockMultiplexer) Start(sessionName, command string) error {
	f.sessions = append(f.sessions, Session{
		SessionInfo: f.sessionInfo(sessionName),
		CommandRan:  command,
	})
	f.sessions[len(f.sessions)-1].CreatedAt = time.Now()
	return nil
}

func (f *MockMultiplexer) StartWithEnv(sessionName, command string, env []string) error {
	f.sessions = append(f.sessions, Session{
		SessionInfo: f.sessionInfo(sessionName),
		CommandRan:  command,
		StartEnv:    append([]string{}, env...),
	})
	f.sessions[len(f.sessions)-1].CreatedAt = time.Now()
	return nil
}

func (f *MockMultiplexer) List() ([]session.SessionInfo, error) {
	var infos []session.SessionInfo
	for _, sess := range f.sessions {
		infos = append(infos, sess.SessionInfo)
	}
	return infos, nil
}

func (f *MockMultiplexer) Find(name string) (session.SessionInfo, error) {
	for _, sess := range f.sessions {
		if sess.Name == name {
			return sess.SessionInfo, nil
		}
	}
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (f *MockMultiplexer) Attach(name string) error {
	for _, s := range f.sessions {
		if s.Name == name {
			return nil
		}
	}

	return errCantFindSession(name)
}

func (f *MockMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	f.LastReadName = name
	f.LastReadLines = lines
	if lines < 0 {
		lines = 200
	}

	truncate := func(buf string) string {
		content := strings.ReplaceAll(buf, "\r\n", "\n")
		linesSlice := strings.Split(content, "\n")
		if lines > 0 {
			for len(linesSlice) > 0 && strings.TrimSpace(linesSlice[len(linesSlice)-1]) == "" {
				linesSlice = linesSlice[:len(linesSlice)-1]
			}
			if len(linesSlice) > lines {
				linesSlice = linesSlice[len(linesSlice)-lines:]
			}
		}
		return strings.Join(linesSlice, "\n")
	}

	// Check per-session buffer first
	if f.ReadBufs != nil {
		if buf, ok := f.ReadBufs[name]; ok {
			return truncate(buf), nil
		}
	}
	return truncate(f.ReadBuf), nil
}

func (f *MockMultiplexer) Send(name string, payload string, appendNewline bool) error {
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

func (f *MockMultiplexer) Kill(name string) error {
	for i, s := range f.sessions {
		if s.Name == name {
			f.sessions = append(f.sessions[:i], f.sessions[i+1:]...)
			return nil
		}
	}

	return errCantFindSession(name)
}

func errCantFindSession(name string) error {
	return pkgerrors.New("can't find session: " + name)
}

// AddSessionWithBuffer adds a session with a specific buffer content.
func (f *MockMultiplexer) AddSessionWithBuffer(name, buffer string) {
	f.sessions = append(f.sessions, Session{
		SessionInfo: f.sessionInfo(name),
	})
	if f.ReadBufs == nil {
		f.ReadBufs = make(map[string]string)
	}
	f.ReadBufs[name] = buffer
}

func (f MockMultiplexer) sessionInfo(name string) session.SessionInfo {
	info := session.SessionInfo{Name: name}
	if f.Backend != "" {
		info.Multiplexer = string(f.Backend)
	}
	return info
}
