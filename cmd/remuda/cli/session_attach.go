package cli

// SessionAttachCmd attaches to an existing session by name.
type SessionAttachCmd struct {
	SessionNamePickOption `embed:""`
}

func (c *SessionAttachCmd) Run(ctx Context) error {
	name, err := c.SessionName(ctx)
	if err != nil {
		return err
	}

	return ctx.Remuda.SessionAttach(name)
}
