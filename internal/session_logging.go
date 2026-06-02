package internal

import "github.com/rs/zerolog"

type creatingSessionLogContext struct {
	Workspace     string
	Session       string
	Agent         string
	Detached      bool
	Container     bool
	ContainerName string
	UsePromptIDs  []string
}

func logCreatingSession(logger zerolog.Logger, ctx creatingSessionLogContext) {
	event := logger.Info().
		Str("workspace", ctx.Workspace).
		Str("session", ctx.Session).
		Str("agent", ctx.Agent).
		Bool("detached", ctx.Detached).
		Bool("container", ctx.Container).
		Str("container_name", ctx.ContainerName)
	if len(ctx.UsePromptIDs) > 0 {
		event = event.Strs("use_prompt_ids", ctx.UsePromptIDs)
	}
	event.Msg("creating session")
}
