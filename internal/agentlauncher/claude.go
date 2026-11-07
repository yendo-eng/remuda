package agentlauncher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
)

var claudeSupportedModels = []string{
	"sonnet",
	"opus",
	"fable",
	"claude-fable-5",
	"claude-fable-5-1",
	"claude-fable-5-2",
	"claude-opus-5-5",
	"claude-opus-5",
	"claude-sonnet-5",
	"claude-sonnet-4-6",
	"claude-opus-4-7",
	"claude-opus-4-8",
	"claude-haiku-5-5",
	"claude-haiku-4-5-20251001",
}

// claudeLauncher builds the command string for the Claude Code CLI.
type claudeLauncher struct {
	Model          string
	Yolo           bool
	ReasoningLevel string
	RemoteControl  bool
	RemoteSession  string
	prefix         []string
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
	command.raw(c.prefix...)
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

func (c claudeLauncher) ContainerEnv() map[string]string {
	containerEnv := map[string]string{"ANTHROPIC_API_KEY": ""}
	if c.Yolo {
		// --dangerously-skip-permissions is rejected for root, which
		// containers run as, unless IS_SANDBOX is set.
		containerEnv["IS_SANDBOX"] = "1"
	}
	return containerEnv
}

// ContainerMounts returns docker mount options that expose host Claude
// OAuth/session state in the container, when available.
func (c claudeLauncher) ContainerMounts(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, err := provider.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	return buildClaudeStateMountOpts(logger, home)
}

func buildClaudeStateMountOpts(logger zerolog.Logger, home string) []string {
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")

	var opts []string
	if st, err := os.Stat(claudeDir); err == nil {
		if st.IsDir() {
			opts = append(opts, "-v", claudeDir+":/root/.claude:rw")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warn().Err(err).Str("path", claudeDir).Msg("failed checking Claude state directory; skipping mount")
	}

	if st, err := os.Stat(claudeJSON); err == nil {
		if !st.IsDir() {
			opts = append(opts, "-v", claudeJSON+":/root/.claude.json:rw")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warn().Err(err).Str("path", claudeJSON).Msg("failed checking Claude state file; skipping mount")
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
}
