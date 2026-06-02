package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/session"
)

// SessionWorkspacePath resolves the absolute workspace directory for a session
// managed by Remuda.
func (k Remuda) SessionWorkspacePath(sessionName string) (string, error) {
	name := strings.TrimSpace(sessionName)
	if name == "" {
		return "", errors.New("session name is required")
	}

	sess, err := k.Session.Find(name)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return "", fmt.Errorf("session %q not found", name)
		}
		return "", err
	}

	workspace, err := sess.WorkspacePath(k.Config.ReposBaseDir)
	if err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(workspace)
	if err != nil {
		return "", errors.Wrap(err, "resolve workspace path")
	}

	return absPath, nil
}
