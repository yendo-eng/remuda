package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal/session"
)

// SessionListCmd lists sessions. It filters for Remuda-style names by default.
type SessionListCmd struct {
	JSON  bool
	NoOrg bool
}

func (a *app) sessionListCmd() *cobra.Command {
	c := &SessionListCmd{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active sessions created by Remuda.",
		Args:  cobra.NoArgs,
	}
	cmd.Flags().BoolVar(&c.JSON, "json", false, "Emit JSON instead of plain text.")
	cmd.Flags().BoolVar(&c.NoOrg, "no-org", false, "Omit the leading org segment from session names.")
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
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
		name := s.Name
		if c.NoOrg {
			name = session.WithoutOrgPrefix(name)
		}
		ctx.Remuda.IO.Outf("%s\n", name)
	}

	return nil
}
