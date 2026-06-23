package agentlauncher

import (
	"strings"

	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

// customLauncher prepends a user-provided command.
type customLauncher struct {
	Cmd string
}

func Custom(cmd string) AgentLauncher {
	return customLauncher{
		Cmd: cmd,
	}
}

func (c customLauncher) Name() string { return "custom" }

func (c customLauncher) Command(prompt string, extraArgs ...string) string {
	if strings.TrimSpace(prompt) == "" {
		return c.Cmd
	}

	var b strings.Builder
	b.WriteString(c.Cmd)
	b.WriteString(" '")
	b.WriteString(shellutil.EscapeSingleQuotes(prompt))
	b.WriteString("'")
	return b.String()
}

func (c customLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return c, false
}

func (c customLauncher) SupportedModels() []string { return nil }

func (c customLauncher) Version() (string, error) {
	return "", nil
}
