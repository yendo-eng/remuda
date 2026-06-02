package session_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	t.Run("tmux explicit", func(t *testing.T) {
		mgr := session.NewSessionManager("tmux")
		require.Contains(t, fmt.Sprintf("%T", mgr), "defaultTmuxManager")
	})

	t.Run("zellij", func(t *testing.T) {
		mgr := session.NewSessionManager("zellij")
		require.Contains(t, fmt.Sprintf("%T", mgr), "zellijManager")
	})

	t.Run("unknown", func(t *testing.T) {
		require.Panics(t, func() { session.NewSessionManager("bogus") })
	})
}
