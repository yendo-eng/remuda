package cli

import (
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/logging"
)

// SessionResumeCmd resumes the most recent supported agent session in an inactive workspace.
type SessionResumeCmd struct {
	AgentSessionOptions       `embed:""`
	ContextEngineeringOptions `embed:""`
	APIKeyOptions             `embed:""`
	VibeContainerOptions      `embed:""`

	WorkspaceDir string `arg:"" optional:"" name:"workspace-dir" help:"Workspace directory to resume." predictor:"workspace-dir"`
	Prompt       string `arg:"" optional:"" name:"prompt" help:"Prompt to send after resuming. Use '-' to read from STDIN."`
	Pick         bool   `name:"pick" help:"Use fzf to interactively select an inactive workspace to resume."`
	Profile      string `name:"profile" env:"REMUDA_PROFILE" help:"Config profile name to apply from config.yaml (profiles section)." predictor:"profile-name"`
	Yolo         bool   `name:"yolo" env:"REMUDA_YOLO" negatable:"" help:"Ignore sandboxing/approvals for supported agents (Codex/Claude)."`
}

func (c *SessionResumeCmd) Validate() error {
	if err := c.VibeContainerOptions.Validate(); err != nil {
		return err
	}

	hasWorkspace := strings.TrimSpace(c.WorkspaceDir) != ""
	if hasWorkspace && c.Pick {
		return errors.New("cannot combine <workspace-dir> with --pick")
	}
	if !hasWorkspace && !c.Pick {
		return errors.New("exactly one of <workspace-dir> and --pick is required")
	}
	if c.Pick {
		return nil
	}
	if strings.TrimSpace(c.WorkspaceDir) == "" {
		return errors.New("<workspace-dir> cannot be blank")
	}
	return nil
}

func (c *SessionResumeCmd) Run(ctx Context, kctx *kong.Context) error {
	if c.Pick && !ctx.Remuda.IO.IsTerminal() {
		return errors.New("--pick requires an interactive TTY")
	}

	logger := logging.FromContext(ctx.ctx)
	var selected string
	if c.Pick {
		inactive, err := ctx.Remuda.InactiveWorkspacesWithIgnore(configuredPruneIgnorePatterns(ctx.ConfigFile))
		if err != nil {
			return errors.Wrap(err, "list inactive workspaces")
		}
		if len(inactive) == 0 {
			ctx.Remuda.IO.Outln("No inactive workspaces to resume.")
			return nil
		}

		picked, err := pickOneWorkspaceWithFZF(logger, envFromContext(ctx), inactive, ctx.Remuda.Config.ReposBaseDir)
		if err != nil {
			return errors.Wrap(err, "pick workspace")
		}
		if strings.TrimSpace(picked) == "" {
			return errors.New("no workspace selected")
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
		return errors.Wrapf(err, "invalid workspace %q", selectedAbs)
	}
	if c.Pick {
		if err := applyPerRepoOverlaysForPickedSessionResume(ctx, kctx, selectedAbs); err != nil {
			return err
		}
	}
	if err := applyDefaultsToSessionResume(c, kctx, ctx.ConfigFile, envFromContext(ctx)); err != nil {
		return err
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
	var invocationArgs []string
	if kctx != nil {
		invocationArgs = kctx.Args
	}
	if err := c.validatePromptUsage(c.Prompt, invocationArgs); err != nil {
		return err
	}

	agentName := strings.TrimSpace(c.Agent)
	if !flagExplicit(kctx, "agent") {
		agentName = resolveSessionResumeAgent(ctx.ConfigFile, envFromContext(ctx))
	}

	prompt := c.Prompt
	if prompt != "" {
		addedContext, err := c.AddedPromptContext(ctx, PromptContextInput{
			GitHubRepoSlug: repoSlugFromWorkspacePath(ctx, ctx.ConfigFile, selectedAbs),
		})
		if err != nil {
			return errors.Wrap(err, "adding prompt context")
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

	return ctx.Remuda.SessionResume(ctx.ctx, internal.SessionResumeCommand{
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
	})
}

func applyPerRepoOverlaysForPickedSessionResume(ctx Context, kctx *kong.Context, workspace string) error {
	cfg := ctx.ConfigFile
	if cfg == nil || len(cfg.PerRepo) == 0 {
		return nil
	}

	slug := normalizeRepoSlug(repoSlugFromWorkspacePath(ctx, cfg, workspace))
	if slug == "" {
		return nil
	}
	overlay, ok := cfg.PerRepo[slug]
	if !ok {
		return nil
	}

	mergeOverlayV1IntoConfig(cfg, overlay, true)
	if overlay.Repos != nil && len(overlay.Repos.Aliases) > 0 {
		github.MergeRepoAliases(overlay.Repos.Aliases)
	}

	var args []string
	if kctx != nil {
		args = kctx.Args
	}
	invocation := resolveInvocationAnalysisWithEnv(ctx, cfg, args, envFromContext(ctx))
	name, selectedSlug, source, ok := selectedProfileFromInvocation(invocation, cfg, slug)
	if !ok {
		return nil
	}
	if source != invocationProfileSourcePerRepo {
		return applyProfileOverlayByName(cfg, name)
	}
	return applyPerRepoProfileOverlayByName(cfg, selectedSlug, name)
}
