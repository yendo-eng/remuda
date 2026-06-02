package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
)

func TestLLMSlugify_LocalFallback(t *testing.T) {
	t.Parallel()
	parser, _ := testutils.RemudaParser(t)

	// Capture output
	var out bytes.Buffer
	env := cli.EnvMap{}
	k := internal.NewRemuda(internal.Config{}, nil, nil, nil, nil, nil)
	kctx := cli.NewContext(t.Context(), k, cli.Stdout(&out), cli.WithEnv(env))

	ctx, err := parser.Parse([]string{"llm", "slugify", "Fix: Allow --repo utils in vibe start"})
	require.NoError(t, err)
	require.NoError(t, ctx.Run(kctx))

	got := out.String()
	require.Contains(t, got, "fix-allow-repo-utils-in-vibe-start")
}
