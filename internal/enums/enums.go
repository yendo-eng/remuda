// Package enums provides canonical enum values shared across the codebase.
// This is the single source of truth for valid agents and session managers.
package enums

import "strings"

// ValidAgents is the canonical set of valid agent names.
// Used by CLI enum tags and config file validation.
var ValidAgents = []string{"codex", "opencode", "claude", "bash"}

// ValidAgentsEnum returns the agents as a comma-separated string for Kong enum tags.
func ValidAgentsEnum() string {
	return strings.Join(ValidAgents, ",")
}

// ValidSessionManagers is the canonical set of valid session manager names.
// Used by CLI and config file validation.
var ValidSessionManagers = []string{"tmux", "zellij"}

// ValidSessionManagersEnum returns the session managers as a comma-separated string for Kong enum tags.
func ValidSessionManagersEnum() string {
	return strings.Join(ValidSessionManagers, ",")
}

// ValidSlugifyReasoningLevels is the canonical list of reasoning effort levels for slugify.
var ValidSlugifyReasoningLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh"}

// ValidSlugifyReasoningLevelsEnum returns the slugify reasoning levels as a comma-separated string for Kong enum tags.
func ValidSlugifyReasoningLevelsEnum() string {
	return strings.Join(ValidSlugifyReasoningLevels, ",")
}
