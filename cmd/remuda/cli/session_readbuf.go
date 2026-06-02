package cli

import (
	"fmt"
	"strings"
)

// SessionReadbufCmd prints the current pane buffer for a session.
type SessionReadbufCmd struct {
	Name  string `kong:"xor=namepick,help='Session name (org/repo/<name>).',predictor='session-name'"`
	Pick  bool   `kong:"xor=namepick,help='Use fzf to pick a session interactively when name is omitted.'"`
	All   bool   `kong:"xor=namepick,help='Dump all lines from every session, prefixed by session-name:line:.'"`
	Lines int    `name:"lines" short:"n" default:"200" help:"Number of recent lines to print; 0 prints the full buffer."`
}

func (c SessionReadbufCmd) Run(ctx Context) error {
	if c.Lines < 0 {
		return fmt.Errorf("--lines must be greater than or equal to 0")
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
			return fmt.Errorf("read buffer for session %q: %w", sess.Name, err)
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
