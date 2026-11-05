package util

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestCmdLogsOnlyCommandName(t *testing.T) {
	t.Parallel()

	for name, build := range map[string]func(zerolog.Logger, string, ...string) *exec.Cmd{
		"Cmd": Cmd,
		"CmdWithEnv": func(logger zerolog.Logger, name string, args ...string) *exec.Cmd {
			return CmdWithEnv(logger, nil, name, args...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			build(zerolog.New(&logs), "git", "clone", "https://user:secret@example.com/org/repo.git")
			require.Contains(t, logs.String(), `"cmd":"git"`)
			require.NotContains(t, logs.String(), "secret")
		})
	}
}
