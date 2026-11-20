package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/smolvm"
)

func TestVibeSmolvmRuntime(t *testing.T) {
	t.Parallel()

	const bothTokens = `
    credentials:
      - {name: gh, env: GH_TOKEN, hosts: [api.github.com]}
      - {name: gh-alt, env: GITHUB_TOKEN, hosts: [api.github.com]}`
	tests := []struct {
		name         string
		container    string
		args         []string
		wantErr      string
		wantInCmd    []string
		wantNotInCmd []string
	}{
		{
			name: "launches a machine with egress and credential bindings",
			container: `
    runtime: smolvm
    egress:
      allow_hosts: [github.com]
      block_private: true` + bothTokens,
			wantInCmd: []string{
				"'smolvm' 'machine' 'create' '--name'",
				"'--from' '/images/vibe-dev.smolmachine'",
				"'--ssh-agent'",
				"'--allow-host' 'github.com' '--block-private'",
				"'--credential' 'gh=GH_TOKEN@api.github.com'",
				"'smolvm' 'machine' 'exec' '-it'",
				"safe.directory",
				"smolvm machine delete --force --name 'org-repo-wk'",
			},
			wantNotInCmd: []string{
				"test-token",
				"/run/host-services/ssh-auth.sock",
				"'--secret-env' 'GH_TOKEN=GH_TOKEN'",
				"'--secret-env' 'GITHUB_TOKEN=GITHUB_TOKEN'",
			},
		},
		{
			name: "binding one of two vars holding the same token fails",
			container: `
    runtime: smolvm
    credentials:
      - {name: gh, env: GH_TOKEN, hosts: [api.github.com]}`,
			wantErr: "GITHUB_TOKEN carries the same value as credential-bound GH_TOKEN",
		},
		{
			name: "egress on the docker runtime fails",
			container: `
    egress:
      allow_hosts: [github.com]`,
			wantErr: `container runtime is "docker"; they require runtime smolvm`,
		},
		{
			name:      "docker options smolvm cannot express fail",
			container: "\n    runtime: smolvm",
			args:      []string{"--container-opt", "--network host"},
			wantErr:   `container option "--network" is not supported by the smolvm runtime`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			workspaceRoot := t.TempDir()
			workspace := filepath.Join(workspaceRoot, "org", "repo", "wk")
			testutils.InitWorkspace(t, workspace)

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(`
version: 1
defaults:
  container:
    image: /images/vibe-dev.smolmachine`+tt.container+"\n"), 0o644))

			sessionMgr := &testutils.MockMultiplexer{}
			h := testutils.NewHarness(t,
				testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
				testutils.WithMultiplexer(sessionMgr),
			)
			h.SetEnv("REMUDA_CONFIG", configPath)
			h.SetEnv("GH_TOKEN", "test-token")

			res := h.Run(append([]string{"vibe", "--in", workspace, "--container"}, append(tt.args, "prompt")...)...)
			recorded := sessionMgr.FindSession(session.SessionNameFromWorkspaceName(workspace))
			if tt.wantErr != "" {
				require.ErrorContains(t, res.Err, tt.wantErr)
				require.Nil(t, recorded)
				return
			}
			require.NoError(t, res.Err, res.String())
			require.NotNil(t, recorded)
			for _, want := range tt.wantInCmd {
				require.Contains(t, recorded.CommandRan, want)
			}
			for _, unwanted := range tt.wantNotInCmd {
				require.NotContains(t, recorded.CommandRan, unwanted)
			}
		})
	}
}

func TestSessionSmolvmMachineLifecycle(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*testutils.Harness, *smolvm.Mock, *docker.Mock, string, string) {
		t.Helper()
		workspaceRoot := t.TempDir()
		workspace := filepath.Join(workspaceRoot, "org", "repo", "vm")
		testutils.InitWorkspace(t, workspace)
		sessionName := session.SessionNameFromWorkspaceName(workspace)
		machineName := docker.ContainerNameFromSession(sessionName)

		vm := &smolvm.Mock{Machines: map[string]bool{machineName: true}}
		dock := &docker.Mock{Running: true}
		multiplexer := &testutils.MockMultiplexer{}
		require.NoError(t, multiplexer.Start(session.AgentStart{SessionName: sessionName}))
		h := testutils.NewHarness(t,
			testutils.WithRemudaConfig(internal.Config{ReposBaseDir: workspaceRoot}),
			testutils.WithMultiplexer(multiplexer),
			testutils.WithDocker(dock),
			testutils.WithSmolvm(vm),
		)
		return h, vm, dock, sessionName, machineName
	}

	t.Run("shell execs into the running machine, not docker", func(t *testing.T) {
		t.Parallel()
		h, vm, dock, sessionName, machineName := setup(t)

		h.RunOK("session", "shell", "--name", sessionName)

		require.Len(t, vm.Execs, 1)
		require.Equal(t, machineName, vm.Execs[0].Machine)
		require.Contains(t, vm.Execs[0].Command, "${SHELL:-/bin/bash}")
		require.Empty(t, dock.Execs)
	})

	t.Run("kill deletes the machine", func(t *testing.T) {
		t.Parallel()
		h, vm, _, sessionName, machineName := setup(t)

		h.RunOK("session", "kill", "--name", sessionName)

		require.Equal(t, []string{machineName}, vm.Deleted)
	})
}
