package agentlauncher

import (
	"strings"

	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

func appendExtraArgs(b *strings.Builder, extraArgs []string) {
	for _, arg := range extraArgs {
		b.WriteString(" ")
		b.WriteString(shellutil.SingleQuote(arg))
	}
}
