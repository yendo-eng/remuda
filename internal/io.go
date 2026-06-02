package internal

import (
	"fmt"
	"io"
	"os"
)

// IO wraps the standard streams used by Remuda commands.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// DefaultIO returns an IO using the process standard streams.
func DefaultIO() IO {
	return IO{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

// IsTerminal returns true if both stdin and stdout are terminals.
func (i IO) IsTerminal() bool {
	fiOut, ok := i.Out.(*os.File)
	if !ok {
		return false
	}
	fiIn, ok := i.In.(*os.File)
	if !ok {
		return false
	}

	fiOutInfo, _ := fiOut.Stat()
	fiInInfo, _ := fiIn.Stat()
	return (fiOutInfo.Mode()&os.ModeCharDevice) != 0 && (fiInInfo.Mode()&os.ModeCharDevice) != 0
}

// Outf writes formatted output to stdout. Errors are intentionally ignored to match fmt helpers.
func (i IO) Outf(format string, args ...any) {
	_, _ = fmt.Fprintf(i.Out, format, args...)
}

// Outln writes a newline-terminated line to stdout. Errors are intentionally ignored to match fmt helpers.
func (i IO) Outln(args ...any) {
	_, _ = fmt.Fprintln(i.Out, args...)
}

// OutWrite writes raw output to stdout without additional formatting. Errors are intentionally ignored to match fmt helpers.
func (i IO) OutWrite(args ...any) {
	_, _ = fmt.Fprint(i.Out, args...)
}

// Errf writes formatted output to stderr. Errors are intentionally ignored to match fmt helpers.
func (i IO) Errf(format string, args ...any) {
	_, _ = fmt.Fprintf(i.Err, format, args...)
}

// Errln writes a newline-terminated line to stderr. Errors are intentionally ignored to match fmt helpers.
func (i IO) Errln(args ...any) {
	_, _ = fmt.Fprintln(i.Err, args...)
}

// ErrWrite writes raw output to stderr without additional formatting. Errors are intentionally ignored to match fmt helpers.
func (i IO) ErrWrite(args ...any) {
	_, _ = fmt.Fprint(i.Err, args...)
}
