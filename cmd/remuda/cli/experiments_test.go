package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
)

func TestValidateExperiments_RejectsUnknown(t *testing.T) {
	t.Parallel()

	_, err := validateExperiments("use-prompts-context-wrapper,typo", "--experiments")
	require.ErrorContains(t, err, `--experiments: unknown experiment "typo"`)
	require.ErrorContains(t, err, "valid: use-prompts-context-wrapper")
}

func TestValidateExperiments_AllowsRetired(t *testing.T) {
	t.Parallel()

	retired, err := validateExperiments("auto-workspace-name", "defaults.experiments")
	require.NoError(t, err)
	require.Equal(t, []string{"auto-workspace-name"}, retired)
}

func TestValidateExperiments_UsesFlexibleListParsing(t *testing.T) {
	t.Parallel()

	retired, err := validateExperiments(" USE-PROMPTS-CONTEXT-WRAPPER\nauto-workspace-name ", "REMUDA_EXPERIMENTS")
	require.NoError(t, err)
	require.Equal(t, []string{"auto-workspace-name"}, retired)
}

func TestVibeCheckUnknownExperimentFailsBeforeCommandValidation(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	ctx := NewContext(
		context.Background(),
		internal.Remuda{},
		WithEnv(EnvMap{}),
		Stderr(&stderr),
	)

	err := Run(ctx, []string{"vibe-check", "--experiments", "not-real"})
	require.ErrorContains(t, err, `--experiments: unknown experiment "not-real"`)
	require.NotContains(t, err.Error(), "branch is required")
}

func TestVibeCheckRetiredExperimentWarnsAndContinues(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	ctx := NewContext(
		context.Background(),
		internal.Remuda{},
		WithEnv(EnvMap{}),
		Stderr(&stderr),
	)

	err := Run(ctx, []string{"vibe-check", "--experiments", "auto-workspace-name"})
	require.ErrorContains(t, err, "repository is not configured")
	require.Contains(t, stderr.String(), `warning: experiment "auto-workspace-name" was mainlined and is now a no-op; remove it`)
}
