package smolvm_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/smolvm"
)

func lookupFrom(env map[string]string) smolvm.LookupEnvFunc {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

func TestAddDockerOpts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o600))
	env := lookupFrom(map[string]string{"GH_TOKEN": "tok"})

	tests := []struct {
		name    string
		opts    []string
		want    smolvm.Machine
		wantErr string
	}{
		{
			name: "directory binds become volumes",
			opts: []string{"-v", dir + ":/data:ro", "--volume=" + dir + ":/rw"},
			want: smolvm.Machine{Volumes: []smolvm.Volume{
				{Host: dir, Guest: "/data", ReadOnly: true},
				{Host: dir, Guest: "/rw"},
			}},
		},
		{
			name: "file binds become copies and tmpfs becomes a guest dir",
			opts: []string{"--tmpfs", "/root/.codex:rw,mode=0755", "-v", file + ":/root/.codex/auth.json:ro"},
			want: smolvm.Machine{
				GuestDirs: []string{"/root/.codex"},
				Files:     []smolvm.FileCopy{{Host: file, Guest: "/root/.codex/auth.json"}},
			},
		},
		{
			name: "env passthrough keeps only names set on the host",
			opts: []string{"-e", "GH_TOKEN", "-e", "UNSET_VAR", "--env=MODE=fast", "-e", "GH_TOKEN"},
			want: smolvm.Machine{
				Env:       []smolvm.EnvVar{{Name: "MODE", Value: "fast"}},
				SecretEnv: []string{"GH_TOKEN"},
			},
		},
		{name: "whole cpus map", opts: []string{"--cpus=4"}, want: smolvm.Machine{CPUs: 4}},
		{name: "fractional cpus fail", opts: []string{"--cpus=1.5"}, wantErr: "whole number"},
		{name: "unsupported option fails", opts: []string{"--network", "host"}, wantErr: `"--network" is not supported`},
		{name: "named volume fails", opts: []string{"-v", "cache:/cache"}, wantErr: "/host/path:/guest/path"},
		{name: "unknown mode fails", opts: []string{"-v", dir + ":/data:z"}, wantErr: `unsupported mode "z"`},
		{name: "missing host path fails", opts: []string{"-v", dir + "/nope:/data"}, wantErr: "no such file"},
		{name: "dangling flag fails", opts: []string{"-e"}, wantErr: "missing its value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var m smolvm.Machine
			err := m.AddDockerOpts(tt.opts, env)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, m)
		})
	}
}

func TestBindCredentials(t *testing.T) {
	t.Parallel()

	gh := smolvm.Credential{Name: "gh", Env: "GH_TOKEN", Hosts: []string{"api.github.com"}}
	tests := []struct {
		name          string
		env           map[string]string
		machine       smolvm.Machine
		creds         []smolvm.Credential
		wantSecretEnv []string
		wantErr       string
	}{
		{
			name:          "bound var is no longer passed through",
			env:           map[string]string{"GH_TOKEN": "tok", "OPENAI_API_KEY": "other"},
			machine:       smolvm.Machine{SecretEnv: []string{"GH_TOKEN", "OPENAI_API_KEY"}},
			creds:         []smolvm.Credential{gh},
			wantSecretEnv: []string{"OPENAI_API_KEY"},
		},
		{
			name:    "unbound var carrying the bound value fails",
			env:     map[string]string{"GH_TOKEN": "tok", "GITHUB_TOKEN": "tok"},
			machine: smolvm.Machine{SecretEnv: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
			creds:   []smolvm.Credential{gh},
			wantErr: "GITHUB_TOKEN carries the same value as credential-bound GH_TOKEN",
		},
		{
			name:    "unset bound var fails",
			env:     map[string]string{},
			creds:   []smolvm.Credential{gh},
			wantErr: "host env var GH_TOKEN is not set",
		},
		{
			name:    "literal env of a bound var fails",
			env:     map[string]string{"GH_TOKEN": "tok"},
			machine: smolvm.Machine{Env: []smolvm.EnvVar{{Name: "GH_TOKEN", Value: "x"}}},
			creds:   []smolvm.Credential{gh},
			wantErr: "also credential-bound",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := tt.machine
			err := m.BindCredentials(tt.creds, lookupFrom(tt.env))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantSecretEnv, m.SecretEnv)
		})
	}
}

// runLaunchScript runs script in bash against a fake smolvm that logs each
// invocation as one line and fails the subcommand named in FAIL_ON.
func runLaunchScript(t *testing.T, script, failOn string) (calls []string, exitCode int) {
	t.Helper()
	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "calls.log")
	fake := "#!/usr/bin/env bash\necho \"$*\" >> \"$CALLS_LOG\"\n[ \"$2\" = \"$FAIL_ON\" ] && exit 7\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "smolvm"), []byte(fake), 0o755))

	//nolint:gosec // G204: the test executes the generated launch script on purpose.
	cmd := exec.CommandContext(context.Background(), "bash", "-c", script+"; echo \"status=$?\"")
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"), "CALLS_LOG="+logPath, "FAIL_ON="+failOn)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	_, status, _ := strings.Cut(strings.TrimSpace(string(out)), "status=")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	exitCode, err = strconv.Atoi(status)
	require.NoError(t, err, string(out))
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n"), exitCode
}

func TestLaunchScript(t *testing.T) {
	t.Parallel()

	m := smolvm.Machine{
		Name:           "org-repo-ws",
		Image:          "/images/vibe-dev.smolmachine",
		WorkspaceHost:  "/host/ws with space",
		WorkspaceGuest: "/workspaces/ws-1234",
		Volumes:        []smolvm.Volume{{Host: "/host/cache", Guest: "/cache", ReadOnly: true}},
		Files:          []smolvm.FileCopy{{Host: "/host/.claude.json", Guest: "/root/.claude.json"}},
		GuestDirs:      []string{"/root/.codex"},
		Env:            []smolvm.EnvVar{{Name: "MODE", Value: "it's fast"}},
		SecretEnv:      []string{"GH_TOKEN"},
		Egress:         smolvm.Egress{AllowHosts: []string{"github.com"}, BlockPrivate: true},
		Credentials:    []smolvm.Credential{{Name: "gh", Env: "GH_TOKEN", Hosts: []string{"api.github.com", "github.com"}, SetHeader: "authorization"}},
		CPUs:           4,
	}
	script := m.LaunchScript("echo 'agent'")

	tests := []struct {
		name      string
		failOn    string
		wantCalls []string
		wantExit  int
	}{
		{
			name: "runs every step then deletes the machine",
			wantCalls: []string{
				"machine create --name org-repo-ws --from /images/vibe-dev.smolmachine -v /host/ws with space:/workspaces/ws-1234 " +
					"-w /workspaces/ws-1234 -v /host/cache:/cache:ro --cpus 4 --ssh-agent --net --allow-host github.com --block-private " +
					"--credential gh=GH_TOKEN@api.github.com,github.com:set-header=authorization -e MODE=it's fast",
				"machine start --name org-repo-ws",
				"machine exec --name org-repo-ws -- mkdir -p /root/.codex /root",
				"machine cp /host/.claude.json org-repo-ws:/root/.claude.json",
				"machine exec -it --name org-repo-ws -w /workspaces/ws-1234 --secret-env GH_TOKEN=GH_TOKEN -- bash -lc echo 'agent'",
				"machine delete --force --name org-repo-ws",
			},
		},
		{
			name:   "a failed step still deletes and keeps the exit status",
			failOn: "exec",
			wantCalls: []string{
				"machine create",
				"machine start",
				"machine exec",
				"machine delete",
			},
			wantExit: 7,
		},
		{
			name:      "create failure never deletes a machine of the same name",
			failOn:    "create",
			wantCalls: []string{"machine create"},
			wantExit:  7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls, exitCode := runLaunchScript(t, script, tt.failOn)
			require.Equal(t, tt.wantExit, exitCode)
			require.Len(t, calls, len(tt.wantCalls), strings.Join(calls, "\n"))
			for i, want := range tt.wantCalls {
				require.True(t, strings.HasPrefix(calls[i], want), "call %d:\n got %q\nwant prefix %q", i, calls[i], want)
			}
		})
	}
}
