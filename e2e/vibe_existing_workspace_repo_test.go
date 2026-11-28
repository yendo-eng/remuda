package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

// Not parallel: config resolution mutates the global repo alias registry.
func TestVibeInUsesWorkspaceRepoOverlay(t *testing.T) {
	github.ResetRepoAliases()
	t.Cleanup(github.ResetRepoAliases)

	config := `
version: 1
repos:
  aliases:
    death-star: https://github.com/acme/death-star.git
    remuda: https://github.com/yendo-eng/remuda.git
  default_repo: death-star
per_repo:
  acme/death-star:
    defaults:
      container:
        opts: ["--death-star-only"]
        inherit_env: [JIRA_API_TOKEN]
  yendo-eng/remuda:
    defaults:
      container:
        opts: ["--remuda-only"]
        inherit_env: [REMUDA_ONLY_TOKEN]
`

	tests := []struct {
		name           string
		repoArgs       []string
		repoURLArgs    []string
		envRepo        string
		envRepoURL     string
		outsideBase    bool
		wantErr        string
		wantOpts       []string
		wantInheritEnv []string
	}{
		{
			name:           "workspace repo overrides config default",
			wantOpts:       []string{"--remuda-only"},
			wantInheritEnv: []string{"REMUDA_ONLY_TOKEN"},
		},
		{
			name:     "explicit repo mismatch is rejected",
			repoArgs: []string{"--repo", "death-star"},
			wantErr:  "--repo selects acme/death-star, but workspace belongs to yendo-eng/remuda",
		},
		{
			name:        "explicit repo URL mismatch is rejected",
			repoURLArgs: []string{"--repo-url", "https://github.com/acme/death-star.git"},
			wantErr:     "--repo-url selects acme/death-star, but workspace belongs to yendo-eng/remuda",
		},
		{
			name:        "both explicit flags are validated",
			repoArgs:    []string{"--repo", "death-star"},
			repoURLArgs: []string{"--repo-url", "https://github.com/yendo-eng/remuda.git"},
			wantErr:     "--repo selects acme/death-star, but workspace belongs to yendo-eng/remuda",
		},
		{
			name:        "explicit repo URL without a slug is rejected",
			repoURLArgs: []string{"--repo-url", "/srv/git/utils.git"},
			wantErr:     "--repo-url cannot be matched to workspace repo yendo-eng/remuda",
		},
		{
			name:           "explicit repo flag overrides environment repo URL",
			repoArgs:       []string{"--repo", "remuda"},
			envRepoURL:     "https://github.com/acme/death-star.git",
			wantOpts:       []string{"--remuda-only"},
			wantInheritEnv: []string{"REMUDA_ONLY_TOKEN"},
		},
		{
			name:           "environment repo overrides workspace",
			envRepo:        "death-star",
			wantOpts:       []string{"--death-star-only"},
			wantInheritEnv: []string{"JIRA_API_TOKEN"},
		},
		{
			name:           "environment repo URL overrides workspace",
			envRepoURL:     "https://github.com/acme/death-star.git",
			wantOpts:       []string{"--death-star-only"},
			wantInheritEnv: []string{"JIRA_API_TOKEN"},
		},
		{
			name:           "workspace outside repos base keeps config default",
			outsideBase:    true,
			wantOpts:       []string{"--death-star-only"},
			wantInheritEnv: []string{"JIRA_API_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reposBaseDir := t.TempDir()
			workspaceRoot := reposBaseDir
			if tt.outsideBase {
				workspaceRoot = t.TempDir()
			}
			workspace := filepath.Join(workspaceRoot, "yendo-eng", "remuda", "wk")
			testutils.InitWorkspace(t, workspace)

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(config), 0o644))

			sessionMgr := &testutils.MockMultiplexer{}
			h := testutils.NewHarness(t,
				testutils.WithRemudaConfig(internal.Config{ReposBaseDir: reposBaseDir}),
				testutils.WithMultiplexer(sessionMgr),
				testutils.WithDocker(&docker.Mock{Running: true}),
			)
			h.SetEnv("REMUDA_CONFIG", configPath)
			h.SetEnv("GH_TOKEN", "test-token")
			h.SetEnv("SSH_AUTH_SOCK", "")
			if tt.envRepo != "" {
				h.SetEnv("REMUDA_DEFAULT_REPO", tt.envRepo)
			}
			if tt.envRepoURL != "" {
				h.SetEnv("REMUDA_DEFAULT_REPO_URL", tt.envRepoURL)
			}

			args := []string{
				"vibe",
				"--in", workspace,
				"--container",
				"--container-name", "ghcr.io/acme/vibe-dev:latest",
				"--agent-cmd", "true",
			}
			args = append(args, tt.repoArgs...)
			args = append(args, tt.repoURLArgs...)
			args = append(args, "prompt")
			res := h.Run(args...)
			if tt.wantErr != "" {
				require.ErrorContains(t, res.Err, tt.wantErr)
				require.Nil(t, sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace)))
				return
			}
			require.NoError(t, res.Err, res.String())

			recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
			require.NotNil(t, recorded)
			for _, opt := range tt.wantOpts {
				require.Contains(t, recorded.CommandRan, opt)
			}

			manifest, ok, err := internal.ReadSessionManifest(workspace)
			require.NoError(t, err)
			require.True(t, ok)
			require.NotNil(t, manifest.Defaults.Container)
			require.NotNil(t, manifest.Defaults.Container.Opts)
			require.Equal(t, tt.wantOpts, *manifest.Defaults.Container.Opts)
			require.NotNil(t, manifest.Defaults.Container.InheritEnv)
			require.Equal(t, tt.wantInheritEnv, *manifest.Defaults.Container.InheritEnv)
		})
	}
}
