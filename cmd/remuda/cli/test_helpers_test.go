package cli

import (
	"context"
	"testing"

	"github.com/yendo-eng/remuda/internal"
)

func newTestContextWithEnv(t *testing.T, env EnvProvider, opts ...func(*Context)) Context {
	t.Helper()
	options := make([]func(*Context), 0, len(opts)+1)
	if env != nil {
		options = append(options, WithEnv(env))
	}
	options = append(options, opts...)
	return NewContext(context.Background(), internal.Remuda{}, options...)
}
