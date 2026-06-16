package internal

import (
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// runForegroundAgent runs a non-detached agent command, streaming its output to
// the user. When the command runs in a container and fails, it scans the tail of
// stderr for a Docker mount-denied error and, if found, wraps the failure with
// actionable file-sharing guidance instead of surfacing docker's raw message.
func (k Remuda) runForegroundAgent(execCmd *exec.Cmd, container bool, workspaceAbs string) error {
	execCmd.Stdout = k.IO.Out
	if !container {
		execCmd.Stderr = k.IO.Err
		return execCmd.Run()
	}

	// Container startup mount errors appear immediately and are short, so a small
	// tail of stderr is enough to detect them without buffering a long session.
	tail := newTailBuffer(8 << 10)
	execCmd.Stderr = io.MultiWriter(k.IO.Err, tail)
	err := execCmd.Run()
	if err != nil {
		if hint := dockerMountDeniedHint(tail.String(), workspaceAbs, k.Config.TmpBaseDir); hint != "" {
			return errors.Wrap(err, hint)
		}
	}
	return err
}

// dockerMountDeniedHint inspects docker output for a file-sharing / mount-denied
// failure and returns actionable guidance, or "" when the output does not look
// like one. Docker Desktop reports unshared bind-mount paths with messages such
// as "Mounts denied: The path X is not shared from the host and is not known to
// Docker." We recognize that and, when the failing workspace is a --tmp worktree,
// point the user at REMUDA_TMP_DIR specifically.
//
// This is reactive on purpose: we no longer guess whether a path is shared
// (Docker Desktop now shares /var/folders by default, so guessing produced false
// positives). Instead we let docker run and only translate the error if it
// actually fails on a mount.
func dockerMountDeniedHint(output, workspaceAbs, tmpBaseDir string) string {
	lower := strings.ToLower(output)
	mountDenied := strings.Contains(lower, "mounts denied") ||
		strings.Contains(lower, "not shared from the host") ||
		strings.Contains(lower, "is not known to docker")
	if !mountDenied {
		return ""
	}

	if strings.TrimSpace(tmpBaseDir) != "" && pathWithin(tmpBaseDir, workspaceAbs) {
		return "the --tmp worktree path is not shared with Docker; add it under " +
			"Docker Desktop > Settings > Resources > File Sharing, or set REMUDA_TMP_DIR " +
			"to a shared directory (e.g. one under your home)"
	}
	return "the workspace path is not shared with Docker; add it under " +
		"Docker Desktop > Settings > Resources > File Sharing"
}

// tailBuffer is an io.Writer that retains only the last max bytes written. It
// lets us scan container startup output for a mount-denied error without
// buffering the entire (potentially long-lived) stderr stream of a foreground
// session.
type tailBuffer struct {
	max  int
	data []byte
}

func newTailBuffer(max int) *tailBuffer {
	return &tailBuffer{max: max}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	b.data = append(b.data, p...)
	if len(b.data) > b.max {
		b.data = b.data[len(b.data)-b.max:]
	}
	return n, nil
}

func (b *tailBuffer) String() string {
	return string(b.data)
}
