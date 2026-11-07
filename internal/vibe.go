package internal

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
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

// StaleWorktreeError reports a --in target that git no longer recognizes as a
// worktree, typically a pruned worktree whose checkout folder was left behind.
type StaleWorktreeError struct {
	Dir string
	Err error
}

func (e StaleWorktreeError) Error() string {
	// git reports the real cause (dead gitdir, missing path, permissions) on
	// stderr and leaves the error itself as a bare exit status.
	cause := e.Err.Error()
	var exitErr *exec.ExitError
	if errors.As(e.Err, &exitErr) && len(exitErr.Stderr) > 0 {
		cause = strings.TrimSpace(string(exitErr.Stderr))
	}
	return fmt.Sprintf("%s is not a valid git worktree (stale or pruned); nothing to resume here: %s", e.Dir, cause)
}

func (e StaleWorktreeError) Unwrap() error { return e.Err }

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
	logger.Debug().Str("agent", cmd.Agent).Msg("starting vibe command")
	if err := validateMultiplexerLaunch(k.Multiplexer, cmd.AgentCmd); err != nil {
		return err
	}

	// figure out agent configuration
	cmd.Model = strings.TrimSpace(cmd.Model)
	cmd.ReasoningLevel = strings.TrimSpace(cmd.ReasoningLevel)
	agent := agentlauncher.Custom(cmd.AgentCmd)
	if cmd.AgentCmd == "" {
		cmd.Model = agentlauncher.EffectiveModel(cmd.Agent, cmd.Model)
		cmd.ReasoningLevel = resolveReasoningLevel(logger, cmd.Agent, cmd.Model, cmd.ReasoningLevel)

		parsed, resolvedModel, err := agentlauncher.ParseWithReasoning(cmd.Agent, cmd.Model, cmd.ReasoningLevel, cmd.Yolo)
		if err != nil {
			return pkgerrors.Wrap(err, "agent")
		}
		cmd.Model = resolvedModel
		agent = parsed
	}
	checkModelSupported(logger, agent, cmd.Model)
	prompt := assemblePrompt(cmd.BeforePrompt, cmd.Prompt, cmd.AfterPrompt)

	var workspace string
	if strings.TrimSpace(cmd.ExistingWorkspace) != "" {
		expanded, err := filepath.Abs(cmd.ExistingWorkspace)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to expand workspace path")
		}
		cmd.ExistingWorkspace = expanded

		if _, err := k.Git.RevParse(cmd.ExistingWorkspace, "--is-inside-work-tree"); err != nil {
			return StaleWorktreeError{Dir: cmd.ExistingWorkspace, Err: err}
		}

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
	var agentArgs []string
	if cmd.AgentCmd == "" {
		extraArgs := slices.Concat(cmd.AgentArgs, sessionAgentArgs(k.Multiplexer, cmd.Detached, agentName))
		agentCommand = agent.Command(prompt, extraArgs...)
		agentArgs = agent.Arguments("", extraArgs...)
	}

	_, err = k.launchAgentSession(agentLaunchCommand{
		Workspace:           workspaceAbs,
		SessionName:         sessionName,
		Agent:               agent,
		Model:               cmd.Model,
		Command:             agentCommand,
		Args:                agentArgs,
		Prompt:              prompt,
		Detached:            cmd.Detached,
		Attach:              cmd.Attach,
		ReplaceExisting:     cmd.Clone.Force,
		Container:           cmd.Container,
		ContainerImage:      cmd.ContainerName,
		ContainerOpts:       cmd.ContainerOpts,
		ContainerInheritEnv: cmd.ContainerInheritEnv,
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

type composedLaunchCommand struct {
	shell string
	argv  []string
	image string
}

func (k Remuda) composeLaunchCommand(
	cmd agentLaunchCommand,
	workspace, sessionName, containerName string,
	envProvider env.Provider,
) (composedLaunchCommand, error) {
	logger := k.Logger
	if !cmd.Container {
		return composedLaunchCommand{shell: cmd.Command}, nil
	}

	containerImage := strings.TrimSpace(cmd.ContainerImage)
	if containerImage == "" {
		return composedLaunchCommand{}, pkgerrors.New(
			"container mode requires an explicit image; pass --container-name or configure defaults.container.image (including profiles.<name>.container.image or per_repo.<slug>.defaults.container.image)",
		)
	}

	if err := k.Docker.CheckRunning(); err != nil {
		return composedLaunchCommand{}, err
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
			return composedLaunchCommand{}, err
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

	containerOpts = append(containerOpts, docker.BuildGoCacheMountOpts(logger)...)
	for _, name := range slices.Sorted(maps.Keys(cmd.Agent.ContainerEnv())) {
		containerOpts = append(containerOpts, "-e", name)
	}
	containerOpts = append(containerOpts, cmd.Agent.ContainerMounts(logger, envProvider)...)
	containerOpts = append(containerOpts, docker.BuildContainerAuthOpts(envProvider)...)
	containerAgent := util.SSHRewriteSnippet() + "\n" + cmd.Command
	launchCmd := docker.BuildRunCommand(absWS, containerImage, containerOpts, containerAgent, containerName)
	launchArgv := docker.BuildRunArgv(absWS, containerImage, containerOpts, containerAgent, containerName)
	return composedLaunchCommand{
		shell: launchCmd,
		argv:  launchArgv,
		image: containerImage,
	}, nil
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
	event := logger.Debug().
		Str("workspace", ctx.Workspace).
		Str("session", ctx.Session).
		Str("agent", ctx.Agent).
		Str("model", ctx.Model).
		Bool("container", ctx.Container).
		Str("container_name", ctx.ContainerName).
		Bool("yolo", ctx.Yolo).
		Bool("detached", ctx.Detached)
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
