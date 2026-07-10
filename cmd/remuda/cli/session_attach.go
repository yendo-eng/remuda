package cli

import "github.com/spf13/cobra"

// SessionAttachCmd attaches to an existing session by name.
type SessionAttachCmd struct {
	SessionNamePickOption
}

func (a *app) sessionAttachCmd() *cobra.Command {
	c := &SessionAttachCmd{}
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attach to an existing session.",
		Args:  cobra.NoArgs,
	}
	c.SessionNamePickOption.register(cmd)
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c *SessionAttachCmd) Run(ctx Context) error {
	name, err := c.SessionName(ctx)
	if err != nil {
		return err
	}

	return ctx.Remuda.SessionAttach(name)
}
