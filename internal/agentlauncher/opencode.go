package agentlauncher

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

// If you have more configured, dump them with `opencode models` and add them
// here.
var openCodeSupportedModels = []string{
	// GitHub Copilot provider models
	"github-copilot/claude-3.5-sonnet",
	"github-copilot/claude-3.7-sonnet",
	"github-copilot/claude-3.7-sonnet-thought",
	"github-copilot/claude-haiku-4.5",
	"github-copilot/claude-opus-4",
	"github-copilot/claude-opus-4.5",
	"github-copilot/claude-opus-4.6",
	"github-copilot/claude-opus-41",
	"github-copilot/claude-sonnet-4",
	"github-copilot/claude-sonnet-4.5",
	"github-copilot/claude-sonnet-4.6",
	"github-copilot/gemini-2.0-flash-001",
	"github-copilot/gemini-2.5-pro",
	"github-copilot/gemini-3-flash-preview",
	"github-copilot/gemini-3-pro-preview",
	"github-copilot/gemini-3.1-pro-preview",
	"github-copilot/gpt-4.1",
	"github-copilot/gpt-4o",
	"github-copilot/gpt-5",
	"github-copilot/gpt-5-codex",
	"github-copilot/gpt-5-mini",
	"github-copilot/gpt-5.1",
	"github-copilot/gpt-5.1-codex",
	"github-copilot/gpt-5.1-codex-max",
	"github-copilot/gpt-5.1-codex-mini",
	"github-copilot/gpt-5.2",
	"github-copilot/gpt-5.2-codex",
	"github-copilot/gpt-5.3-codex",
	"github-copilot/gpt-5.4",
	"github-copilot/gpt-5.4-mini",
	"github-copilot/grok-code-fast-1",
	"github-copilot/o3",
	"github-copilot/o3-mini",
	"github-copilot/o4-mini",
	"github-copilot/oswe-vscode-prime",

	// OpenCode Zen
	"opencode/big-pickle",
	"opencode/gpt-5-nano",
	"opencode/grok-code",
	"opencode/mimo-v2-flash-free",
	"opencode/minimax-m2.5-free",
	"opencode/nemotron-3-super-free",

	// Google provider models
	"google/gemini-1.5-flash",
	"google/gemini-1.5-flash-8b",
	"google/gemini-1.5-pro",
	"google/gemini-2.0-flash",
	"google/gemini-2.0-flash-lite",
	"google/gemini-2.5-flash",
	"google/gemini-2.5-flash-lite",
	"google/gemini-2.5-pro",
	"google/gemini-3-pro-preview",

	// OpenAI provider models
	"openai/codex-mini-latest",
	"openai/gpt-3.5-turbo",
	"openai/gpt-4",
	"openai/gpt-4-turbo",
	"openai/gpt-4.1",
	"openai/gpt-4.1-mini",
	"openai/gpt-4.1-nano",
	"openai/gpt-4o",
	"openai/gpt-4o-2024-05-13",
	"openai/gpt-4o-2024-08-06",
	"openai/gpt-4o-2024-11-20",
	"openai/gpt-4o-mini",
	"openai/gpt-5",
	"openai/gpt-5-codex",
	"openai/gpt-5-mini",
	"openai/gpt-5-nano",
	"openai/gpt-5-pro",
	"openai/gpt-5.1",
	"openai/gpt-5.1-chat-latest",
	"openai/gpt-5.1-codex",
	"openai/gpt-5.1-codex-max",
	"openai/gpt-5.1-codex-mini",
	"openai/gpt-5.2",
	"openai/gpt-5.2-codex",
	"openai/gpt-5.2-chat-latest",
	"openai/gpt-5.2-pro",
	"openai/gpt-5.3-codex",
	"openai/gpt-5.3-codex-spark",
	"openai/gpt-5.4",
	"openai/gpt-5.4-mini",
	"openai/gpt-5.4-nano",
	"openai/gpt-5.4-pro",
	"openai/o1",
	"openai/o1-mini",
	"openai/o1-preview",
	"openai/o1-pro",
	"openai/o3",
	"openai/o3-deep-research",
	"openai/o3-mini",
	"openai/o3-pro",
	"openai/o4-mini",
	"openai/o4-mini-deep-research",
	"openai/text-embedding-3-large",
	"openai/text-embedding-3-small",
	"openai/text-embedding-ada-002",
}

// OpenCodeSupportedModels returns the supported model identifiers for the OpenCode CLI.
func (opencodeLauncher) SupportedModels() []string {
	return append([]string(nil), openCodeSupportedModels...)
}

// opencodeLauncher builds command for the OpenCode CLI.
type opencodeLauncher struct {
	Model string
}

func OpenCode(model string) AgentLauncher {
	return opencodeLauncher{
		Model: model,
	}
}

func (o opencodeLauncher) Name() string { return "opencode" }

func (o opencodeLauncher) Command(prompt string) string {
	var b strings.Builder
	b.WriteString("opencode")
	if strings.TrimSpace(prompt) != "" {
		b.WriteString(" --prompt '")
		b.WriteString(shellutil.EscapeSingleQuotes(prompt))
		b.WriteString("'")
	}
	if o.Model != "" && o.Model != ModelAgentDefault {
		b.WriteString(" --model ")
		b.WriteString(shellutil.SingleQuote(o.Model))
	}
	return b.String()
}

func (o opencodeLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return o, false
}

func (o opencodeLauncher) MinimumVersion() string { return "v0.4.0" }

func (o opencodeLauncher) Version() (string, error) {
	out, err := util.RunCmdOutput("opencode", "--version")
	if err != nil {
		return "", err
	}

	return out, nil
}
