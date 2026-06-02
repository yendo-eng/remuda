package internal

import (
	"strings"
	"unicode"
)

func (k Remuda) SessionAttach(name string) error {
	k.IO.ErrWrite(terminalTitleSequence(name))
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
