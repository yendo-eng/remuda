package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/session"
)

type captureEnvSession struct {
	env []string
}

func (c *captureEnvSession) Name() string                            { return string(session.SessionManagerTmux) }
func (c *captureEnvSession) Start(sessionName, command string) error { return nil }
func (c *captureEnvSession) StartWithEnv(sessionName, command string, envValues []string) error {
	c.env = append([]string{}, envValues...)
	return nil
}
func (c *captureEnvSession) List() ([]session.SessionInfo, error) { return nil, nil }
func (c *captureEnvSession) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (c *captureEnvSession) Attach(name string) error                                   { return nil }
func (c *captureEnvSession) ReadBuffer(name string, lines int) (string, error)          { return "", nil }
func (c *captureEnvSession) Send(name string, payload string, appendNewline bool) error { return nil }
func (c *captureEnvSession) Kill(name string) error                                     { return nil }

func TestStartSessionWithEnvAddsPathWhenMissing(t *testing.T) {
	const hostPath = "/tmp/host/bin"
	t.Setenv("PATH", hostPath)

	mgr := &captureEnvSession{}
	provider := env.StaticProvider{Values: map[string]string{"FOO": "bar"}}

	require.NoError(t, startSessionWithEnv(mgr, "sess", "cmd", provider))

	value, ok := envValue(mgr.env, "PATH")
	require.True(t, ok)
	require.Equal(t, hostPath, value)
	value, ok = envValue(mgr.env, "FOO")
	require.True(t, ok)
	require.Equal(t, "bar", value)
}

func TestStartSessionWithEnvKeepsProvidedPath(t *testing.T) {
	t.Setenv("PATH", "/tmp/host/bin")

	mgr := &captureEnvSession{}
	provider := env.StaticProvider{Values: map[string]string{"PATH": "/custom/bin"}}

	require.NoError(t, startSessionWithEnv(mgr, "sess", "cmd", provider))

	value, ok := envValue(mgr.env, "PATH")
	require.True(t, ok)
	require.Equal(t, "/custom/bin", value)
}

func envValue(values []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range values {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	return "", false
}
