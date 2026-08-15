package session

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/logging"
)

type SupportedMultiplexer string

const (
	MultiplexerTmux   SupportedMultiplexer = "tmux"
	MultiplexerZellij SupportedMultiplexer = "zellij"
	MultiplexerHerdr  SupportedMultiplexer = "herdr"
)

var ErrSessionNotFound = pkgerrors.New("session not found")

func (s *SupportedMultiplexer) UnmarshalText(text []byte) error {
	val := strings.ToLower(strings.TrimSpace(string(text)))
	if !slices.Contains(enums.ValidMultiplexers, val) {
		return pkgerrors.Errorf("unknown session manager %q (valid: %s)",
			string(text), strings.Join(enums.ValidMultiplexers, ", "))
	}
	*s = SupportedMultiplexer(val)
	return nil
}

func NewMultiplexer(name SupportedMultiplexer) Multiplexer {
	return NewMultiplexerWithLogger(name, logging.DefaultLogger())
}

func NewMultiplexerWithLogger(name SupportedMultiplexer, logger zerolog.Logger) Multiplexer {
	switch name {
	case MultiplexerTmux:
		return NewTmuxWithLogger(logger)
	case MultiplexerZellij:
		return NewZellijWithLogger(logger)
	case MultiplexerHerdr:
		return NewHerdrWithLogger(logger)
	default:
		panic("unsupported session manager: " + string(name))
	}
}

// Multiplexer is an interface for a terminal multiplexer, such as tmux, used
// to house Remuda sessions.
type Multiplexer interface {
	// Name of the multiplexer.
	Name() string
	// Start starts a detached session that runs the given shell command.
	Start(sessionName, command string) error
	// List returns all sessions visible to tmux.
	List() ([]SessionInfo, error)
	// Find returns a session by name.
	Find(name string) (SessionInfo, error)
	// Attach attaches to an existing session by name (no detach of other clients).
	Attach(name string) error
	// ReadBuffer captures the last N lines from the session's primary pane. When
	// lines is 0, the entire scrollback is returned.
	ReadBuffer(name string, lines int) (string, error)
	// Send sends input to the session's active pane. When appendNewline is true,
	// a trailing newline/Enter is sent after the payload (unless already present).
	Send(name string, payload string, appendNewline bool) error
	// Kill terminates a session by name.
	Kill(name string) error
}

// EnvStarter allows callers to supply an explicit environment when starting sessions.
type EnvStarter interface {
	StartWithEnv(sessionName, command string, env []string) error
}

type AgentStart struct {
	SessionName string
	Workspace   string
	Command     string
	CommandArgv []string
	Container   bool
	Agent       string
	Args        []string
	Prompt      string
	Env         []string
}

// AgentStarter starts a known agent without routing its argv through a shell.
type AgentStarter interface {
	StartAgent(start AgentStart) error
}

// LoggerSetter allows wiring a per-invocation logger into multiplexers.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

// SessionInfo is a minimal description of a multiplexer session.
type SessionInfo struct {
	Name        string
	Attached    bool
	CreatedAt   time.Time // Zero means unknown.
	Multiplexer string
}

func (s SessionInfo) IsRemudaSession() bool {
	parts := strings.Split(strings.TrimSpace(s.Name), "/")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}

// maps org/repo/folder → base/org/repo/folder.
func (s SessionInfo) WorkspacePath(base string) (string, error) {
	if !s.IsRemudaSession() {
		return "", pkgerrors.New("not a Remuda session")
	}

	parts := strings.Split(strings.TrimSpace(s.Name), "/")
	if len(parts) != 3 {
		return "", pkgerrors.New("invalid session name format")
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", pkgerrors.New("invalid session name format")
	}
	org, repo, folder := parts[0], parts[1], parts[2]
	// First try the direct mapping.
	direct := filepath.Join(base, org, repo, folder)
	if st, err := os.Stat(direct); err == nil && st.IsDir() {
		return direct, nil
	}

	// Fallback: tmux converts dots to underscores in session names on some systems.
	// To resolve the correct workspace folder, look for a sibling directory whose
	// name, when sanitized ('.' → '_'), matches the session folder token.
	repoDir := filepath.Join(base, org, repo)
	if entries, err := os.ReadDir(repoDir); err == nil {
		target := folder
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if sanitizeTmuxSessionToken(name) == target {
				return filepath.Join(repoDir, name), nil
			}
		}
	}

	// Best effort: return the direct mapping even if it doesn't exist so callers
	// can diagnose missing directories consistently.
	return direct, nil
}

// sanitizeTmuxSessionToken mirrors tmux's tendency to map '.' to '_' in session
// names. Keep this local to the session package to avoid import cycles.
func sanitizeTmuxSessionToken(s string) string {
	return strings.ReplaceAll(s, ".", "_")
}
