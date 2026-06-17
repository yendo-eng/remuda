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

// SanitizeTmuxSessionToken mirrors tmux's tendency to map dots to underscores
// in session names on some systems.
func SanitizeTmuxSessionToken(s string) string {
	return strings.ReplaceAll(s, ".", "_")
}

func FZFPreviewCommand(mgr SessionManager) string {
	switch mgr.(type) {
	case *defaultTmuxManager:
		// fzf runs preview commands via the user's shell ($SHELL -c ...). Wrap in
		// bash so we can safely default to a reasonable tail length even when
		// fzf doesn't provide FZF_PREVIEW_LINES.
		//
		// Use {}: to target the session's *current* pane, since users may have
		// switched windows/panes after launch.
		return "bash -lc 'term_lines=$(tput lines 2>/dev/null || echo 60); max=$((term_lines*66/100)); if (( max < 10 )); then max=10; fi; lines=${FZF_PREVIEW_LINES:-}; if [[ -z \"$lines\" || ! \"$lines\" =~ ^[0-9]+$ || \"$lines\" -le 0 ]]; then lines=$max; fi; if (( lines > max )); then lines=$max; fi; tmux capture-pane -p -S -$lines -t {}: 2>/dev/null || echo \"<no buffer>\"'"
	case *zellijManager:
		return ""
	default:
		return ""
	}
}
