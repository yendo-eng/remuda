package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/session"
)

type noopGit struct{}

func (noopGit) Clone(repoURL, dir string) error                          { return nil }
func (noopGit) Pull(dir string) error                                    { return nil }
func (noopGit) WorktreeAdd(dir, branch string, args ...string) error     { return nil }
func (noopGit) WorktreeRemove(dir string, args ...string) error          { return nil }
func (noopGit) Checkout(dir string, args ...string) error                { return nil }
func (noopGit) ShowRef(dir, ref string, opts ...string) error            { return nil }
func (noopGit) RevParse(dir, rev string, opts ...string) (string, error) { return "", nil }
func (noopGit) Branch(dir string, args ...string) error                  { return nil }

var _ git.Git = noopGit{}

func TestRun_WiresSessionManagerFlag(t *testing.T) {
	t.Parallel()

	for _, manager := range []string{"zellij", "herdr"} {
		manager := manager
		t.Run(manager, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			var errBuf bytes.Buffer
			k := internal.NewRemuda(
				internal.Config{ReposBaseDir: t.TempDir()},
				noopGit{},
				nil,
				nil,
				nil,
				nil,
				internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &errBuf}),
			)
			ctx := NewContext(context.Background(), k,
				WithEnv(EnvMap{}),
				WithHomeDir(t.TempDir()),
				WithWorkingDir(t.TempDir()),
				WithMultiplexerFactory(func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
					return stubNamedMultiplexer{name: string(name)}
				}),
			)

			require.NoError(t, Run(ctx, []string{"--session-manager", manager, "session", "list"}))
			require.Contains(t, out.String(), "("+manager+")")
		})
	}
}

func TestRun_AggregatesMultiplexersWhenExperimentEnabled(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: t.TempDir()},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &bytes.Buffer{}}),
	)
	ctx := NewContext(context.Background(), k,
		WithEnv(EnvMap{}),
		WithHomeDir(t.TempDir()),
		WithWorkingDir(t.TempDir()),
		WithMultiplexerFactory(func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
			return stubNamedMultiplexer{
				name:     string(name),
				sessions: []session.SessionInfo{{Name: "org/repo/" + string(name)}},
			}
		}),
	)

	require.NoError(t, Run(ctx, []string{"--experiments", "aggregate-multiplexer", "--session-manager", "herdr", "session", "list"}))
	require.Equal(t, "org/repo/tmux\norg/repo/zellij\norg/repo/herdr\n", out.String())
}

func TestRun_ListsMultiplexerInJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	k := internal.NewRemuda(
		internal.Config{ReposBaseDir: t.TempDir()},
		noopGit{},
		nil,
		nil,
		nil,
		nil,
		internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &bytes.Buffer{}}),
	)
	ctx := NewContext(context.Background(), k,
		WithEnv(EnvMap{}),
		WithHomeDir(t.TempDir()),
		WithWorkingDir(t.TempDir()),
		WithMultiplexerFactory(func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
			return stubNamedMultiplexer{
				name:     string(name),
				sessions: []session.SessionInfo{{Name: "org/repo/" + string(name), Multiplexer: string(name)}},
			}
		}),
	)

	require.NoError(t, Run(ctx, []string{"--experiments", "aggregate-multiplexer", "--session-manager", "herdr", "session", "list", "--json"}))
	require.JSONEq(t, `[
  {
    "Name": "org/repo/tmux",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "tmux"
  },
  {
    "Name": "org/repo/zellij",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "zellij"
  },
  {
    "Name": "org/repo/herdr",
    "Attached": false,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "Multiplexer": "herdr"
  }
]`, out.String())
}

func TestRun_AggregatesSessionNameCompletionWhenExperimentEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        EnvMap
		configYAML string
		args       []string
	}{
		{
			name: "environment",
			env: EnvMap{
				"REMUDA_EXPERIMENTS":     "aggregate-multiplexer",
				"REMUDA_SESSION_MANAGER": "herdr",
			},
			args: []string{"__complete", "session", "kill", "--name", ""},
		},
		{
			name:       "config",
			configYAML: "version: 1\ndefaults:\n  experiments:\n    - aggregate-multiplexer\n",
			args:       []string{"__complete", "--session-manager", "herdr", "session", "attach", "--name", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			if tt.configYAML != "" {
				configPath := filepath.Join(home, ".config", "remuda", "config.yaml")
				require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
				require.NoError(t, os.WriteFile(configPath, []byte(tt.configYAML), 0o644))
			}
			var out bytes.Buffer
			k := internal.NewRemuda(
				internal.Config{ReposBaseDir: t.TempDir()},
				noopGit{},
				nil,
				nil,
				nil,
				nil,
				internal.WithIO(internal.IO{In: bytes.NewBuffer(nil), Out: &out, Err: &bytes.Buffer{}}),
			)
			ctx := NewContext(context.Background(), k,
				WithEnv(tt.env),
				WithHomeDir(home),
				WithWorkingDir(home),
				WithMultiplexerFactory(func(name session.SupportedMultiplexer, _ zerolog.Logger) session.Multiplexer {
					return stubNamedMultiplexer{
						name:     string(name),
						sessions: []session.SessionInfo{{Name: "org/repo/" + string(name)}},
					}
				}),
			)

			require.NoError(t, Run(ctx, tt.args))
			require.Contains(t, out.String(), "org/repo/tmux\n")
			require.Contains(t, out.String(), "org/repo/zellij\n")
			require.Contains(t, out.String(), "org/repo/herdr\n")
		})
	}
}

type stubNamedMultiplexer struct {
	name     string
	sessions []session.SessionInfo
}

func (m stubNamedMultiplexer) Name() string { return m.name }
func (m stubNamedMultiplexer) Start(sessionName, command string) error {
	return nil
}
func (m stubNamedMultiplexer) List() ([]session.SessionInfo, error) {
	return m.sessions, nil
}
func (m stubNamedMultiplexer) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}
func (m stubNamedMultiplexer) Attach(name string) error { return nil }
func (m stubNamedMultiplexer) ReadBuffer(name string, lines int) (string, error) {
	return "", nil
}
func (m stubNamedMultiplexer) Send(name string, payload string, appendNewline bool) error {
	return nil
}
func (m stubNamedMultiplexer) Kill(name string) error { return nil }
