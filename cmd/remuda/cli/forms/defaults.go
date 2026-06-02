package forms

import (
	"github.com/charmbracelet/huh"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
)

const DefaultWidth = 72

// Sugar over huh.NewForm(huh.NewGroup(fields...)) with usual defaults.
func New(fields ...huh.Field) *huh.Form {
	form := huh.
		NewForm(huh.NewGroup(fields...)).
		WithWidth(DefaultWidth).
		WithShowHelp(false)
	return form
}

const UserSelectedCustomAgent = "__custom_agent__"

// builds a Select for one of our supported agents, or a custom command.
func AgentSelect(into *string) *huh.Select[string] {
	options := []huh.Option[string]{}
	for _, agent := range agentlauncher.SupportedAgents() {
		options = append(options, huh.NewOption(string(agent), string(agent)))
	}
	options = append(options, huh.NewOption("Custom command", UserSelectedCustomAgent))
	return huh.NewSelect[string]().
		Title("Agent").
		Description("Agent program to use").
		Options(options...).
		Value(into)
}
