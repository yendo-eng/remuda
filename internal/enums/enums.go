// Package enums provides canonical enum values shared across the codebase.
// This is the single source of truth for valid agents and session managers.
package enums

// ValidAgents is the canonical set of valid agent names.
// Used by CLI enum tags and config file validation.
var ValidAgents = []string{"codex", "opencode", "claude", "bash"}

// ValidSessionManagers is the canonical set of valid session manager names.
// Used by CLI and config file validation.
var ValidSessionManagers = []string{"tmux", "zellij"}

const (
	UsePromptPositionBefore = "before"
	UsePromptPositionAfter  = "after"
)

// ValidUsePromptPositions is the canonical set of saved-prompt placements.
// Used by CLI enum tags and config file validation.
var ValidUsePromptPositions = []string{UsePromptPositionBefore, UsePromptPositionAfter}

// ValidSlugifyReasoningLevels is the canonical list of reasoning effort levels for slugify.
// Slugify uses a lightweight title-generation call, so it intentionally excludes max and ultra.
var ValidSlugifyReasoningLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh"}
