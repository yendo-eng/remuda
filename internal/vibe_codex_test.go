package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestVibe_DetachedHerdrCoreCodexRunsWithoutDaemon(t *testing.T) {
	t.Parallel()

	sm := namedCaptureMultiplexer{captureMultiplexer: &captureMultiplexer{}, name: session.MultiplexerHerdrCore}
	k := Remuda{
		Git:         &fakeGit{},
		Multiplexer: sm,
		IO:          DefaultIO(),
		Env:         env.StaticProvider{Values: map[string]string{"PATH": "/usr/bin:/bin"}},
	}

	err := k.Vibe(context.Background(), VibeCommand{
		Agent:             "codex",
		AgentArgs:         []string{"--search"},
		Prompt:            "fix it",
		Detached:          true,
		ExistingWorkspace: t.TempDir(),
	})
	require.NoError(t, err)
	require.Contains(t, sm.startCmd, "'--search' '--no-daemon' -- 'fix it'")
}
