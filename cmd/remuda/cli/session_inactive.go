package cli

import "github.com/pkg/errors"

// SessionInactiveCmd prints inactive workspace paths (one per line).
type SessionInactiveCmd struct{}

func (c SessionInactiveCmd) Run(ctx Context) error {
	inactive, err := ctx.Remuda.SessionInactive()
	if err != nil {
		return errors.Wrap(err, "session inactive")
	}

	for _, ws := range inactive {
		ctx.Remuda.IO.Outln(ws)
	}

	return nil
}
