package cli

// SessionPathCmd prints the workspace path associated with a session name.
type SessionPathCmd struct {
	SessionNamePickOption `embed:""`
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
