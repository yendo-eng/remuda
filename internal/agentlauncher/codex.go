package agentlauncher

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

// codexLauncher builds the command string for the Codex CLI.
type codexLauncher struct {
	Model string
	Yolo  bool
	// ReasoningLevel is the Codex reasoning effort level (if set).
	ReasoningLevel string
}

func Codex(model string, yolo bool, reasoningLevel string) AgentLauncher {
	return codexLauncher{
		Model:          model,
		Yolo:           yolo,
		ReasoningLevel: reasoningLevel,
	}
}

func (c codexLauncher) Name() string { return "codex" }

func (c codexLauncher) Arguments(prompt string, extraArgs ...string) []string {
	return c.launch(prompt, extraArgs...).argv()
}

func (c codexLauncher) Command(prompt string, extraArgs ...string) string {
	return c.launch(prompt, extraArgs...).command()
}

func (c codexLauncher) launch(prompt string, extraArgs ...string) launch {
	command := launch{executable: "codex"}
	if c.Yolo {
		command.raw(
			"--dangerously-bypass-approvals-and-sandbox",
			"--dangerously-bypass-hook-trust",
		)
		command.raw("--config")
		command.rendered("shell_environment_policy.ignore_default_excludes=true", "shell_environment_policy.ignore_default_excludes=\"true\"")
	}
	if c.Model != "" && c.Model != ModelAgentDefault {
		command.raw("--model")
		command.quoted(c.Model)
	}
	if strings.TrimSpace(c.ReasoningLevel) != "" {
		command.raw("--config")
		command.rendered("model_reasoning_effort="+c.ReasoningLevel, "model_reasoning_effort="+shellutil.SingleQuote(c.ReasoningLevel))
	}
	command.quoted(extraArgs...)
	if strings.TrimSpace(prompt) != "" {
		command.raw("--")
		command.quoted(prompt)
	}
	return command
}

func (c codexLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return c, false
}

// Not an exhaustive list, nor is this guaranteed to be up to date.
func (c codexLauncher) SupportedModels() []string {
	return []string{
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.3-codex",
		"gpt-5.2-codex",
		"gpt-5.1-codex-max",
		"gpt-5.1-codex-mini",
		"gpt-5.2",
		"gpt-5-codex",
		"gpt-5-codex-mini",
		"gpt-5.1-codex",
		"gpt-5",
		"gpt-5.1",
		"codex-mini-latest",
	}
}

func (c codexLauncher) Version() (string, error) {
	out, err := util.RunCmdOutput("codex", "--version")
	if err != nil {
		return "", err
	}

	// The output may have junk in it like "codex" - just extract the semver.
	found := util.SemverRegex.FindString(out)

	// Add a "v" prefix if missing.
	if !strings.HasPrefix(found, "v") {
		found = "v" + found
	}

	return found, nil
}
