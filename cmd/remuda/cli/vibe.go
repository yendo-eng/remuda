package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal"
)

type VibeCmd struct {
	VibeNameWizardOption      `embed:""`
	CloneRepoOption           `embed:""`
	CloneHooksOption          `embed:""`
	FullCloneOption           `embed:""`
	AgentSessionOptions       `embed:""`
	ExperimentsOption         `embed:""`
	ContextEngineeringOptions `embed:""`
	APIKeyOptions             `embed:""`
	SlugifyOptions            `embed:""`
	VibeContainerOptions      `embed:""`

	Prompt string `arg:"" optional:"" name:"prompt" help:"Prompt to pass to the coding agent. Use '-' to read from STDIN."`
	In     string `kong:"name=in,help='Launch inside an existing workspace folder instead of cloning.',predictor='workspace-dir'"`

	// Branch overrides the default of using --name for git branch checkout.
	Branch string `name:"branch" help:"Checkout this branch instead of deriving from --name."`

	Profile string `name:"profile" env:"REMUDA_PROFILE" help:"Config profile name to apply from config.yaml (profiles section)." predictor:"profile-name"`

	Yolo bool `name:"yolo" env:"REMUDA_YOLO" negatable:"" help:"Ignore sandboxing/approvals for supported agents (Codex/Claude)."`

	Remote bool `name:"remote" help:"Enable remote control when supported by the selected agent."`

	// If we end up needing this later, uncomment it
	// GitUseSSH     bool     `name:"git-use-ssh" help:"When --container, rewrite HTTPS origin to SSH if SSH agent is available."`
}

// VibeNameWizardOption groups CLI switches for setting a workspace name or
// launching the interactive wizard.
//
// For `remuda vibe`, --name is optional: when omitted (and --in is not used),
// Remuda derives a workspace name from the prompt.
type VibeNameWizardOption struct {
	Name   string `name:"name" help:"Workspace name; reused as branch (and session) name. If omitted, Remuda derives a name from the first --jira ticket title (when provided) or from the prompt." xor:"name_or_wizard"`
	Wizard bool   `name:"wizard" help:"Launch interactive wizard for this command (requires a TTY)." xor:"name_or_wizard"`
}

func (c *VibeCmd) Validate() error {
	if err := c.VibeContainerOptions.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.In) == "" {
		return nil
	}

	// --in set: disallow conflicting flags.
	if c.Wizard {
		return errors.New("--in cannot be combined with --wizard")
	}
	if strings.TrimSpace(c.Name) != "" {
		return errors.New("--name cannot be combined with --in; it is derived from the folder path")
	}
	if strings.TrimSpace(c.Branch) != "" {
		return errors.New("--branch cannot be combined with --in; no clone/checkout is performed")
	}
	if c.FullClone {
		return errors.New("--full-clone cannot be combined with --in; no clone is performed")
	}

	return nil
}

func (c *VibeCmd) Run(ctx Context, kctx *kong.Context) error {
	if c.Wizard {
		var err error
		*c, err = launchVibeStartWizard(ctx, *c)
		if err != nil {
			return err
		}
	}

	if resolved, _, err := resolvePromptFromStdin(ctx.Remuda.IO.In, c.Prompt); err != nil {
		return err
	} else {
		c.Prompt = resolved
	}

	if strings.TrimSpace(c.Prompt) == "" {
		c.Prompt = ""
	}
	if err := c.validatePromptUsage(c.Prompt, kctx.Args); err != nil {
		return err
	}

	if strings.TrimSpace(c.In) != "" {
		if abs := absPathFromContext(c.In, ctx); abs != "" {
			c.In = abs
		}
	}

	allowFTUE := !c.Wizard && strings.TrimSpace(c.In) == ""
	repoSelection, err := resolveRepoSelectionWithFTUE(ctx, kctx, c.CloneRepoOption, RepoResolutionOptions{
		AllowFallback:     true,
		ExistingWorkspace: c.In,
		ReposBaseDir:      ctx.Remuda.Config.ReposBaseDir,
	}, allowFTUE)
	if err != nil {
		return err
	}
	repoURL := repoSelection.RepoURL
	repoSlug := repoSelection.RepoSlug

	if !c.Wizard {
		if err := applyProfileOverlayByName(ctx.ConfigFile, c.Profile); err != nil {
			return err
		}
		if err := applyPerRepoDefaultsToVibe(c, kctx, ctx.ConfigFile, envFromContext(ctx)); err != nil {
			return err
		}
	}
	if err := validateContainerImageSelection(c.Container, c.ContainerName); err != nil {
		return err
	}

	cmd := internal.VibeCommand{
		Name:                c.Name,
		Agent:               c.Agent,
		Model:               c.Model,
		ReasoningLevel:      c.ReasoningLevel,
		AgentCmd:            c.AgentCmd,
		SkipVersionCheck:    c.SkipVersionCheck,
		Prompt:              c.Prompt,
		Detached:            c.DetachedMode(),
		Attach:              c.Attach,
		Yolo:                c.Yolo,
		Container:           c.Container,
		ContainerName:       c.ContainerName,
		ContainerOpts:       c.ContainerOpt,
		ContainerInheritEnv: c.ContainerInheritEnv,
		RemoteControl:       c.Remote,
	}
	cmd.ExistingWorkspace = c.In

	usePromptIDs := c.effectiveUsePromptNames()
	wrapUsePrompts := c.ExperimentEnabled(experimentUsePromptsContextWrapper)
	usePromptsSelected := len(usePromptIDs) > 0
	cmd.UsePromptIDs = usePromptIDs

	if strings.TrimSpace(c.Prompt) != "" {
		addedContext, err := c.AddedPromptContext(ctx, PromptContextInput{
			GitHubRepoSlug: repoSlug,
			WrapUsePrompts: wrapUsePrompts,
		})
		if err != nil {
			return errors.Wrap(err, "adding prompt context")
		}
		cmd.BeforePrompt = append(cmd.BeforePrompt, addedContext...)
		if shouldAddMainPromptMarker(wrapUsePrompts, usePromptsSelected) {
			cmd.BeforePrompt = append(cmd.BeforePrompt, "Main prompt:")
		}
	}

	cmd.Clone.Name = c.Name
	cmd.Clone.RepoURL = repoURL
	cmd.Clone.SkipCloneHooks = c.NoCloneHooks
	cmd.Clone.Force = c.Force
	cmd.Clone.FullClone = c.FullClone
	cmd.Clone.Branch = c.Branch

	// Auto-generate a workspace name when --name is omitted.
	if generated, ok, err := deriveDefaultVibeWorkspaceName(ctx, *c); err != nil {
		return err
	} else if ok {
		c.Name = generated
		cmd.Name = generated
		cmd.Clone.Name = generated
	}

	err = ctx.Remuda.Vibe(ctx.ctx, cmd)
	if err != nil {
		return errors.Wrap(err, "vibe")
	}

	return nil
}

func usesPromptPrefaceFromArgs(args []string) bool {
	for _, arg := range args {
		if arg == "-u" || arg == "--use" || strings.HasPrefix(arg, "--use=") || strings.HasPrefix(arg, "-u=") || strings.HasPrefix(arg, "-u") {
			return true
		}
	}
	return false
}
