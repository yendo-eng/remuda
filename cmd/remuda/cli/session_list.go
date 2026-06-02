package cli

import "encoding/json"

// SessionListCmd lists sessions. It filters for Remuda-style names by default.
type SessionListCmd struct {
	JSON bool `name:"json" help:"Emit JSON instead of plain text."`
}

func (c *SessionListCmd) Run(ctx Context) error {
	sessions, err := ctx.Remuda.SessionList()
	if err != nil {
		return err
	}

	if c.JSON {
		enc := json.NewEncoder(ctx.Remuda.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		ctx.Remuda.IO.Outf("No Remuda sessions found. (%s)\n", ctx.Remuda.Session.Name())
		return nil
	}

	for _, s := range sessions {
		ctx.Remuda.IO.Outf("%s\n", s.Name)
	}

	return nil
}
