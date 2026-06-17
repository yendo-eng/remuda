package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/logging"
)

type SupportedSessionManager string

const (
	SessionManagerTmux   SupportedSessionManager = "tmux"
	SessionManagerZellij SupportedSessionManager = "zellij"
)

var ErrSessionNotFound = errors.New("session not found")

func (s *SupportedSessionManager) UnmarshalText(text []byte) error {
	val := strings.ToLower(strings.TrimSpace(string(text)))
	if !slices.Contains(enums.ValidSessionManagers, val) {
		return fmt.Errorf("unknown session manager %q (valid: %s)",
			string(text), strings.Join(enums.ValidSessionManagers, ", "))
	}
	*s = SupportedSessionManager(val)
	return nil
}

func NewSessionManager(name SupportedSessionManager) SessionManager {
	return NewSessionManagerWithLogger(name, logging.DefaultLogger())
}

func NewSessionManagerWithLogger(name SupportedSessionManager, logger zerolog.Logger) SessionManager {
	switch name {
	case SessionManagerTmux:
		return NewTmuxManagerWithLogger(logger)
	case SessionManagerZellij:
		return NewZellijManagerWithLogger(logger)
	default:
		panic("unsupported session manager: " + string(name))
	}
}

type SessionManager interface {
	// Name of the session manager
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

// LoggerSetter allows wiring a per-invocation logger into session managers.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

// SessionInfo is a minimal description of a tmux session.
type SessionInfo struct {
	Name      string
	Attached  bool
	CreatedAt time.Time // Zero means unknown.
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
	return s.WorkspacePathFromRoots(base)
}

// WorkspacePathFromRoots resolves a session's workspace directory by probing each
// candidate root in order. A session named org/repo/folder maps to
// <root>/org/repo/folder. This supports --tmp sessions, whose worktree lives
// under the OS-temp root rather than the persistent repos base dir. The first
// root that contains the workspace wins; when none do, the direct mapping under
// the first root is returned so callers can diagnose missing directories
// consistently.
func (s SessionInfo) WorkspacePathFromRoots(roots ...string) (string, error) {
	if !s.IsRemudaSession() {
		return "", errors.New("not a Remuda session")
	}

	parts := strings.Split(strings.TrimSpace(s.Name), "/")
	if len(parts) != 3 {
		return "", errors.New("invalid session name format")
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", errors.New("invalid session name format")
	}
	if len(roots) == 0 {
		return "", errors.New("no workspace roots provided")
	}
	org, repo, folder := parts[0], parts[1], parts[2]

	for _, base := range roots {
		direct := filepath.Join(base, org, repo, folder)
		if st, err := os.Stat(direct); err == nil && st.IsDir() {
			return direct, nil
		}

		// Fallback: tmux converts dots to underscores in session names on some
		// systems. To resolve the correct workspace folder, look for a sibling
		// directory whose name, when sanitized ('.' → '_'), matches the session
		// folder token.
		repoDir := filepath.Join(base, org, repo)
		if entries, err := os.ReadDir(repoDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if SanitizeTmuxSessionToken(e.Name()) == folder {
					return filepath.Join(repoDir, e.Name()), nil
				}
			}
		}
	}

	// Best effort: return the direct mapping under the first root even if it does
	// not exist so callers can diagnose missing directories consistently.
	return filepath.Join(roots[0], org, repo, folder), nil
}
