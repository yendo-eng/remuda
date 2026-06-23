package agentlauncher

import "strings"

func appendExtraArgs(b *strings.Builder, extraArgs []string) {
	for _, arg := range extraArgs {
		b.WriteString(" ")
		b.WriteString(arg)
	}
}
