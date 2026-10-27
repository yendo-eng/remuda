package session

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/quick"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

const (
	rexTestSession = "yendo/remuda/rm-2l5u"
	rexTestID      = "session:01a17ac2-2031-7e48-9b9c-43dd6612bef5"
	rexOtherID     = "session:01a17ac2-4f00-7d11-8b22-0c33d44e55f6"
	rexLeftoverID  = "session:01a17a00-0000-7c00-8a00-000000000001"
	// rexTestVersion is what rex version prints for the release Remuda was
	// verified against.
	rexTestVersion = "rex 0.0.1792787366-g623b2d\ncommit: 623b2d17d416b146ce9993a7cd2dfce964ea60e5\nbuilt: 2026-10-23T20:29:26Z\napi: revision 7 (supports 0 to 7)"
	rexTestWindow  = `{"window_id":"window:agent","label":"agent","focused_block_id":"block:agent","layers":[]}`
)

var rexTestCreatedAt = time.Date(2026, time.October, 27, 0, 28, 25, 9_000_000, time.UTC)

func TestRexFind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sessions []rexSession
		find     string
		want     SessionInfo
		wantErr  error
	}{
		{
			name:     "exact label",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 1}},
			find:     rexTestSession,
			want:     SessionInfo{Name: rexTestSession, CreatedAt: rexTestCreatedAt, Multiplexer: "rex"},
		},
		{
			name:     "label differing only in case",
			sessions: []rexSession{{ID: rexTestID, Label: "Yendo/Remuda/RM-2L5U", Windows: 1}},
			find:     rexTestSession,
			wantErr:  ErrSessionNotFound,
		},
		{
			name:     "shared label",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 1}, {ID: rexOtherID, Label: rexTestSession, Windows: 2}},
			find:     rexTestSession,
			wantErr:  AmbiguousSessionError{Name: rexTestSession, IDs: []string{rexTestID, rexOtherID}},
		},
		{
			name:     "empty session left by an exited agent",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 0}},
			find:     rexTestSession,
			wantErr:  ErrSessionNotFound,
		},
		{
			name:     "live session beside an empty one",
			sessions: []rexSession{{ID: rexOtherID, Label: rexTestSession, Windows: 0}, {ID: rexTestID, Label: rexTestSession, Windows: 1}},
			find:     rexTestSession,
			want:     SessionInfo{Name: rexTestSession, CreatedAt: rexTestCreatedAt, Multiplexer: "rex"},
		},
		{
			name:     "label outside the Remuda naming scheme",
			sessions: []rexSession{{ID: rexTestID, Label: "Research", Windows: 1}},
			find:     "Research",
			wantErr:  ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newFakeRex(t, t.TempDir(), rexSessionBranches(tt.sessions...))
			info, err := r.Find(tt.find)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, info)
		})
	}
}

func TestRexList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newFakeRex(t, dir, rexSessionBranches(
		rexSession{ID: rexTestID, Label: rexTestSession, Windows: 1},
		rexSession{ID: rexOtherID, Label: "yendo/remuda/exited", Windows: 0},
		rexSession{ID: rexLeftoverID, Label: "Research", Windows: 2},
		rexSession{ID: "session:main", Label: "yendo/remuda/unknown-age", Windows: 1},
	))

	sessions, err := r.List()
	require.NoError(t, err)
	require.Equal(t, []SessionInfo{
		{Name: rexTestSession, CreatedAt: rexTestCreatedAt, Multiplexer: "rex"},
		{Name: "yendo/remuda/unknown-age", Multiplexer: "rex"},
	}, sessions)
	// Only sessions with Remuda labels are inspected.
	require.Equal(t, []string{
		"rex --autostart=false version",
		"rex --autostart=false ls --json",
		"rex --autostart=false session inspect " + rexTestID + " --json",
		"rex --autostart=false session inspect " + rexOtherID + " --json",
		"rex --autostart=false session inspect session:main --json",
	}, rexCalls(t, dir))
}

func TestRexListInspectFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		inspect string
		want    []SessionInfo
		wantErr string
	}{
		{
			name:    "session killed after ls",
			inspect: `echo 'Error: session target "` + rexOtherID + `" not found' >&2; exit 3`,
			want:    []SessionInfo{{Name: rexTestSession, CreatedAt: rexTestCreatedAt, Multiplexer: "rex"}},
		},
		{
			name:    "other failure",
			inspect: `echo 'Error: lost connection to the server' >&2; exit 1`,
			wantErr: "rex: Error: lost connection to the server: exit status 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newFakeRex(t, t.TempDir(), `"session inspect `+rexOtherID+` --json") `+tt.inspect+" ;;\n"+rexSessionBranches(
				rexSession{ID: rexTestID, Label: rexTestSession, Windows: 1},
				rexSession{ID: rexOtherID, Label: "yendo/remuda/other", Windows: 1},
			))

			sessions, err := r.List()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, sessions)
		})
	}
}

func TestRexChecksAPIRevision(t *testing.T) {
	t.Parallel()

	const current = "api: revision 7 (supports 0 to 7)"
	tests := []struct {
		name    string
		version string
		wantErr string
	}{
		{name: "current", version: rexTestVersion},
		{name: "newer", version: strings.Replace(rexTestVersion, current, "api: revision 8 (supports 0 to 8)", 1)},
		{name: "older", version: strings.Replace(rexTestVersion, current, "api: revision 6 (supports 0 to 6)", 1), wantErr: "rex api revision 6 is older than 7"},
		{name: "unreported", version: strings.Replace(rexTestVersion, "\n"+current, "", 1), wantErr: "rex version reports no api revision"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			r := newFakeRex(t, dir, "version) printf '%s\\n' "+shellutil.SingleQuote(tt.version)+" ;;\n"+rexSessionBranches())

			_, err := r.List()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Equal(t, []string{"rex --autostart=false version"}, rexCalls(t, dir))
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"rex --autostart=false version", "rex --autostart=false ls --json"}, rexCalls(t, dir))
		})
	}
}

func TestRexStartWithEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.json")
	panePath := filepath.Join(dir, "pane")
	r := newFakeRex(t, dir, rexSessionBranches()+`"api call session.create -") cat > `+shellutil.SingleQuote(specPath)+" ;;")

	require.NoError(t, r.StartWithEnv(
		rexTestSession,
		`printf '%s|%s|%s|%s' "$OPENAI_API_KEY" "$REX_SESSION" "${REX_CALLER-unset}" "$TERM" > `+shellutil.SingleQuote(panePath),
		[]string{"OPENAI_API_KEY=sk-test-secret", "REX_SESSION=session:caller", "REX_CALLER=caller", "HOME=" + dir, "PATH=/usr/bin:/bin"},
	))

	// The secret stays out of argv and the create spec.
	require.Equal(t, []string{
		"rex --autostart=false version",
		"rex --autostart=false ls --json",
		"rex --autostart=false api call session.create -",
	}, rexCalls(t, dir))
	spec, err := os.ReadFile(specPath)
	require.NoError(t, err)
	require.NotContains(t, string(spec), "sk-test-secret")

	var created struct {
		InitialWindows []struct {
			Layout struct {
				Block struct {
					Options struct {
						Command []string `json:"command"`
					} `json:"options"`
				} `json:"block"`
			} `json:"layout"`
		} `json:"initial_windows"`
	}
	require.NoError(t, json.Unmarshal(spec, &created))
	command := created.InitialWindows[0].Layout.Block.Options.Command
	require.Len(t, command, 3)
	encodedScript, err := json.Marshal(command[2])
	require.NoError(t, err)
	require.JSONEq(t, `{
		"label": "yendo/remuda/rm-2l5u",
		"initial_windows": [{
			"window_label": "agent",
			"layout": {"block": {
				"flavor": "com.superlogical.terminal.shell",
				"label": "agent",
				"options": {
					"shell": "none",
					"login": false,
					"command": ["bash", "-lc", "<script>"],
					"size": {"columns": 200, "rows": 50}
				}
			}}
		}]
	}`, strings.Replace(string(spec), string(encodedScript), `"<script>"`, 1))

	envFile := filepath.Join(os.TempDir(), regexp.MustCompile(`remuda-rex-env-\d+`).FindString(command[2]))
	t.Cleanup(func() { _ = os.Remove(envFile) })
	require.FileExists(t, envFile)

	// Run the block's command as Rex would, in a pane with its own Rex
	// variables: the agent sees the caller's secret but the pane's identity.
	//nolint:gosec // G204: runs the command this test captured from the create spec.
	pane := exec.CommandContext(t.Context(), command[0], command[1:]...)
	pane.Env = []string{"HOME=" + dir, "PATH=/usr/bin:/bin", "TERM=xterm-rex", "REX_SESSION=session:pane", "USER=agent", "LOGNAME=agent"}
	output, err := pane.CombinedOutput()
	require.NoError(t, err, string(output))
	got, err := os.ReadFile(panePath)
	require.NoError(t, err)
	require.Equal(t, "sk-test-secret|session:pane|unset|xterm-rex", string(got))
	require.NoFileExists(t, envFile)
}

func TestRexStartChecksExistingSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sessions []rexSession
		wantErr  error
	}{
		{
			name:     "live session",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 1}},
			wantErr:  SessionAlreadyExistsError{Name: rexTestSession},
		},
		{
			name:     "shared label",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 1}, {ID: rexOtherID, Label: rexTestSession, Windows: 1}},
			wantErr:  AmbiguousSessionError{Name: rexTestSession, IDs: []string{rexTestID, rexOtherID}},
		},
		{
			name:     "empty session left by an exited agent",
			sessions: []rexSession{{ID: rexTestID, Label: rexTestSession, Windows: 0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			r := newFakeRex(t, dir, rexSessionBranches(tt.sessions...))

			require.Equal(t, tt.wantErr, r.Start(rexTestSession, "codex"))
			created := strings.Contains(strings.Join(rexCalls(t, dir), "\n"), "api call session.create -")
			require.Equal(t, tt.wantErr == nil, created)
		})
	}
}

func TestRexSessionOperations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newFakeRex(t, dir, rexSessionBranches(
		rexSession{ID: rexLeftoverID, Label: rexTestSession, Windows: 0},
		rexSession{ID: rexTestID, Label: rexTestSession, Windows: 1},
		rexSession{ID: rexOtherID, Label: "yendo/remuda/other", Windows: 0},
	)+`"capture -s `+rexTestID+` -b agent --trim --unwrap") printf 'one\ntwo\nthree\n\n' ;;
"client ls --json") echo '{"clients":[{"client_id":"client:cli","info":{"kind":"cli","name":"rex"}},{"client_id":"client:app","info":{"kind":"app","name":"Rex Beta"}}]}' ;;`)

	buffer, err := r.ReadBuffer(rexTestSession, 2)
	require.NoError(t, err)
	require.Equal(t, "two\nthree", buffer)
	require.NoError(t, r.Send(rexTestSession, "continue", true))
	require.NoError(t, r.Attach(rexTestSession))

	// Every rex call disables autostart, each lookup inspects only the
	// sessions under the name, and the live one is then targeted by ID.
	lookup := []string{
		"rex --autostart=false ls --json",
		"rex --autostart=false session inspect " + rexLeftoverID + " --json",
		"rex --autostart=false session inspect " + rexTestID + " --json",
	}
	require.Equal(t, slices.Concat(
		[]string{"rex --autostart=false version"},
		lookup,
		[]string{"rex --autostart=false capture -s " + rexTestID + " -b agent --trim --unwrap"},
		lookup,
		[]string{
			"rex --autostart=false send -s " + rexTestID + " -b agent continue",
			"rex --autostart=false send-key -s " + rexTestID + " -b agent enter",
		},
		lookup,
		[]string{
			"rex --autostart=false client ls --json",
			"open -a Rex Beta",
			"rex --autostart=false -C client:app do session.select session_id=" + rexTestID,
		},
	), rexCalls(t, dir))
}

func TestRexKill(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sessions   []rexSession
		wantErr    error
		wantKilled []string
	}{
		{
			name: "live session and empty ones under its name",
			sessions: []rexSession{
				{ID: rexLeftoverID, Label: rexTestSession, Windows: 0},
				{ID: rexTestID, Label: rexTestSession, Windows: 1},
				{ID: rexOtherID, Label: "yendo/remuda/other", Windows: 0},
			},
			wantKilled: []string{rexLeftoverID, rexTestID},
		},
		{
			name: "only empty sessions",
			sessions: []rexSession{
				{ID: rexLeftoverID, Label: rexTestSession, Windows: 0},
				{ID: rexTestID, Label: rexTestSession, Windows: 0},
			},
			wantKilled: []string{rexLeftoverID, rexTestID},
		},
		{
			name:     "no session",
			sessions: []rexSession{{ID: rexOtherID, Label: "yendo/remuda/other", Windows: 1}},
			wantErr:  ErrSessionNotFound,
		},
		{
			name: "shared label",
			sessions: []rexSession{
				{ID: rexTestID, Label: rexTestSession, Windows: 1},
				{ID: rexOtherID, Label: rexTestSession, Windows: 1},
				{ID: rexLeftoverID, Label: rexTestSession, Windows: 0},
			},
			wantErr: AmbiguousSessionError{Name: rexTestSession, IDs: []string{rexTestID, rexOtherID}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			r := newFakeRex(t, dir, rexSessionBranches(tt.sessions...))

			require.Equal(t, tt.wantErr, r.Kill(rexTestSession))
			var killed []string
			for _, call := range rexCalls(t, dir) {
				if id, ok := strings.CutPrefix(call, "rex --autostart=false kill "); ok {
					killed = append(killed, id)
				}
			}
			require.Equal(t, tt.wantKilled, killed)
		})
	}
}

func TestRexAttachFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, err error)
	}{
		{
			name:   "no app connected",
			script: `"client ls --json") echo '{"clients":[{"client_id":"client:cli","info":{"kind":"cli","name":"rex"}}]}' ;;`,
			check: func(t *testing.T, err error) {
				require.ErrorIs(t, err, ErrRexAppNotConnected)
			},
		},
		{
			name: "remote control off",
			script: `"client ls --json") echo '{"clients":[{"client_id":"client:app","info":{"kind":"app","name":"Rex Beta"}}]}' ;;
"-C client:app do session.select session_id=` + rexTestID + `") echo 'Error: Rex.app is not accepting actions. Turn on Remote Control in its Settings under Rex Server.' >&2; exit 1 ;;`,
			check: func(t *testing.T, err error) {
				var commandErr RexCommandError
				require.ErrorAs(t, err, &commandErr)
				require.Equal(t, "Error: Rex.app is not accepting actions. Turn on Remote Control in its Settings under Rex Server.", commandErr.Output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newFakeRex(t, t.TempDir(), rexSessionBranches(rexSession{ID: rexTestID, Label: rexTestSession, Windows: 1})+tt.script)
			tt.check(t, r.Attach(rexTestSession))
		})
	}
}

func TestRexSessionCreatedAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want time.Time
	}{
		{name: "UUIDv7", id: rexTestID, want: rexTestCreatedAt},
		{name: "other UUID version", id: "session:01a17ac2-2031-4e48-9b9c-43dd6612bef5"},
		{name: "not a UUID", id: "session:main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, rexSessionCreatedAt(tt.id))
		})
	}

	t.Run("any timestamp round-trips", func(t *testing.T) {
		t.Parallel()

		roundTrips := func(millis uint64, random [10]byte) bool {
			millis &= 1<<48 - 1
			var stamp [8]byte
			binary.BigEndian.PutUint64(stamp[:], millis)
			var raw [16]byte
			copy(raw[:6], stamp[2:])
			copy(raw[6:], random[:])
			raw[6] = 0x70 | raw[6]&0x0f
			id := fmt.Sprintf("session:%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:])
			return rexSessionCreatedAt(id).Equal(time.UnixMilli(int64(millis)))
		}
		require.NoError(t, quick.Check(roundTrips, nil))
	})
}

// newFakeRex returns a rex whose CLI and open(1) are stand-ins in dir, both
// logging each call to dir/calls. The rex stand-in fails any call that could
// start a server, answers version as rexTestVersion, and otherwise runs
// script: branches of a case statement over its remaining arguments.
func newFakeRex(t *testing.T, dir, script string) *rex {
	t.Helper()

	calls := shellutil.SingleQuote(filepath.Join(dir, "calls"))
	writeExecutable(t, filepath.Join(dir, "rex"), `#!/bin/sh
printf 'rex %s\n' "$*" >> `+calls+`
if [ "$1" != --autostart=false ]; then echo 'Error: would start a server' >&2; exit 2; fi
shift
case "$*" in
`+script+`
version) printf '%s\n' `+shellutil.SingleQuote(rexTestVersion)+` ;;
esac
`)
	writeExecutable(t, filepath.Join(dir, "open"), "#!/bin/sh\nprintf 'open %s\\n' \"$*\" >> "+calls+"\n")
	return &rex{logger: zerolog.Nop(), bin: filepath.Join(dir, "rex"), open: filepath.Join(dir, "open")}
}

// writeExecutable writes a stand-in program. Holding ForkLock keeps forks in
// parallel tests from inheriting the file while it is open for writing, which
// would make exec fail with ETXTBSY (go.dev/issue/22315).
func writeExecutable(t *testing.T, path, script string) {
	t.Helper()

	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

// rexSessionBranches answers rex ls --json and session inspect for sessions,
// in the shapes Rex prints them. Rex leaves windows out of an empty session's
// inspect output.
func rexSessionBranches(sessions ...rexSession) string {
	entries := make([]string, len(sessions))
	var inspects strings.Builder
	for i, session := range sessions {
		entries[i] = fmt.Sprintf(`{"session_id":%q,"label":%q}`, session.ID, session.Label)
		windows := ""
		if session.Windows > 0 {
			windows = `,"windows":[` + strings.Join(slices.Repeat([]string{rexTestWindow}, session.Windows), ",") + `]`
		}
		fmt.Fprintf(&inspects, `"session inspect %s --json") echo '{"revision":2,"session_id":%q,"label":%q,"clients":[]%s}' ;;`+"\n", session.ID, session.ID, session.Label, windows)
	}
	return `"ls --json") echo '{"sessions":[` + strings.Join(entries, ",") + `]}' ;;` + "\n" + inspects.String()
}

func rexCalls(t *testing.T, dir string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "calls"))
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
