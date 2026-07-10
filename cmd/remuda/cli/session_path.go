package cli

import "github.com/spf13/cobra"

// SessionPathCmd prints the workspace path associated with a session name.
type SessionPathCmd struct {
	SessionNamePickOption
}

func (a *app) sessionPathCmd() *cobra.Command {
	c := &SessionPathCmd{}
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print the absolute workspace path for a session.",
		Args:  cobra.NoArgs,
	}
	c.SessionNamePickOption.register(cmd)
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c *SessionPathCmd) Run(ctx Context) error {
	name, err := c.SessionName(ctx)
	if err != nil {
		return err
	}

	path, err := ctx.Remuda.SessionWorkspacePath(name)
	if err != nil {
		return err
	}

	ctx.Remuda.IO.Outln(path)
	return nil
}
