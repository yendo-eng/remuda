package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// SessionReadbufCmd prints the current pane buffer for a session.
type SessionReadbufCmd struct {
	Name  string
	Pick  bool
	All   bool
	Lines int
}

func (a *app) sessionReadbufCmd() *cobra.Command {
	c := &SessionReadbufCmd{}
	cmd := &cobra.Command{
		Use:   "readbuf",
		Short: "Print the current pane buffer for logs (tail).",
		Args:  cobra.NoArgs,
	}
	fs := cmd.Flags()
	fs.StringVar(&c.Name, "name", "", "Session name (org/repo/<name>).")
	fs.BoolVar(&c.Pick, "pick", false, "Use fzf to pick a session interactively when name is omitted.")
	fs.BoolVar(&c.All, "all", false, "Dump all lines from every session, prefixed by session-name:line:.")
	fs.IntVarP(&c.Lines, "lines", "n", 200, "Number of recent lines to print; 0 prints the full buffer.")
	cmd.MarkFlagsMutuallyExclusive("name", "pick", "all")
	registerSessionNameCompletion(cmd, "name")
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c SessionReadbufCmd) Run(ctx Context) error {
	if c.Lines < 0 {
		return pkgerrors.Errorf("--lines must be greater than or equal to 0")
	}

	if c.All {
		return c.runAll(ctx)
	}

	opt := SessionNamePickOption{Name: c.Name, Pick: c.Pick}
	name, err := opt.SessionName(ctx)
	if err != nil {
		return err
	}

	buf, err := ctx.Remuda.SessionReadBuffer(name, c.Lines)
	if err != nil {
		return err
	}

	ctx.Remuda.IO.OutWrite(buf)
	return nil
}

func (c SessionReadbufCmd) runAll(ctx Context) error {
	// TODO: Consider parallelizing with goroutines per session, streaming results
	// through a channel. Needs perf testing vs current serial read->write approach.
	sessions, err := ctx.Remuda.SessionList()
	if err != nil {
		return err
	}

	for _, sess := range sessions {
		buf, err := ctx.Remuda.SessionReadBuffer(sess.Name, c.Lines)
		if err != nil {
			return pkgerrors.Wrapf(err, "read buffer for session %q", sess.Name)
		}

		lines := strings.Split(buf, "\n")
		for lineNum, line := range lines {
			// Skip empty trailing line from split
			if lineNum == len(lines)-1 && line == "" {
				continue
			}
			ctx.Remuda.IO.Outf("%s:%d: %s\n", sess.Name, lineNum+1, line)
		}
	}

	return nil
}
