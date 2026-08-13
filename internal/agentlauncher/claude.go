package agentlauncher

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
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

func (c claudeLauncher) Arguments(prompt string, extraArgs ...string) []string {
	return c.launch(prompt, extraArgs...).argv()
}

func (c claudeLauncher) Command(prompt string, extraArgs ...string) string {
	return c.launch(prompt, extraArgs...).command()
}

func (c claudeLauncher) launch(prompt string, extraArgs ...string) launch {
	command := launch{executable: "claude"}
	if c.Model != "" && c.Model != ModelAgentDefault {
		command.raw("--model")
		command.quoted(c.Model)
	}
	if c.Yolo {
		command.raw("--dangerously-skip-permissions")
	}
	if strings.TrimSpace(c.ReasoningLevel) != "" {
		command.raw("--effort")
		command.quoted(c.ReasoningLevel)
	}
	if c.RemoteControl {
		command.raw("--remote-control")
		if strings.TrimSpace(c.RemoteSession) != "" {
			command.quoted(c.RemoteSession)
		}
	}
	command.quoted(extraArgs...)
	if strings.TrimSpace(prompt) != "" {
		if c.RemoteControl && strings.TrimSpace(c.RemoteSession) == "" {
			command.raw("--")
		}
		command.quoted(prompt)
	}
	return command
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
