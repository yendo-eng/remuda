package cli

import (
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// SessionInactiveCmd prints inactive workspace paths (one per line).
type SessionInactiveCmd struct{}

func (a *app) sessionInactiveCmd() *cobra.Command {
	c := &SessionInactiveCmd{}
	cmd := &cobra.Command{
		Use:   "inactive",
		Short: "Print inactive workspace paths (no active session), one per line.",
		Args:  cobra.NoArgs,
	}
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c SessionInactiveCmd) Run(ctx Context) error {
	inactive, err := ctx.Remuda.SessionInactive()
	if err != nil {
		return pkgerrors.Wrap(err, "session inactive")
	}

	for _, ws := range inactive {
		ctx.Remuda.IO.Outln(ws)
	}

	return nil
}
