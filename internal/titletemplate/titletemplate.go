// Package titletemplate renders the terminal-title template applied on
// session attach. It is a dependency-free leaf package so both
// internal/configfile (validation) and the internal package (rendering) can
// import it without a cycle.
package titletemplate

import (
	"regexp"
	"slices"
	"strings"

	pkgerrors "github.com/pkg/errors"
)

// Default is the template used when session.terminal_title is unset,
// preserving the pre-existing org/repo/name title.
const Default = "{org}/{repo}/{name}"

// Off disables title-setting when the template equals this literal
// (case-insensitive).
const Off = "off"

// ValidPlaceholders are the only substitutions a template may reference.
var ValidPlaceholders = []string{"org", "repo", "name"}

var placeholderPattern = regexp.MustCompile(`\{[^{}]*\}`)

// IsOff reports whether template disables title-setting.
func IsOff(template string) bool {
	return strings.EqualFold(strings.TrimSpace(template), Off)
}

// Validate rejects templates that reference an unknown placeholder.
func Validate(template string) error {
	if IsOff(template) {
		return nil
	}
	for _, match := range placeholderPattern.FindAllString(template, -1) {
		name := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		if !slices.Contains(ValidPlaceholders, name) {
			return pkgerrors.Errorf("unknown placeholder %q (valid: {org}, {repo}, {name})", match)
		}
	}
	return nil
}

// Render substitutes {org}, {repo}, {name} in template with the
// corresponding segment of sessionName (org/repo/name). It reports ok=false
// when the template is "off" (title-setting disabled). If sessionName isn't
// exactly 3 segments, it falls back to sessionName unchanged rather than
// rendering a partial/empty title.
func Render(template, sessionName string) (title string, ok bool) {
	if IsOff(template) {
		return "", false
	}
	if template == "" {
		template = Default
	}

	parts := strings.Split(sessionName, "/")
	if len(parts) != 3 {
		return sessionName, true
	}

	replacer := strings.NewReplacer(
		"{org}", parts[0],
		"{repo}", parts[1],
		"{name}", parts[2],
	)
	return replacer.Replace(template), true
}
