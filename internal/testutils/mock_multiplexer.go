package testutils

import (
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/session"
)

type MockMultiplexer struct {
	Backend         session.SupportedMultiplexer
	MultiplexerName string
	ListErr         error
	StartErr        error
	AttachErr       error
	ReadBufferErr   error
	SendErr         error
	SendErrs        map[string]error
	KillErr         error
	Sessions        []Session
	ReadBuf         string
	ReadBufs        map[string]string
	LastStart       session.AgentStart
	Starts          []session.AgentStart
	Attached        []string
	LastReadName    string
	LastReadLines   int
	LastSendName    string
	LastSendInput   string
	LastSendEnter   bool
	Sends           []SendCall
	Killed          []string
}

type Session struct {
	session.SessionInfo
	CommandRan string
	StartEnv   []string
}

type SendCall struct {
	Name          string
	Payload       string
	AppendNewline bool
}

func (f *MockMultiplexer) FindSession(name string) *Session {
	for i := range f.Sessions {
		if f.Sessions[i].Name == name {
			return &f.Sessions[i]
		}
	}
	return nil
}

func (f MockMultiplexer) Name() string {
	if f.MultiplexerName != "" {
		return f.MultiplexerName
	}
	return "mock"
}

func (f *MockMultiplexer) Start(start session.AgentStart) error {
	if f.StartErr != nil {
		return f.StartErr
	}
	f.LastStart = start
	f.Starts = append(f.Starts, start)
	f.Sessions = append(f.Sessions, Session{
		SessionInfo: f.sessionInfo(start.SessionName),
		CommandRan:  start.Command,
		StartEnv:    append([]string{}, start.Env...),
	})
	f.Sessions[len(f.Sessions)-1].CreatedAt = time.Now()
	return nil
}

func (f *MockMultiplexer) List() ([]session.SessionInfo, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	var infos []session.SessionInfo
	for _, sess := range f.Sessions {
		infos = append(infos, sess.SessionInfo)
	}
	return infos, nil
}

func (f *MockMultiplexer) Find(name string) (session.SessionInfo, error) {
	if f.ListErr != nil {
		return session.SessionInfo{}, f.ListErr
	}
	for _, sess := range f.Sessions {
		if sess.Name == name {
			return sess.SessionInfo, nil
		}
	}
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (f *MockMultiplexer) Attach(name string) error {
	if f.AttachErr != nil {
		return f.AttachErr
	}
	if f.FindSession(name) == nil {
		return errCantFindSession(name)
	}
	f.Attached = append(f.Attached, name)
	return nil
}

func (f *MockMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	f.LastReadName = name
	f.LastReadLines = lines
	if f.ReadBufferErr != nil {
		return "", f.ReadBufferErr
	}
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
	if f.ReadBufs != nil {
		if buf, ok := f.ReadBufs[name]; ok {
			return truncate(buf), nil
		}
	}
	return truncate(f.ReadBuf), nil
}

func (f *MockMultiplexer) Send(name string, payload string, appendNewline bool) error {
	if f.SendErr != nil {
		return f.SendErr
	}
	if err := f.SendErrs[name]; err != nil {
		return err
	}
	if f.FindSession(name) == nil {
		return errCantFindSession(name)
	}
	f.LastSendName = name
	f.LastSendInput = payload
	f.LastSendEnter = appendNewline
	f.Sends = append(f.Sends, SendCall{Name: name, Payload: payload, AppendNewline: appendNewline})
	return nil
}

func (f *MockMultiplexer) Kill(name string) error {
	if f.KillErr != nil {
		return f.KillErr
	}
	for i, s := range f.Sessions {
		if s.Name == name {
			f.Sessions = append(f.Sessions[:i], f.Sessions[i+1:]...)
			f.Killed = append(f.Killed, name)
			return nil
		}
	}
	return errCantFindSession(name)
}

func (f *MockMultiplexer) AddSession(info session.SessionInfo) {
	f.Sessions = append(f.Sessions, Session{SessionInfo: info})
}

func (f *MockMultiplexer) AddSessionWithBuffer(name, buffer string) {
	f.AddSession(f.sessionInfo(name))
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

func errCantFindSession(name string) error {
	return pkgerrors.New("can't find session: " + name)
}
