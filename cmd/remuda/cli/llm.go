package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/yendo-eng/remuda/internal/llm"
	"github.com/yendo-eng/remuda/internal/logging"
)

type LLMSlugifyCmd struct {
	SlugifyOptions
	Prompt string
}

func (a *app) llmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "LLM utilities (experimental).",
	}

	c := &LLMSlugifyCmd{}
	var fl *flagSet
	slugify := &cobra.Command{
		Use:   "slugify <prompt>",
		Short: "Turn prompt into a short kebab-case slug.",
		Args:  cobra.ExactArgs(1),
	}
	fl = newFlagSet(slugify.Flags())
	c.SlugifyOptions.register(slugify, fl)
	a.simpleCmd(slugify, fl, func(args []string) error {
		c.Prompt = args[0]
		return c.Run(*a.kctx)
	})

	cmd.AddCommand(slugify)
	return cmd
}

func (c *LLMSlugifyCmd) Run(ctx Context) error {
	service := llm.NewFromEnvProvider(
		ctx.Remuda.Env,
		llm.WithSlugifyReasoningLevel(c.SlugifyReasoningLevel),
		llm.WithLogger(logging.FromContext(ctx.ctx)),
	)
	slug, err := service.Slugify(context.Background(), c.Prompt)
	if err != nil {
		return err
	}
	ctx.Remuda.IO.Outln(slug)
	return nil
}
