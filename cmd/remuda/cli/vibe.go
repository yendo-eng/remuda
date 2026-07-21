package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	expregistry "github.com/yendo-eng/remuda/internal/experiments"
)

type VibeCmd struct {
	VibeNameWizardOption
	CloneRepoOption
	CloneHooksOption
	FullCloneOption
	AgentSessionOptions
	ContextEngineeringOptions
	APIKeyOptions
	SlugifyOptions
	VibeContainerOptions

	Prompt  string
	In      string
	Branch  string
	Profile string
	Yolo    bool
	Remote  bool
}

// VibeNameWizardOption groups CLI switches for setting a workspace name or
// launching the interactive wizard.
//
// For `remuda vibe`, --name is optional: when omitted (and --in is not used),
// Remuda derives a workspace name from the prompt.
type VibeNameWizardOption struct {
	Name   string
	Wizard bool
}

func (o *VibeNameWizardOption) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&o.Name, "name", "", "Workspace name; reused as branch (and session) name. If omitted, Remuda derives a name from the first --jira ticket title (when provided) or from the prompt.")
	fs.BoolVar(&o.Wizard, "wizard", false, "Launch interactive wizard for this command (requires a TTY).")
	cmd.MarkFlagsMutuallyExclusive("name", "wizard")
}

func (a *app) vibeCmd() *cobra.Command {
	c := &VibeCmd{}
	var fl *flagSet
	cmd := &cobra.Command{
		Use:   "vibe [prompt]",
		Short: "Clone and launch an AI coding session.",
		Long:  "Clone and launch an AI coding session. Use '-' as the prompt to read it from STDIN.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.Prompt = args[0]
			}
			err := a.prepare(cmd, prepareOpts{
				fl:       fl,
				profiled: true,
				slugFn: func() string {
					c.CloneRepoOption.normalize()
					return c.repoSelection(*a.kctx, RepoResolutionOptions{
						ExistingWorkspace: c.In,
						ReposBaseDir:      a.kctx.Remuda.Config.ReposBaseDir,
					}).RepoSlug
				},
			})
			if err != nil {
				return err
			}
			c.CloneRepoOption.normalize()
			if err := c.AgentSessionOptions.afterApply(); err != nil {
				return err
			}
			if err := c.ContextEngineeringOptions.afterApply(*a.kctx); err != nil {
				return err
			}
			if err := c.validate(); err != nil {
				return err
			}
			return c.Run(*a.kctx)
		},
	}

	fl = newFlagSet(cmd.Flags())
	c.VibeNameWizardOption.register(cmd)
	c.CloneRepoOption.register(cmd, fl)
	c.CloneHooksOption.register(cmd)
	c.FullCloneOption.register(cmd, fl)
	c.AgentSessionOptions.register(cmd, fl)
	c.ContextEngineeringOptions.register(cmd, fl)
	c.APIKeyOptions.register(cmd, fl)
	c.SlugifyOptions.register(cmd, fl)
	c.VibeContainerOptions.register(cmd, fl)

	fs := cmd.Flags()
	fs.StringVar(&c.In, "in", "", "Launch inside an existing workspace folder instead of cloning.")
	fs.StringVar(&c.Branch, "branch", "", "Checkout this branch instead of deriving from --name.")
	fs.BoolVar(&c.Remote, "remote", false, "Enable remote control when supported by the selected agent.")
	registerProfileFlag(cmd, &c.Profile)
	registerYoloFlag(cmd, fl, &c.Yolo)
	registerWorkspaceDirCompletion(cmd, "in")

	return cmd
}

func (c *VibeCmd) validate() error {
	if err := c.VibeContainerOptions.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.In) == "" {
		return nil
	}

	// --in set: disallow conflicting flags.
	if c.Wizard {
		return pkgerrors.New("--in cannot be combined with --wizard")
	}
	if strings.TrimSpace(c.Name) != "" {
		return pkgerrors.New("--name cannot be combined with --in; it is derived from the folder path")
	}
	if strings.TrimSpace(c.Branch) != "" {
		return pkgerrors.New("--branch cannot be combined with --in; no clone/checkout is performed")
	}
	if c.FullClone {
		return pkgerrors.New("--full-clone cannot be combined with --in; no clone is performed")
	}

	return nil
}

func (c *VibeCmd) Run(ctx Context) error {
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
	if err := c.validatePromptUsage(ctx, c.Prompt); err != nil {
		return err
	}

	if strings.TrimSpace(c.In) != "" {
		if abs := absPathFromContext(c.In, ctx); abs != "" {
			c.In = abs
		}
	}

	allowFTUE := !c.Wizard && strings.TrimSpace(c.In) == ""
	repoSelection, err := resolveRepoSelectionWithFTUE(ctx, c.CloneRepoOption, RepoResolutionOptions{
		AllowFallback:     true,
		ExistingWorkspace: c.In,
		ReposBaseDir:      ctx.Remuda.Config.ReposBaseDir,
	}, allowFTUE)
	if err != nil {
		return err
	}
	repoURL := repoSelection.RepoURL
	repoSlug := repoSelection.RepoSlug

	if err := validateContainerImageSelection(c.Container, c.ContainerName); err != nil {
		return err
	}
	agentArgs := effectiveAgentArgsFromKoanf(ctx.EffectiveConfig(), c.Agent, c.AgentArg)

	cmd := internal.VibeCommand{
		Name:                c.Name,
		Agent:               c.Agent,
		Model:               c.Model,
		ReasoningLevel:      c.ReasoningLevel,
		AgentCmd:            c.AgentCmd,
		AgentArgs:           agentArgs,
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
	if ctx.FlagExplicit("openai-api-key") || strings.TrimSpace(c.OpenAIAPIKey) != "" {
		cmd.EnvOverrides = map[string]string{"OPENAI_API_KEY": c.OpenAIAPIKey}
	}
	cmd.ExistingWorkspace = c.In

	usePromptIDs := c.effectiveUsePromptNames()
	wrapUsePrompts := ctx.ExperimentEnabled(expregistry.UsePromptsContextWrapper)
	usePromptsSelected := len(usePromptIDs) > 0
	cmd.UsePromptIDs = usePromptIDs

	if strings.TrimSpace(c.Prompt) != "" {
		parts, err := c.AddedPromptContext(ctx, PromptContextInput{
			GitHubRepoSlug: repoSlug,
			WrapUsePrompts: wrapUsePrompts,
		})
		if err != nil {
			return pkgerrors.Wrap(err, "adding prompt context")
		}
		before, after := arrangePromptContext(parts, c.effectiveUsePromptsPosition(), shouldAddMainPromptMarker(wrapUsePrompts, usePromptsSelected))
		cmd.BeforePrompt = append(cmd.BeforePrompt, before...)
		cmd.AfterPrompt = append(cmd.AfterPrompt, after...)
	}

	cmd.Clone.Name = c.Name
	cmd.Clone.RepoURL = repoURL
	cmd.Clone.SkipCloneHooks = c.NoCloneHooks
	cmd.Clone.Force = c.Force
	cmd.Clone.FullClone = c.FullClone
	cmd.Clone.CoWCopy = ctx.ExperimentEnabled(expregistry.CoWClone)
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
		return pkgerrors.Wrap(err, "vibe")
	}

	return nil
}
