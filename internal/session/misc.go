package session

import (
	"path/filepath"
	"strings"
)

// SessionNameFromWorkspaceName returns a compact session name in the form
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
