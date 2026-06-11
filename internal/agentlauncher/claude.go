package agentlauncher

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

var claudeSupportedModels = []string{
	"sonnet",
	"opus",
	"fable",
	"claude-fable-5",
	"claude-sonnet-4-6",
	"claude-opus-4-7",
	"claude-opus-4-8",
}

// claudeLauncher builds the command string for the Claude Code CLI.
type claudeLauncher struct {
	Model          string
	Yolo           bool
	ReasoningLevel string
	RemoteControl  bool
	RemoteSession  string
}

func Claude(model string, yolo bool, reasoningLevel string) AgentLauncher {
	return claudeLauncher{
		Model:          model,
		Yolo:           yolo,
		ReasoningLevel: reasoningLevel,
	}
}

func (c claudeLauncher) Name() string { return "claude" }

func (c claudeLauncher) Command(prompt string) string {
	var b strings.Builder
	b.WriteString("claude")
	if c.Model != "" && c.Model != ModelAgentDefault {
		b.WriteString(" --model ")
		b.WriteString(shellutil.SingleQuote(c.Model))
	}
	if c.Yolo {
		b.WriteString(" --dangerously-skip-permissions")
	}
	if strings.TrimSpace(c.ReasoningLevel) != "" {
		b.WriteString(" --effort ")
		b.WriteString(shellutil.SingleQuote(c.ReasoningLevel))
	}
	if c.RemoteControl {
		b.WriteString(" --remote-control")
		if strings.TrimSpace(c.RemoteSession) != "" {
			b.WriteString(" ")
			b.WriteString(shellutil.SingleQuote(c.RemoteSession))
		}
	}
	if strings.TrimSpace(prompt) != "" {
		if c.RemoteControl && strings.TrimSpace(c.RemoteSession) == "" {
			b.WriteString(" --")
		}
		b.WriteString(" '")
		b.WriteString(shellutil.EscapeSingleQuotes(prompt))
		b.WriteString("'")
	}
	return b.String()
}

func (c claudeLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	c.RemoteControl = true
	c.RemoteSession = sessionName
	return c, true
}

func (c claudeLauncher) SupportedModels() []string {
	return append([]string(nil), claudeSupportedModels...)
}

func (c claudeLauncher) Version() (string, error) {
	return util.RunCmdOutput("claude", "--version")
}
