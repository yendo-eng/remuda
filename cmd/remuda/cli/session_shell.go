package cli

import "github.com/spf13/cobra"

// SessionShellCmd drops the user into a shell for a session.
//
// For containerized sessions, it opens a shell inside the running container.
// For non-container sessions (or when the container isn't available), it falls back
// to opening a host shell in the session's workspace directory.
type SessionShellCmd struct {
	SessionNamePickOption
	Host bool
}

func (a *app) sessionShellCmd() *cobra.Command {
	c := &SessionShellCmd{}
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open a shell for a session (container when available; use --host to force a host shell in the workspace).",
		Args:  cobra.NoArgs,
	}
	c.SessionNamePickOption.register(cmd)
	cmd.Flags().BoolVar(&c.Host, "host", false, "Open a host shell in the session workspace (skip Docker).")
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c SessionShellCmd) Run(ctx Context) error {
	name, err := c.SessionName(ctx)
	if err != nil {
		return err
	}

	if c.Host {
		return ctx.Remuda.SessionHostShell(name)
	}
	return ctx.Remuda.SessionShell(name)
}
