// Package enums provides canonical enum values shared across the codebase.
// This is the single source of truth for valid agents and session managers.
package enums

// ValidAgents is the canonical set of valid agent names.
// Used by CLI enum tags and config file validation.
var ValidAgents = []string{"codex", "opencode", "claude", "bash"}

// ValidSessionManagers is the canonical set of valid session manager names.
// Used by CLI and config file validation.
var ValidSessionManagers = []string{"tmux", "zellij"}

// ValidSlugifyReasoningLevels is the canonical list of reasoning effort levels for slugify.
var ValidSlugifyReasoningLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh"}
