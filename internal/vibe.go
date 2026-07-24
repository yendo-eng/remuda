package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/agentlauncher"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
)

// crashRecoverySleepSeconds is the duration (in seconds) to keep a detached
// session alive after the agent command exits. This allows inspection of the
// session buffer when the agent crashes unexpectedly.
const crashRecoverySleepSeconds = 3600 // 1 hour

type VibeCommand struct {
	// Strings prepended to the prompt in order.
	BeforePrompt []string

	// Prompt IDs selected via --use (after applying --no-use).
	UsePromptIDs []string

	// The main prompt text.
	Prompt string

	// Strings appended to the prompt in order.
	AfterPrompt []string

	// Name of the session.
	Name string

	// Agent to launch
	Agent string

	// Custom agent command
	AgentCmd string

	// Additional args appended to built-in agent launch commands.
	AgentArgs []string

	// Model to use
	Model string

	// Reasoning effort level (codex only).
	ReasoningLevel string

	// Run the agent in the background
	Detached bool

	// Attach to the session immediately after launching.
	Attach bool

	// Repository info for cloning the workspace.
	Clone CloneCommand

	// Existing workspace path to reuse instead of cloning.
	ExistingWorkspace string

	// For certain agents, enable "yolo" mode which skips sandboxing/approval steps.
	Yolo bool

	// Run the session inside a Docker container.
	Container bool

	// Container image to use.
	ContainerName string

	// Additional raw docker run options to append.
	ContainerOpts []string

	// Additional env var names to forward into docker run (docker -e <NAME>).
	ContainerInheritEnv []string

	// Enable agent remote-control launch behavior where available.
	RemoteControl bool

	// Environment values to apply to the launched agent without embedding them
	// in the shell command string.
	EnvOverrides map[string]string

	// Position saved prompts (UsePromptIDs) are arranged in relative to the
	// main prompt ("before" or "after"). Recorded in the session manifest.
	UsePromptsPosition string

	// When true (gated on the session-manifest experiment), write a
	// .remuda.json launch manifest into the workspace so session resume can
	// reconstruct these settings later.
	SessionManifestEnabled bool
}

func (k Remuda) Vibe(ctx context.Context, cmd VibeCommand) error {
	logger := logging.FromContext(ctx)
	k.SetLogger(logger)
	logger.Debug().Str("agent", cmd.Agent).Msg("starting vibe command")

	// figure out agent configuration
	cmd.Model = strings.TrimSpace(cmd.Model)
	cmd.ReasoningLevel = strings.TrimSpace(cmd.ReasoningLevel)
	agent := agentlauncher.Custom(cmd.AgentCmd)
	if cmd.AgentCmd == "" {
		cmd.Model = agentlauncher.EffectiveModel(cmd.Agent, cmd.Model)
		resolvedReasoningLevel, err := resolveReasoningLevel(logger, cmd.Agent, cmd.Model, cmd.AgentCmd, cmd.ReasoningLevel)
		if err != nil {
			return pkgerrors.Wrap(err, "reasoning-level")
		}
		cmd.ReasoningLevel = resolvedReasoningLevel

		parsed, resolvedModel, err := agentlauncher.ParseWithReasoning(cmd.Agent, cmd.Model, cmd.ReasoningLevel, cmd.Yolo)
		if err != nil {
			return pkgerrors.Wrap(err, "agent")
		}
		cmd.Model = resolvedModel
		agent = parsed
	}
	checkModelSupported(logger, agent, cmd.Model)
	prompt := assemblePrompt(cmd.BeforePrompt, cmd.Prompt, cmd.AfterPrompt)

	// TODO: reinstate this if encessary
	// cmd.APIKeyOptions.ApplyToEnv()

	var workspace string
	if strings.TrimSpace(cmd.ExistingWorkspace) != "" {
		expanded, err := filepath.Abs(cmd.ExistingWorkspace)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to expand workspace path")
		}
		cmd.ExistingWorkspace = expanded

		workspace = cmd.ExistingWorkspace
	} else {
		var err error
		workspace, err = k.Clone(cmd.Clone)
		if err != nil {
			return pkgerrors.Wrap(err, "clone")
		}
	}

	if strings.TrimSpace(workspace) == "" {
		return pkgerrors.New("workspace path is empty")
	}

	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		workspaceAbs = workspace
	}

	if cmd.SessionManifestEnabled {
		manifest := SessionManifest{
			Agent:          agent.Name(),
			Model:          cmd.Model,
			ReasoningLevel: cmd.ReasoningLevel,
			Yolo:           cmd.Yolo,
			AgentCmd:       cmd.AgentCmd,
			AgentArgs:      cmd.AgentArgs,
			UsePrompts:     cmd.UsePromptIDs,
			UsePosition:    cmd.UsePromptsPosition,
			Container: SessionManifestContainer{
				Enabled:    cmd.Container,
				Image:      cmd.ContainerName,
				Opts:       cmd.ContainerOpts,
				InheritEnv: cmd.ContainerInheritEnv,
			},
		}
		if err := WriteSessionManifest(k.Git, workspaceAbs, manifest); err != nil {
			return pkgerrors.Wrap(err, "session manifest")
		}
	}

	sessionName := session.SessionNameFromWorkspaceName(workspace)
	containerName := docker.ContainerNameFromSession(sessionName)
	if cmd.RemoteControl {
		var remoteApplied bool
		agent, remoteApplied = agent.WithRemoteControl(sessionName)
		if !remoteApplied {
			logger.Warn().
				Str("agent", agent.Name()).
				Msg("remote control is not supported for this agent; ignoring --remote")
		}
	}
	logger.Debug().Str("workspace", workspace).Str("session", sessionName).Msg("workspace ready")
	logCreatingSession(logger, creatingSessionLogContext{
		Workspace:     workspaceAbs,
		Session:       sessionName,
		Agent:         agent.Name(),
		Detached:      cmd.Detached,
		Container:     cmd.Container,
		ContainerName: containerName,
		UsePromptIDs:  cmd.UsePromptIDs,
	})

	agentName := agent.Name()
	containerImage := strings.TrimSpace(cmd.ContainerName)

	logctx := launchingAgentLogContext{
		Workspace:      workspace,
		Session:        sessionName,
		Agent:          agentName,
		Model:          cmd.Model,
		Yolo:           cmd.Yolo,
		Detached:       cmd.Detached,
		Container:      cmd.Container,
		ContainerImage: containerImage,
		ContainerName:  containerName,
		BeforePrompt:   cmd.BeforePrompt,
		AfterPrompt:    cmd.AfterPrompt,
	}
	if !logctx.Container {
		logctx.ContainerImage = ""
		logctx.ContainerName = ""
	}
	logLaunchingAgent(logger, logctx)

	agentCommand := agent.Command(prompt)
	if cmd.AgentCmd == "" {
		agentCommand = agent.Command(prompt, cmd.AgentArgs...)
	}

	_, err = k.launchAgentSession(agentLaunchCommand{
		Workspace:           workspaceAbs,
		SessionName:         sessionName,
		AgentName:           agentName,
		Model:               cmd.Model,
		Command:             agentCommand,
		Detached:            cmd.Detached,
		Attach:              cmd.Attach,
		ReplaceExisting:     cmd.Clone.Force,
		Container:           cmd.Container,
		ContainerImage:      cmd.ContainerName,
		ContainerOpts:       cmd.ContainerOpts,
		ContainerInheritEnv: cmd.ContainerInheritEnv,
		Yolo:                cmd.Yolo,
		EnvOverrides:        cmd.EnvOverrides,
	})
	return err
}

func checkModelSupported(logger zerolog.Logger, agent agentlauncher.AgentLauncher, model string) {
	if model == "" {
		return
	}
	if model == agentlauncher.ModelAgentDefault {
		return
	}
	if model == agentlauncher.ModelAgentDefault {
		return
	}

	models := agent.SupportedModels()

	if models == nil {
		return
	}

	for _, m := range models {
		if m == model {
			return
		}
	}

	logger.Warn().
		Str("model", model).
		Str("agent", agent.Name()).
		Msg("warning: the selected model may not be supported by the chosen agent")
}

func (k Remuda) composeLaunchCommand(
	cmd VibeCommand,
	workspace, agentCmd, sessionName, containerName string,
	envProvider env.Provider,
) (string, string, error) {
	logger := k.logger()
	if !cmd.Container {
		return agentCmd, "", nil
	}

	containerImage := strings.TrimSpace(cmd.ContainerName)
	if containerImage == "" {
		return "", "", pkgerrors.New(
			"container mode requires an explicit image; pass --container-name or configure defaults.container.image (including profiles.<name>.container.image or per_repo.<slug>.defaults.container.image)",
		)
	}

	if err := k.Docker.CheckRunning(); err != nil {
		return "", "", err
	}

	github.EnsureTokenInEnvWithProvider(envProvider)

	absWS, err := filepath.Abs(workspace)
	if err != nil {
		absWS = workspace
	}

	// User-supplied --container-opt values are raw docker CLI text (eg. "-v
	// /base:/base" or "--cpus=4"); split each into its own argv tokens so they
	// combine correctly with the argv opts built below instead of relying on a
	// shell to word-split them later.
	var containerOpts []string
	for _, raw := range cmd.ContainerOpts {
		containerOpts = append(containerOpts, strings.Fields(raw)...)
	}
	if len(cmd.ContainerInheritEnv) > 0 {
		inheritOpts, err := containerInheritEnvOpts(cmd.ContainerInheritEnv)
		if err != nil {
			return "", "", err
		}
		containerOpts = append(containerOpts, inheritOpts...)
	}
	containerOpts = append([]string{"-e", "BD_ACTOR"}, containerOpts...)
	if _, ok := envProvider.LookupEnv("BEADS_DIR"); ok && !containerOptsDefineEnv(containerOpts, "BEADS_DIR") {
		containerOpts = append(containerOpts, "-e", "BEADS_DIR")
	}
	if mountVal, ok := docker.ExtraGitMountForWorktree(absWS); ok {
		containerOpts = append([]string{"-v", mountVal}, containerOpts...)
	}

	containerOpts = append(containerOpts, docker.BuildGoCacheMountOptsWithLogger(logger)...)
	if strings.TrimSpace(cmd.Agent) == "" || strings.EqualFold(cmd.Agent, "codex") || strings.EqualFold(cmd.Agent, "bash") {
		containerOpts = append(containerOpts, codexDockerVolumeMountOptions(logger, envProvider)...)
	}
	if strings.EqualFold(cmd.Agent, "claude") || strings.EqualFold(cmd.Agent, "bash") {
		containerOpts = append(containerOpts, "-e", "ANTHROPIC_API_KEY")
	}
	if strings.EqualFold(cmd.Agent, "claude") && cmd.Yolo {
		containerOpts = append(containerOpts, "-e", "IS_SANDBOX")
	}

	authOpts := docker.BuildContainerAuthOptsWithProvider(envProvider)
	allOpts := append(append([]string{}, containerOpts...), authOpts...)
	if strings.EqualFold(cmd.Agent, "opencode") || strings.EqualFold(cmd.Agent, "bash") {
		allOpts = append(allOpts, docker.BuildOpenCodeStateMountOptsWithLogger(logger, envProvider)...)
	}
	if strings.EqualFold(cmd.Agent, "claude") || strings.EqualFold(cmd.Agent, "bash") {
		allOpts = append(allOpts, docker.BuildClaudeStateMountOptsWithLogger(logger, envProvider)...)
	}
	containerAgent := util.SSHRewriteSnippet() + "\n" + agentCmd
	launchCmd := docker.BuildRunCommand(absWS, containerImage, allOpts, containerAgent, false, containerName)
	return launchCmd, containerImage, nil
}

func containerOptsDefineEnv(opts []string, name string) bool {
	var fields []string
	for _, opt := range opts {
		fields = append(fields, strings.Fields(opt)...)
	}

	for i := 0; i < len(fields); i++ {
		field := strings.TrimSpace(fields[i])
		switch {
		case field == "-e" || field == "--env":
			if i+1 < len(fields) && dockerEnvSpecName(fields[i+1]) == name {
				return true
			}
			i++
		case strings.HasPrefix(field, "-e="):
			if dockerEnvSpecName(strings.TrimPrefix(field, "-e=")) == name {
				return true
			}
		case strings.HasPrefix(field, "-e") && len(field) > len("-e"):
			if dockerEnvSpecName(strings.TrimPrefix(field, "-e")) == name {
				return true
			}
		case strings.HasPrefix(field, "--env="):
			if dockerEnvSpecName(strings.TrimPrefix(field, "--env=")) == name {
				return true
			}
		}
	}
	return false
}

func dockerEnvSpecName(spec string) string {
	spec = strings.Trim(strings.TrimSpace(spec), `"'`)
	name, _, _ := strings.Cut(spec, "=")
	return name
}

func containerInheritEnvOpts(names []string) ([]string, error) {
	opts := make([]string, 0, len(names)*2)
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !util.IsValidEnvVarName(name) {
			return nil, pkgerrors.Errorf("invalid env var name %q", raw)
		}
		opts = append(opts, "-e", name)
	}
	return opts, nil
}

// wrapWithCrashRecoverySleep appends a sleep command after the given command
// so that detached sessions remain alive for inspection after the agent exits.
func wrapWithCrashRecoverySleep(cmd string) string {
	return fmt.Sprintf("%s; sleep %d", cmd, crashRecoverySleepSeconds)
}

// codexDockerVolumeMountOptions prepares Docker volume mount options for the Codex CLI.
// It forwards authentication artifacts (auth.json + config.toml) plus ~/.codex/prompts, ~/.codex/rules,
// ~/.codex/skills, and ~/.codex/AGENTS.md whenever they are available so custom instructions and slash
// command state work inside --container sessions.
// If the OPENAI_API_KEY is not set or any file operations fail, it returns only the mounts that can be derived
// (for example the prompts directory) and logs warnings for auth-related failures.
func codexDockerVolumeMountOptions(logger zerolog.Logger, provider env.Provider) []string {
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

type launchingAgentLogContext struct {
	Workspace      string
	Session        string
	Agent          string
	Model          string
	Yolo           bool
	Detached       bool
	Container      bool
	ContainerImage string
	ContainerName  string
	BeforePrompt   []string
	AfterPrompt    []string
}

func logLaunchingAgent(logger zerolog.Logger, ctx launchingAgentLogContext) {
	// TODO: clean this up so that it is actually useful
	event := logger.Debug().
		Str("workspace", ctx.Workspace).
		Str("session", ctx.Session).
		Str("agent", ctx.Agent).
		Str("model", ctx.Model).
		// Bool("custom_agent_cmd", ctx.CustomAgentCmd).
		Bool("container", ctx.Container).
		Str("container_name", ctx.ContainerName).
		Bool("yolo", ctx.Yolo).
		Bool("detached", ctx.Detached)
	if ctx.Model != "" {
		event = event.Str("model", ctx.Model)
	}
	if ctx.Container && ctx.ContainerImage != "" {
		event = event.Str("container_image", ctx.ContainerImage)
	}
	if len(ctx.BeforePrompt) > 0 {
		event = event.Strs("before_prompt", ctx.BeforePrompt)
	}
	if len(ctx.AfterPrompt) > 0 {
		event = event.Strs("after_prompt", ctx.AfterPrompt)
	}
	event.Msg("agent debug info")
}
