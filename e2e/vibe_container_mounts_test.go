package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestVibeContainerMountsAgentState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agent     string
		homePaths []string
		want      func(home string) []string
	}{
		{
			name:      "codex account login",
			agent:     "codex",
			homePaths: []string{".codex/auth.json", ".codex/prompts/"},
			want: func(home string) []string {
				return []string{"-v", filepath.Join(home, ".codex") + ":/root/.codex:rw"}
			},
		},
		{
			name:      "codex without account login or API key",
			agent:     "codex",
			homePaths: []string{".codex/prompts/", ".codex/rules/", ".codex/skills/"},
			want: func(home string) []string {
				codexDir := filepath.Join(home, ".codex")
				return []string{
					"-v", filepath.Join(codexDir, "prompts") + ":/root/.codex/prompts:ro",
					"-v", filepath.Join(codexDir, "rules") + ":/root/.codex/rules:ro",
					"-v", filepath.Join(codexDir, "skills") + ":/root/.codex/skills:ro",
					"-v", filepath.Join(codexDir, "history.jsonl") + ":/root/.codex/history.jsonl:rw",
					"-v", filepath.Join(codexDir, "sessions") + ":/root/.codex/sessions:rw",
				}
			},
		},
		{
			name:      "opencode",
			agent:     "opencode",
			homePaths: []string{".local/share/opencode/"},
			want: func(home string) []string {
				return []string{"-v", filepath.Join(home, ".local", "share", "opencode") + ":/root/.local/share/opencode:rw"}
			},
		},
		{
			name:      "bash",
			agent:     "bash",
			homePaths: []string{".claude/", ".claude.json", ".local/share/opencode/"},
			want: func(home string) []string {
				return []string{
					"-v", filepath.Join(home, ".codex", "history.jsonl") + ":/root/.codex/history.jsonl:rw",
					"-v", filepath.Join(home, ".codex", "sessions") + ":/root/.codex/sessions:rw",
					"-v", filepath.Join(home, ".local", "share", "opencode") + ":/root/.local/share/opencode:rw",
					"-v", filepath.Join(home, ".claude") + ":/root/.claude:rw",
					"-v", filepath.Join(home, ".claude.json") + ":/root/.claude.json:rw",
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			home, mounts := containerAgentStateMounts(t, tt.agent, "", tt.homePaths)
			require.Equal(t, tt.want(home), mounts)
		})
	}
}

func TestVibeCodexContainerMountsAPIKeyAuth(t *testing.T) {
	t.Parallel()

	home, mounts := containerAgentStateMounts(t, "codex", "sk-test", []string{
		".codex/auth.json",
		".codex/config.toml",
		".codex/prompts/",
		".codex/rules/",
		".codex/skills/",
		".codex/AGENTS.md",
	})

	var authDir string
	for _, mount := range mounts {
		if src, ok := strings.CutSuffix(mount, ":/root/.codex/auth.json:ro"); ok {
			authDir = filepath.Dir(src)
		}
	}
	// The synthesized auth dir is created in the OS temp dir and never removed.
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(authDir)) })
	codexDir := filepath.Join(home, ".codex")
	require.Equal(t, []string{
		"--tmpfs", "/root/.codex:rw,mode=0755",
		"-v", filepath.Join(authDir, "auth.json") + ":/root/.codex/auth.json:ro",
		"-v", filepath.Join(codexDir, "history.jsonl") + ":/root/.codex/history.jsonl:rw",
		"-v", filepath.Join(codexDir, "sessions") + ":/root/.codex/sessions:rw",
		"-v", filepath.Join(authDir, "config.toml") + ":/root/.codex/config.toml:ro",
		"-v", filepath.Join(codexDir, "prompts") + ":/root/.codex/prompts:ro",
		"-v", filepath.Join(codexDir, "rules") + ":/root/.codex/rules:ro",
		"-v", filepath.Join(codexDir, "skills") + ":/root/.codex/skills:ro",
		"-v", filepath.Join(codexDir, "AGENTS.md") + ":/root/.codex/AGENTS.md:ro",
	}, mounts)

	auth, err := os.ReadFile(filepath.Join(authDir, "auth.json"))
	require.NoError(t, err)
	require.Equal(t, `{"OPENAI_API_KEY":"sk-test"}`+"\n", string(auth))
	hostConfig, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	require.NoError(t, err)
	config, err := os.ReadFile(filepath.Join(authDir, "config.toml"))
	require.NoError(t, err)
	require.Equal(t, string(hostConfig), string(config))
}

// containerAgentStateMounts runs a containerized vibe of agent with
// OPENAI_API_KEY set to apiKey and homePaths created under the home dir (a
// trailing slash makes a directory). It returns the home dir and the -v and
// --tmpfs options of the docker run command that target agent state in the
// container, in order.
func containerAgentStateMounts(t *testing.T, agent, apiKey string, homePaths []string) (string, []string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
	testutils.InitWorkspace(t, workspace)

	sessionMgr := &testutils.MockMultiplexer{}
	h := testutils.NewHarness(t,
		testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
		testutils.WithMultiplexer(sessionMgr),
		testutils.WithDocker(&docker.Mock{Running: true}),
	)
	h.SetWorkingDir(workspaceRoot)
	h.SetEnv("GH_TOKEN", "gh-token")
	h.SetEnv("SSH_AUTH_SOCK", "")
	h.SetEnv("OPENAI_API_KEY", apiKey)
	for _, p := range homePaths {
		path := filepath.Join(h.HomeDir, p)
		if strings.HasSuffix(p, "/") {
			require.NoError(t, os.MkdirAll(path, 0o755))
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(p), 0o600))
	}

	h.RunOK("vibe", "--in", workspace, "--container", "--container-name", "img", "--agent", agent, "mount agent state")
	recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
	require.NotNil(t, recorded)

	var mounts []string
	for _, match := range regexp.MustCompile(`'(-v|--tmpfs)' '([^']*)'`).FindAllStringSubmatch(recorded.CommandRan, -1) {
		flag, value := match[1], match[2]
		src, target := "", value
		if flag == "-v" {
			src, target, _ = strings.Cut(value, ":")
		}
		if !strings.HasPrefix(target, "/root/.codex") && !strings.HasPrefix(target, "/root/.claude") && !strings.HasPrefix(target, "/root/.local/share/opencode") {
			continue
		}
		if src != "" {
			// A missing source would be created by docker as an empty directory.
			_, err := os.Stat(src)
			require.NoError(t, err)
		}
		mounts = append(mounts, flag, value)
	}
	return h.HomeDir, mounts
}
