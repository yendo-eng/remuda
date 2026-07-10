package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type SessionSendNamePickOption struct {
	Names []string
	Pick  bool
}

func (o *SessionSendNamePickOption) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringArrayVar(&o.Names, "name", nil, "Session name (org/repo/<name>). Repeatable.")
	fs.BoolVar(&o.Pick, "pick", false, "Use fzf to pick one or more sessions.")
	registerSessionNameCompletion(cmd, "name")
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
	SessionSendNamePickOption
	Prompt    string
	NoNewline bool
}

func (a *app) sessionSendCmd() *cobra.Command {
	c := &SessionSendCmd{}
	cmd := &cobra.Command{
		Use:   "send <prompt>",
		Short: "Send a prompt to a running session. Use '-' to read the prompt from STDIN.",
		Args:  cobra.ExactArgs(1),
	}
	c.SessionSendNamePickOption.register(cmd)
	cmd.Flags().BoolVar(&c.NoNewline, "no-newline", false, "Do not append a trailing newline/Enter after sending the prompt.")
	return a.simpleCmd(cmd, nil, func(args []string) error {
		c.Prompt = args[0]
		if err := c.Validate(); err != nil {
			return err
		}
		return c.Run(*a.kctx)
	})
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
