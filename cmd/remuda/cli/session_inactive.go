package cli

import "github.com/pkg/errors"

// SessionInactiveCmd prints inactive workspace paths (one per line).
type SessionInactiveCmd struct {
	IncludeTmp bool `name:"include-tmp" help:"Also scan the OS-temp root for --tmp session worktrees (hidden by default)."`
}

func (c SessionInactiveCmd) Run(ctx Context) error {
	inactive, err := ctx.Remuda.SessionInactiveWithOptions(c.IncludeTmp)
	if err != nil {
		return errors.Wrap(err, "session inactive")
	}

	for _, ws := range inactive {
		ctx.Remuda.IO.Outln(ws)
	}

	return nil
}
