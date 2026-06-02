package cli

import (
	"context"

	"github.com/yendo-eng/remuda/internal/llm"
	"github.com/yendo-eng/remuda/internal/logging"
)

// LLMRootCmd groups experimental LLM helpers.
type LLMRootCmd struct {
	Slugify LLMSlugifyCmd `cmd:"" help:"Turn prompt into a short kebab-case slug."`
}

type LLMSlugifyCmd struct {
	SlugifyOptions `embed:""`
	Prompt         string `arg:"" name:"prompt" help:"Prompt to convert into a slug."`
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
