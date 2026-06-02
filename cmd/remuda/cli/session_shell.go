package cli

// SessionShellCmd drops the user into a shell for a session.
//
// For containerized sessions, it opens a shell inside the running container.
// For non-container sessions (or when the container isn't available), it falls back
// to opening a host shell in the session's workspace directory.
type SessionShellCmd struct {
	SessionNamePickOption `embed:""`
	Host                  bool `name:"host" help:"Open a host shell in the session workspace (skip Docker)."`
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
