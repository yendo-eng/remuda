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

func (c codexLauncher) Command(prompt string) string {
	var b strings.Builder
	b.WriteString("codex")
	if c.Yolo {
		b.WriteString(" --dangerously-bypass-approvals-and-sandbox")
		// Allow env passthrough inside Codex's sandbox in yolo mode
		b.WriteString(" --config shell_environment_policy.ignore_default_excludes=\"true\"")
	}
	if c.Model != "" && c.Model != ModelAgentDefault {
		b.WriteString(" --model ")
		b.WriteString(shellutil.SingleQuote(c.Model))
	}
	if strings.TrimSpace(c.ReasoningLevel) != "" {
		b.WriteString(" --config model_reasoning_effort=")
		b.WriteString(shellutil.SingleQuote(c.ReasoningLevel))
	}
	if strings.TrimSpace(prompt) != "" {
		b.WriteString(" -- '")
		b.WriteString(shellutil.EscapeSingleQuotes(prompt))
		b.WriteString("'")
	}
	return b.String()
}

func (c codexLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return c, false
}

// Not an exhaustive list, nor is this guaranteed to be up to date.
func (c codexLauncher) SupportedModels() []string {
	return []string{
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
