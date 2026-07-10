package cli

import (
	"strings"

	"github.com/knadh/koanf/v2"
	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/logging"
)

// SessionResumeCmd resumes the most recent supported agent session in an inactive workspace.
type SessionResumeCmd struct {
	AgentSessionOptions
	ContextEngineeringOptions
	APIKeyOptions
	VibeContainerOptions

	WorkspaceDir string
	Prompt       string
	Pick         bool
	Profile      string
	Yolo         bool
}

func (a *app) sessionResumeCmd() *cobra.Command {
	c := &SessionResumeCmd{}
	var fl *flagSet
	cmd := &cobra.Command{
		Use:   "resume [workspace-dir] [prompt]",
		Short: "Resume the most recent supported agent session in an inactive workspace.",
		Long:  "Resume the most recent supported agent session in an inactive workspace. Use '-' as the prompt to read it from STDIN.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				c.WorkspaceDir = args[0]
			}
			if len(args) > 1 {
				c.Prompt = args[1]
			}
			err := a.prepare(cmd, prepareOpts{
				fl:       fl,
				profiled: true,
				slugFn: func() string {
					if c.Pick || strings.TrimSpace(c.WorkspaceDir) == "" {
						return ""
					}
					return repoSlugFromWorkspacePath(*a.kctx, a.kctx.ConfigFile, c.WorkspaceDir)
				},
			})
			if err != nil {
				return err
			}
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
	c.AgentSessionOptions.register(cmd, fl)
	c.ContextEngineeringOptions.register(cmd, fl)
	c.APIKeyOptions.register(cmd, fl)
	c.VibeContainerOptions.register(cmd, fl)
	cmd.Flags().BoolVar(&c.Pick, "pick", false, "Use fzf to interactively select an inactive workspace to resume.")
	registerProfileFlag(cmd, &c.Profile)
	registerYoloFlag(cmd, fl, &c.Yolo)
	registerWorkspaceDirPositionalCompletion(cmd)

	return cmd
}

func (c *SessionResumeCmd) validate() error {
	if err := c.VibeContainerOptions.Validate(); err != nil {
		return err
	}

	hasWorkspace := strings.TrimSpace(c.WorkspaceDir) != ""
	hasPrompt := strings.TrimSpace(c.Prompt) != ""
	if c.Pick && hasWorkspace && !hasPrompt {
		// In --pick mode the first positional binds to WorkspaceDir.
		// Treat that value as the optional resume prompt.
		c.Prompt = c.WorkspaceDir
		c.WorkspaceDir = ""
		hasWorkspace = false
	}
	if hasWorkspace && c.Pick {
		return pkgerrors.New("cannot combine <workspace-dir> with --pick")
	}
	if !hasWorkspace && !c.Pick {
		return pkgerrors.New("exactly one of <workspace-dir> and --pick is required")
	}
	if c.Pick {
		return nil
	}
	if strings.TrimSpace(c.WorkspaceDir) == "" {
		return pkgerrors.New("<workspace-dir> cannot be blank")
	}
	return nil
}

func (c *SessionResumeCmd) Run(ctx Context) error {
	if c.Pick && !ctx.Remuda.IO.IsTerminal() {
		return pkgerrors.New("--pick requires an interactive TTY")
	}

	logger := logging.FromContext(ctx.ctx)
	var selected string
	if c.Pick {
		inactive, err := ctx.Remuda.InactiveWorkspacesWithIgnore(configuredPruneIgnorePatterns(ctx.ConfigFile))
		if err != nil {
			return pkgerrors.Wrap(err, "list inactive workspaces")
		}
		if len(inactive) == 0 {
			ctx.Remuda.IO.Outln("No inactive workspaces to resume.")
			return nil
		}

		picked, err := pickOneWorkspaceWithFZF(logger, envFromContext(ctx), inactive, ctx.Remuda.Config.ReposBaseDir)
		if err != nil {
			return pkgerrors.Wrap(err, "pick workspace")
		}
		if strings.TrimSpace(picked) == "" {
			return pkgerrors.New("no workspace selected")
		}
		selected = picked
	} else {
		home, homeErr := homeDirFromContext(ctx)
		expanded, err := expandHomePath(strings.TrimSpace(c.WorkspaceDir), home, homeErr)
		if err != nil {
			return err
		}
		selected = absPathFromContext(expanded, ctx)
	}

	selectedAbs := absPathFromContext(selected, ctx)
	if err := internal.ValidateWorkspacePath(ctx.Remuda.Config.ReposBaseDir, selectedAbs); err != nil {
		return pkgerrors.Wrapf(err, "invalid workspace %q", selectedAbs)
	}
	if c.Pick {
		// The picked workspace determines the per_repo overlays.
		slug := repoSlugFromWorkspacePath(ctx, ctx.ConfigFile, selectedAbs)
		if err := ctx.ApplyRepoOverlays(slug); err != nil {
			return err
		}
	}
	if err := validateContainerImageSelection(c.Container, c.ContainerName); err != nil {
		return err
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

	agentName := strings.TrimSpace(c.Agent)
	if !ctx.FlagExplicit("agent") {
		agentName = resolveSessionResumeAgent(ctx.EffectiveConfig(), envFromContext(ctx))
	}

	prompt := c.Prompt
	if prompt != "" {
		addedContext, err := c.AddedPromptContext(ctx, PromptContextInput{
			GitHubRepoSlug: repoSlugFromWorkspacePath(ctx, ctx.ConfigFile, selectedAbs),
		})
		if err != nil {
			return pkgerrors.Wrap(err, "adding prompt context")
		}
		if len(addedContext) > 0 {
			var fullPrompt strings.Builder
			for _, p := range addedContext {
				fullPrompt.WriteString(p)
				fullPrompt.WriteString("\n")
			}
			fullPrompt.WriteString(prompt)
			prompt = fullPrompt.String()
		}
	}

	cmd := internal.SessionResumeCommand{
		Workspace:           selectedAbs,
		Agent:               agentName,
		Model:               c.Model,
		AgentCmd:            c.AgentCmd,
		Prompt:              prompt,
		Detached:            c.DetachedMode(),
		Attach:              c.Attach,
		Yolo:                c.Yolo,
		ReasoningLevel:      c.ReasoningLevel,
		OpenAIAPIKey:        c.OpenAIAPIKey,
		Container:           c.Container,
		ContainerName:       c.ContainerName,
		ContainerOpts:       c.ContainerOpt,
		ContainerInheritEnv: c.ContainerInheritEnv,
	}
	if ctx.FlagExplicit("openai-api-key") || strings.TrimSpace(c.OpenAIAPIKey) != "" {
		cmd.EnvOverrides = map[string]string{"OPENAI_API_KEY": c.OpenAIAPIKey}
	}

	return ctx.Remuda.SessionResume(ctx.ctx, cmd)
}

// resolveSessionResumeAgent picks the resume agent when --agent is not set
// explicitly: only claude resumes as claude; anything else falls back to
// codex (the historically supported resume path).
func resolveSessionResumeAgent(eff *koanf.Koanf, env EnvProvider) string {
	env = envOrDefault(env)
	if val, ok := env.LookupEnv("REMUDA_AGENT"); ok {
		trimmed := strings.TrimSpace(val)
		if trimmed != "" && strings.EqualFold(trimmed, "claude") {
			return "claude"
		}
		if trimmed != "" {
			return "codex"
		}
	}
	if eff == nil || !eff.Exists("defaults.agent") {
		return "codex"
	}
	if strings.EqualFold(strings.TrimSpace(eff.String("defaults.agent")), "claude") {
		return "claude"
	}
	return "codex"
}
