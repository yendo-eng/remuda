package session

import (
	"path/filepath"
	"strings"
)

// SessionNameFromWorkspaceName returns a compact tmux session name in the form
// "org/repo/<folder>" based on the provided workspace path, which is expected
// to follow the layout: <baseRoot>/<org>/<repo>/<folder>.
// If the path has fewer than 3 components, it falls back to the last element.
func SessionNameFromWorkspaceName(workspaceName string) string {
	cleaned := filepath.Clean(workspaceName)
	parts := strings.Split(filepath.ToSlash(cleaned), "/")
	if len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return filepath.Base(cleaned)
}

// lastLines returns the last n lines of captured pane output, ignoring
// trailing blank lines. When n is 0, the whole capture is returned.
func lastLines(capture string, n int) string {
	lines := strings.Split(strings.ReplaceAll(capture, "\r\n", "\n"), "\n")
	if n > 0 {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	return strings.Join(lines, "\n")
}

// WithoutOrgPrefix removes the leading org segment from a session name.
// Session names with fewer than 2 slash-separated segments are returned unchanged.
func WithoutOrgPrefix(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return name
	}
	return strings.Join(parts[1:], "/")
}

func FZFPreviewCommand(mgr Multiplexer) string {
	switch mgr.(type) {
	case *tmux:
		// fzf runs preview commands via the user's shell ($SHELL -c ...). Wrap in
		// bash so we can safely default to a reasonable tail length even when
		// fzf doesn't provide FZF_PREVIEW_LINES.
		//
		// Use {}: to target the session's *current* pane, since users may have
		// switched windows/panes after launch.
		return "bash -lc 'term_lines=$(tput lines 2>/dev/null || echo 60); max=$((term_lines*66/100)); if (( max < 10 )); then max=10; fi; lines=${FZF_PREVIEW_LINES:-}; if [[ -z \"$lines\" || ! \"$lines\" =~ ^[0-9]+$ || \"$lines\" -le 0 ]]; then lines=$max; fi; if (( lines > max )); then lines=$max; fi; tmux capture-pane -p -S -$lines -t {}: 2>/dev/null || echo \"<no buffer>\"'"
	case *zellij:
		return ""
	default:
		return ""
	}
}
