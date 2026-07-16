package internal

import (
	"strings"
	"unicode"

	"github.com/yendo-eng/remuda/internal/titletemplate"
)

func (k Remuda) SessionAttach(name string) error {
	if title, ok := titletemplate.Render(k.Config.TerminalTitle, name); ok && title != "" {
		k.IO.ErrWrite(terminalTitleSequence(title))
	}
	return k.Session.Attach(name)
}

func terminalTitleSequence(title string) string {
	const prefix = "\x1b]2;"
	const suffix = "\x07"

	var b strings.Builder
	b.Grow(len(title))
	for _, r := range title {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}

	return prefix + b.String() + suffix
}
