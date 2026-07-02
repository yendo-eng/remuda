package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
)

type SessionSendNamePickOption struct {
	Names []string `name:"name" help:"Session name (org/repo/<name>). Repeatable." predictor:"session-name" sep:"none"`
	Pick  bool     `name:"pick" help:"Use fzf to pick one or more sessions."`
}

func (o SessionSendNamePickOption) Validate() error {
	if len(o.Names) == 0 && !o.Pick {
		return pkgerrors.New("--name or --pick is required")
	}
	if len(o.Names) > 0 && o.Pick {
		return pkgerrors.New("--name and --pick cannot be used together")
	}
	return nil
}

func (o SessionSendNamePickOption) SessionNames(ctx Context) ([]string, error) {
	if len(o.Names) > 0 {
		return o.Names, nil
	}
	return pickSessionNames(ctx, true)
}

// SessionSendCmd sends a prompt to one or more running sessions.
type SessionSendCmd struct {
	SessionSendNamePickOption `embed:""`
	Prompt                    string `arg:"" name:"prompt" help:"Prompt to send to the session(s). Use '-' to read from STDIN."`
	NoNewline                 bool   `name:"no-newline" help:"Do not append a trailing newline/Enter after sending the prompt."`
}

func (c *SessionSendCmd) Validate() error {
	return c.SessionSendNamePickOption.Validate()
}

func (c *SessionSendCmd) Run(ctx Context) error {
	names, err := c.SessionNames(ctx)
	if err != nil {
		return err
	}

	prompt, _, err := resolvePromptFromStdin(ctx.Remuda.IO.In, c.Prompt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" {
		return pkgerrors.New("prompt is required")
	}

	return sendPromptToSessions(ctx, names, prompt, !c.NoNewline)
}

func sendPromptToSessions(ctx Context, names []string, prompt string, appendNewline bool) error {
	for _, name := range names {
		if err := ctx.Remuda.SessionSend(name, prompt, appendNewline); err != nil {
			return pkgerrors.Wrapf(err, "send to session %q", name)
		}
	}

	return nil
}
