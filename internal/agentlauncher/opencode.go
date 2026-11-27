package agentlauncher

import "strings"

// Model IDs are sourced from https://models.dev/api.json.
var openCodeSupportedModels = []string{
	// GitHub Copilot provider models
	"github-copilot/claude-fable-5",
	"github-copilot/claude-fable-5.1",
	"github-copilot/claude-fable-5.2",
	"github-copilot/claude-haiku-4.5",
	"github-copilot/claude-haiku-5.5",
	"github-copilot/claude-opus-4.8",
	"github-copilot/claude-opus-5.5",
	"github-copilot/claude-sonnet-4.6",
	"github-copilot/claude-sonnet-5",
	"github-copilot/claude-sonnet-5.5",
	"github-copilot/gemini-3.8-flash",
	"github-copilot/gemini-4-argon",
	"github-copilot/gemini-4-carbon",
	"github-copilot/gemini-4-hydrogen",
	"github-copilot/gemini-4-neon",
	"github-copilot/gpt-5.3-codex",
	"github-copilot/gpt-5.4-nano",
	"github-copilot/gpt-5.6-luna",
	"github-copilot/gpt-5.6-sol",
	"github-copilot/gpt-6-astra",
	"github-copilot/gpt-6-luna",
	"github-copilot/gpt-6-sol",
	"github-copilot/gpt-6.1-sol",
	"github-copilot/gpt-6.1-terra",
	"github-copilot/gpt-6.2-astra",
	"github-copilot/gpt-6.2-sol",
	"github-copilot/grok-4.5",
	"github-copilot/grok-4.6",
	"github-copilot/grok-4.7",
	"github-copilot/kimi-k3",
	"github-copilot/mai-code-1.1-flash",
	"github-copilot/mai-code-2",
	"github-copilot/mai-code-2-flash",

	// OpenCode Zen
	"opencode/beam-free",
	"opencode/big-pickle",
	"opencode/cat-jam-free",
	"opencode/claude-3-5-haiku",
	"opencode/claude-fable-5",
	"opencode/claude-fable-5-1",
	"opencode/claude-fable-5-2",
	"opencode/claude-haiku-4-5",
	"opencode/claude-haiku-5-5",
	"opencode/claude-opus-4-1",
	"opencode/claude-opus-4-5",
	"opencode/claude-opus-4-6",
	"opencode/claude-opus-4-7",
	"opencode/claude-opus-4-8",
	"opencode/claude-opus-5",
	"opencode/claude-opus-5-5",
	"opencode/claude-sonnet-4",
	"opencode/claude-sonnet-4-5",
	"opencode/claude-sonnet-4-6",
	"opencode/claude-sonnet-5",
	"opencode/claude-sonnet-5-5",
	"opencode/crouton-alpha-free",
	"opencode/deepseek-v4-flash",
	"opencode/deepseek-v4-flash-free",
	"opencode/deepseek-v4-flash-vision-exp",
	"opencode/deepseek-v4-pro",
	"opencode/deepseek-v4.1-flash",
	"opencode/deepseek-v4.1-flash-free",
	"opencode/exo-free",
	"opencode/fledge-alpha-free",
	"opencode/flint-40b-free",
	"opencode/gemini-3-flash",
	"opencode/gemini-3-pro",
	"opencode/gemini-3.1-pro",
	"opencode/gemini-3.5-flash",
	"opencode/gemini-3.5-flash-lite",
	"opencode/gemini-3.6-flash",
	"opencode/gemini-3.7-flash",
	"opencode/gemini-3.8-flash",
	"opencode/gemini-4-argon",
	"opencode/gemini-4-carbon",
	"opencode/gemini-4-hydrogen",
	"opencode/gemini-4-neon",
	"opencode/glm-4.6",
	"opencode/glm-4.7",
	"opencode/glm-4.7-free",
	"opencode/glm-5",
	"opencode/glm-5-free",
	"opencode/glm-5.1",
	"opencode/glm-5.2",
	"opencode/glm-5.3",
	"opencode/glm-5.3-flash",
	"opencode/glm-5.3-flash-free",
	"opencode/gpt-5",
	"opencode/gpt-5-codex",
	"opencode/gpt-5-nano",
	"opencode/gpt-5.1",
	"opencode/gpt-5.1-codex",
	"opencode/gpt-5.1-codex-max",
	"opencode/gpt-5.1-codex-mini",
	"opencode/gpt-5.2",
	"opencode/gpt-5.2-codex",
	"opencode/gpt-5.3-codex",
	"opencode/gpt-5.3-codex-spark",
	"opencode/gpt-5.4",
	"opencode/gpt-5.4-mini",
	"opencode/gpt-5.4-nano",
	"opencode/gpt-5.4-pro",
	"opencode/gpt-5.5",
	"opencode/gpt-5.5-pro",
	"opencode/gpt-5.6-luna",
	"opencode/gpt-5.6-sol",
	"opencode/gpt-5.6-terra",
	"opencode/gpt-6-astra",
	"opencode/gpt-6-luna",
	"opencode/gpt-6-sol",
	"opencode/gpt-6.1-sol",
	"opencode/gpt-6.2-astra",
	"opencode/gpt-6.2-sol",
	"opencode/grok-4.5",
	"opencode/grok-4.6",
	"opencode/grok-4.7",
	"opencode/grok-build-0.1",
	"opencode/grok-code",
	"opencode/hy3-free",
	"opencode/hy3-preview-free",
	"opencode/hy4-flash-free",
	"opencode/iceman-free",
	"opencode/inkling-2-small-free",
	"opencode/kimi-k2",
	"opencode/kimi-k2-thinking",
	"opencode/kimi-k2.5",
	"opencode/kimi-k2.5-free",
	"opencode/kimi-k2.6",
	"opencode/kimi-k2.7-code",
	"opencode/kimi-k3",
	"opencode/kimi-k3.1-pro",
	"opencode/laguna-s-2.1-free",
	"opencode/ling-2.6-flash-free",
	"opencode/ling-3.0-flash-fin-free",
	"opencode/ling-3.0-flash-free",
	"opencode/ling-3.0-tiny-free",
	"opencode/ling-3.1-flash-free",
	"opencode/ll-alpha-free",
	"opencode/longcat-2.0-free",
	"opencode/longcat-2.5-free",
	"opencode/longcat-2.5-preview-free",
	"opencode/mimo-v2-flash-free",
	"opencode/mimo-v2-omni-free",
	"opencode/mimo-v2-pro-free",
	"opencode/mimo-v2.5-free",
	"opencode/mimo-v2.6-flash-free",
	"opencode/minimax-m2.1",
	"opencode/minimax-m2.1-free",
	"opencode/minimax-m2.5",
	"opencode/minimax-m2.5-free",
	"opencode/minimax-m2.7",
	"opencode/minimax-m3",
	"opencode/minimax-m3-free",
	"opencode/minimax-m3.1-flash-free",
	"opencode/mistral-large-4",
	"opencode/muse-ember-1.0",
	"opencode/muse-spark-1.2",
	"opencode/muse-spark-1.2-contributor-free",
	"opencode/muse-spark-1.3",
	"opencode/muse-spark-1.3-contributor-free",
	"opencode/muse-wonder-1.0",
	"opencode/nemotron-3-super-free",
	"opencode/nemotron-3-ultra-free",
	"opencode/nemotron-3.5-lightning-free",
	"opencode/nemotron-3.5-super-vl-free",
	"opencode/north-mini-code-free",
	"opencode/pulse-alpha-free",
	"opencode/qwen3-coder",
	"opencode/qwen3.5-plus",
	"opencode/qwen3.6-plus",
	"opencode/qwen3.6-plus-free",
	"opencode/qwen3.8-flash",
	"opencode/qwen3.8-max",
	"opencode/qwen4-max-free",
	"opencode/ring-2.6-1t-free",
	"opencode/seaglass-alpha-free",
	"opencode/space-bunny-free",
	"opencode/step-5-preview-free",
	"opencode/tidepool-alpha-free",
	"opencode/trinity-large-preview-free",
	"opencode/x-preview-f-free",

	// Google provider models
	"google/deep-research-max-preview-04-2026",
	"google/deep-research-preview-04-2026",
	"google/gemini-2.5-flash-lite",
	"google/gemini-3-flash-preview",
	"google/gemini-3.1-pro-preview",
	"google/gemini-3.1-pro-preview-customtools",
	"google/gemini-3.5-flash-lite",
	"google/gemini-3.6-flash",
	"google/gemini-3.8-flash",
	"google/gemini-4-argon",
	"google/gemini-4-carbon",
	"google/gemini-4-hydrogen",
	"google/gemini-4-neon",
	"google/gemini-flash-latest",
	"google/gemini-flash-lite-latest",
	"google/gemini-pro-latest",
	"google/gemma-4-26b-a4b-it",
	"google/gemma-4-31b-it",

	// OpenAI provider models
	"openai/gpt-5.2",
	"openai/gpt-5.2-pro",
	"openai/gpt-5.4",
	"openai/gpt-5.4-mini",
	"openai/gpt-5.4-pro",
	"openai/gpt-5.5",
	"openai/gpt-5.5-pro",
	"openai/gpt-5.6",
	"openai/gpt-5.6-luna",
	"openai/gpt-5.6-sol",
	"openai/gpt-5.6-terra",
	"openai/gpt-6-astra",
	"openai/gpt-6-astra-healthcare",
	"openai/gpt-6-luna",
	"openai/gpt-6-sol",
	"openai/gpt-6.1-sol",
	"openai/gpt-6.1-sol-healthcare",
	"openai/gpt-6.1-terra",
	"openai/gpt-6.2-astra",
	"openai/gpt-6.2-astra-healthcare",
	"openai/gpt-6.2-sol",
	"openai/gpt-6.2-sol-healthcare",
	"openai/gpt-daybreak-blue-latest",
	"openai/gpt-daybreak-red-latest",
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

func (o opencodeLauncher) Arguments(prompt string, extraArgs ...string) []string {
	return o.launch(prompt, extraArgs...).argv()
}

func (o opencodeLauncher) Command(prompt string, extraArgs ...string) string {
	return o.launch(prompt, extraArgs...).command()
}

func (o opencodeLauncher) launch(prompt string, extraArgs ...string) launch {
	command := launch{executable: "opencode"}
	if o.Model != "" && o.Model != ModelAgentDefault {
		command.raw("--model")
		command.quoted(o.Model)
	}
	command.quoted(extraArgs...)
	if strings.TrimSpace(prompt) != "" {
		command.raw("--prompt")
		command.quoted(prompt)
	}
	return command
}

func (o opencodeLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return o, false
}
