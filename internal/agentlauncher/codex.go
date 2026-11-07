package agentlauncher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

// codexLauncher builds the command string for the Codex CLI.
type codexLauncher struct {
	Model string
	Yolo  bool
	// ReasoningLevel is the Codex reasoning effort level (if set).
	ReasoningLevel string
	prefix         []string
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
	command.raw(c.prefix...)
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
		"gpt-6-astra",
		"gpt-6-sol",
		"gpt-6-luna",
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

func (c codexLauncher) ContainerEnv() map[string]string { return nil }

// ContainerMounts prepares Docker volume mount options for the Codex CLI.
// It forwards authentication artifacts (auth.json + config.toml) plus ~/.codex/prompts, ~/.codex/rules,
// ~/.codex/skills, and ~/.codex/AGENTS.md whenever they are available so custom instructions and slash
// command state work inside --container sessions.
// If the OPENAI_API_KEY is not set or any file operations fail, it returns only the mounts that can be derived
// (for example the prompts directory) and logs warnings for auth-related failures.
func (c codexLauncher) ContainerMounts(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	var containerOpts []string
	apiKeySet := strings.TrimSpace(provider.Getenv("OPENAI_API_KEY")) != ""
	var accountAuthMount []string
	if !apiKeySet {
		accountAuthMount = codexAccountAuthMountOptions(provider)
	}
	promptMount := codexPromptsMountOptions(provider)
	rulesMount := codexRulesMountOptions(provider)
	skillsMount := codexSkillsMountOptions(logger, provider)
	agentsMount := codexAgentsMountOptions(provider)
	stateMount := codexStateMountOptions(logger, provider)
	promptsApplied := false
	rulesApplied := false
	skillsApplied := false
	agentsApplied := false
	stateApplied := false
	if len(accountAuthMount) > 0 {
		containerOpts = append(containerOpts, accountAuthMount...)
		stateApplied = true
		agentsApplied = true
		skillsApplied = true
		rulesApplied = true
		promptsApplied = true
	}
	if key := provider.Getenv("OPENAI_API_KEY"); strings.TrimSpace(key) != "" {
		if tmpDir, tmpErr := os.MkdirTemp("", "codex-auth-*"); tmpErr == nil {
			_ = os.MkdirAll(tmpDir, 0o755)
			authPath := filepath.Join(tmpDir, "auth.json")
			payload := fmt.Sprintf("{\"OPENAI_API_KEY\":%q}\n", strings.TrimSpace(key))
			if writeErr := os.WriteFile(authPath, []byte(payload), 0o600); writeErr == nil {
				if home, herr := provider.UserHomeDir(); herr == nil {
					cfgSrc := filepath.Join(home, ".codex", "config.toml")
					if data, rerr := os.ReadFile(cfgSrc); rerr == nil {
						cfgDst := filepath.Join(tmpDir, "config.toml")
						//nolint:gosec // G703: cfgDst is a fixed temp path under tmpDir created in this function.
						if werr := os.WriteFile(cfgDst, data, 0o644); werr != nil {
							logger.Warn().Err(werr).Msg("failed copying ~/.codex/config.toml into container mount")
						}
					}
				}
				opts := []string{
					"--tmpfs", "/root/.codex:rw,mode=0755",
					"-v", authPath + ":/root/.codex/auth.json:ro",
				}
				if len(stateMount) > 0 {
					opts = append(opts, stateMount...)
					stateApplied = true
				}
				cfgDst := filepath.Join(tmpDir, "config.toml")
				if _, statErr := os.Stat(cfgDst); statErr == nil {
					opts = append(opts, "-v", cfgDst+":/root/.codex/config.toml:ro")
				}
				if len(promptMount) > 0 {
					opts = append(opts, promptMount...)
					promptsApplied = true
				}
				if len(rulesMount) > 0 {
					opts = append(opts, rulesMount...)
					rulesApplied = true
				}
				if len(skillsMount) > 0 {
					opts = append(opts, skillsMount...)
					skillsApplied = true
				}
				if len(agentsMount) > 0 {
					opts = append(opts, agentsMount...)
					agentsApplied = true
				}
				containerOpts = append(opts, containerOpts...)
			} else {
				logger.Warn().Err(writeErr).Msg("failed writing Codex auth.json; continuing without mount")
			}
		} else {
			logger.Warn().Err(tmpErr).Msg("failed creating temp dir for Codex auth; continuing without mount")
		}
	}
	if len(stateMount) > 0 && !stateApplied {
		containerOpts = append(stateMount, containerOpts...)
	}
	if len(agentsMount) > 0 && !agentsApplied {
		containerOpts = append(agentsMount, containerOpts...)
	}
	if len(skillsMount) > 0 && !skillsApplied {
		containerOpts = append(skillsMount, containerOpts...)
	}
	if len(rulesMount) > 0 && !rulesApplied {
		containerOpts = append(rulesMount, containerOpts...)
	}
	if len(promptMount) > 0 && !promptsApplied {
		containerOpts = append(promptMount, containerOpts...)
	}
	return containerOpts
}

// codexPromptsMountOptions returns a read-only bind mount for ~/.codex/prompts when present.
func codexPromptsMountOptions(provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}
	promptsDir := filepath.Join(home, ".codex", "prompts")
	info, err := os.Stat(promptsDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	return []string{"-v", promptsDir + ":/root/.codex/prompts:ro"}
}

// codexRulesMountOptions returns a read-only bind mount for ~/.codex/rules when present.
func codexRulesMountOptions(provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}
	rulesPath := filepath.Join(home, ".codex", "rules")
	info, err := os.Stat(rulesPath)
	if err != nil {
		return nil
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil
	}
	return []string{"-v", rulesPath + ":/root/.codex/rules:ro"}
}

// codexSkillsMountOptions returns a read-only bind mount for ~/.codex/skills when present.
func codexSkillsMountOptions(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}
	skillsDir := filepath.Join(home, ".codex", "skills")
	info, err := os.Stat(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug().Str("path", skillsDir).Msg("codex skills dir missing; skipping mount")
		}
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	return []string{"-v", skillsDir + ":/root/.codex/skills:ro"}
}

// codexAgentsMountOptions returns a read-only bind mount for ~/.codex/AGENTS.md when present.
func codexAgentsMountOptions(provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}
	agentsPath := filepath.Join(home, ".codex", "AGENTS.md")
	info, err := os.Stat(agentsPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []string{"-v", agentsPath + ":/root/.codex/AGENTS.md:ro"}
}

// codexAccountAuthMountOptions returns a bind mount for the whole host ~/.codex directory,
// used only on the account-login path (OPENAI_API_KEY unset) when ~/.codex/auth.json exists.
// The whole directory is mounted, rather than just auth.json, because Codex refreshes its
// access token by writing a temp file and atomically renaming it over auth.json; a single-file
// bind mount would not survive that rename. Mounting the whole directory also covers
// history.jsonl, sessions, prompts, rules, skills, and AGENTS.md, so callers should skip those
// individual sub-mounts when this one applies.
func codexAccountAuthMountOptions(provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}
	codexDir := filepath.Join(home, ".codex")
	authPath := filepath.Join(codexDir, "auth.json")
	info, err := os.Stat(authPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return []string{"-v", codexDir + ":/root/.codex:rw"}
}

func codexStateMountOptions(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, herr := provider.UserHomeDir()
	if herr != nil {
		return nil
	}

	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		logger.Warn().Err(err).Msg("failed ensuring ~/.codex exists; continuing without Codex state mounts")
		return nil
	}

	var opts []string

	historyPath := filepath.Join(codexDir, "history.jsonl")
	if err := ensureRegularFile(historyPath, 0o600); err != nil {
		logger.Warn().Err(err).Msg("failed ensuring ~/.codex/history.jsonl exists; continuing without Codex history mount")
	} else {
		opts = append(opts, "-v", historyPath+":/root/.codex/history.jsonl:rw")
	}

	sessionsDir := filepath.Join(codexDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		logger.Warn().Err(err).Msg("failed ensuring ~/.codex/sessions exists; continuing without Codex sessions mount")
	} else {
		opts = append(opts, "-v", sessionsDir+":/root/.codex/sessions:rw")
	}

	return opts
}

func ensureRegularFile(path string, perm os.FileMode) error {
	st, err := os.Stat(path)
	if err == nil {
		if st.Mode().IsRegular() {
			return nil
		}
		return pkgerrors.Errorf("%s exists but is not a regular file", path)
	}
	if !os.IsNotExist(err) {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return f.Close()
}
