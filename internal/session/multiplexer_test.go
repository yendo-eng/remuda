package session_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestNewMultiplexer(t *testing.T) {
	t.Parallel()

	t.Run("tmux explicit", func(t *testing.T) {
		mgr := session.NewMultiplexer("tmux", logging.DefaultLogger())
		require.Contains(t, fmt.Sprintf("%T", mgr), "tmux")
	})

	t.Run("zellij", func(t *testing.T) {
		mgr := session.NewMultiplexer("zellij", logging.DefaultLogger())
		require.Contains(t, fmt.Sprintf("%T", mgr), "zellij")
	})

	t.Run("herdr-core", func(t *testing.T) {
		mgr := session.NewMultiplexer("herdr-core", logging.DefaultLogger())
		require.Contains(t, fmt.Sprintf("%T", mgr), "herdrCore")
	})

	t.Run("rex", func(t *testing.T) {
		mgr := session.NewMultiplexer("rex", logging.DefaultLogger())
		require.Contains(t, fmt.Sprintf("%T", mgr), "rex")
	})

	t.Run("unknown", func(t *testing.T) {
		require.Panics(t, func() { session.NewMultiplexer("bogus", logging.DefaultLogger()) })
	})
}
