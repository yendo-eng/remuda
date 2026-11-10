package agentlauncher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

func (c codexLauncher) ContainerMounts(logger zerolog.Logger, provider env.Provider) []string {
	home, ok := hostHome(provider)
	if !ok {
		return nil
	}
	codexDir := filepath.Join(home, ".codex")
	apiKey := strings.TrimSpace(provider.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		if info, err := os.Stat(filepath.Join(codexDir, "auth.json")); err == nil && info.Mode().IsRegular() {
			// The whole directory is mounted, rather than just auth.json, because
			// Codex access tokens are refreshed by atomically renaming a temp file
			// over auth.json, which can't be done over a single-file bind mount.
			// The directory mount also includes everything mounted individually
			// below.
			return []string{"-v", codexDir + ":/root/.codex:rw"}
		}
	}

	customizationMounts := slices.Concat(
		bindIfExists(logger, filepath.Join(codexDir, "prompts"), "/root/.codex/prompts", "ro", fs.FileMode.IsDir),
		bindIfExists(logger, filepath.Join(codexDir, "rules"), "/root/.codex/rules", "ro", func(m fs.FileMode) bool { return m.IsDir() || m.IsRegular() }),
		bindIfExists(logger, filepath.Join(codexDir, "skills"), "/root/.codex/skills", "ro", fs.FileMode.IsDir),
		bindIfExists(logger, filepath.Join(codexDir, "AGENTS.md"), "/root/.codex/AGENTS.md", "ro", fs.FileMode.IsRegular),
	)
	if apiKey != "" {
		tmpDir, err := writeCodexAPIKeyAuth(logger, apiKey, codexDir)
		if err == nil {
			return slices.Concat(
				[]string{
					"--tmpfs", "/root/.codex:rw,mode=0755",
					"-v", filepath.Join(tmpDir, "auth.json") + ":/root/.codex/auth.json:ro",
				},
				codexStateMountOptions(logger, codexDir),
				bindIfExists(logger, filepath.Join(tmpDir, "config.toml"), "/root/.codex/config.toml", "ro", fs.FileMode.IsRegular),
				customizationMounts,
			)
		}
		logger.Warn().Err(err).Msg("failed preparing Codex auth.json; continuing without mount")
	}
	return append(customizationMounts, codexStateMountOptions(logger, codexDir)...)
}

// writeCodexAPIKeyAuth writes an auth.json holding apiKey, plus a copy of the
// host's config.toml when there is one, into a new temp dir and returns it.
func writeCodexAPIKeyAuth(logger zerolog.Logger, apiKey, codexDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "codex-auth-*")
	if err != nil {
		return "", pkgerrors.Wrap(err, "creating temp dir for Codex auth")
	}
	payload := fmt.Sprintf("{\"OPENAI_API_KEY\":%q}\n", apiKey)
	if err := os.WriteFile(filepath.Join(tmpDir, "auth.json"), []byte(payload), 0o600); err != nil {
		return "", pkgerrors.Wrap(err, "writing Codex auth.json")
	}
	if data, err := os.ReadFile(filepath.Join(codexDir, "config.toml")); err == nil {
		cfgDst := filepath.Join(tmpDir, "config.toml")
		//nolint:gosec // G703: cfgDst is a fixed temp path under tmpDir created in this function.
		if err := os.WriteFile(cfgDst, data, 0o644); err != nil {
			logger.Warn().Err(err).Msg("failed copying ~/.codex/config.toml into container mount")
		}
	}
	return tmpDir, nil
}

func codexStateMountOptions(logger zerolog.Logger, codexDir string) []string {
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
