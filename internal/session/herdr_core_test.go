package session

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

const (
	herdrCoreTestSession   = "yendo/remuda/rm-u1r1"
	herdrCoreTestWorkspace = "/src/remuda"
	herdrCoreTestCreatedMS = 1793320105009
	// herdrCoreTestHello is the handshake sent first on every connection.
	herdrCoreTestHello = `hello {"generation":1}`
	// herdrCoreTestLookup is the request for the terminals labeled
	// herdrCoreTestSession.
	herdrCoreTestLookup = `terminal.list {"all":true,"labels":{"remuda.session":"yendo/remuda/rm-u1r1"}}`
)

var herdrCoreTestInfo = SessionInfo{
	Name:        herdrCoreTestSession,
	CreatedAt:   time.UnixMilli(herdrCoreTestCreatedMS).UTC(),
	Multiplexer: "herdr-core",
}

func TestHerdrCoreSocket(t *testing.T) {
	t.Parallel()

	const configSocket = `socket = "/run/hc/config.sock"`
	tests := []struct {
		name string
		env  map[string]string
		// files are written under the test's directory, which is $DIR in env,
		// want and wantErr; its home subdirectory is the home directory.
		files   map[string]string
		want    string
		wantErr string
	}{
		{
			name:  "HC_SOCKET",
			env:   map[string]string{"HC_SOCKET": "/run/hc/env.sock", "HC_SESSION": "work"},
			files: map[string]string{"home/.config/herdr-core/config.toml": configSocket},
			want:  "/run/hc/env.sock",
		},
		{
			name:  "config socket",
			env:   map[string]string{"HC_SESSION": "work"},
			files: map[string]string{"home/.config/herdr-core/config.toml": configSocket},
			want:  "/run/hc/config.sock",
		},
		{
			name: "config under XDG_CONFIG_HOME",
			env:  map[string]string{"XDG_CONFIG_HOME": "$DIR/config"},
			files: map[string]string{
				"config/herdr-core/config.toml":       configSocket,
				"home/.config/herdr-core/config.toml": `socket = "/run/hc/ignored.sock"`,
			},
			want: "/run/hc/config.sock",
		},
		{
			name:  "config without socket",
			files: map[string]string{"home/.config/herdr-core/config.toml": "wait_timeout = 60\n"},
			want:  "$DIR/home/.local/state/herdr-core/default/default.sock",
		},
		{
			name: "HC_SESSION under XDG_STATE_HOME",
			env:  map[string]string{"HC_SESSION": "work", "XDG_STATE_HOME": "$DIR/state"},
			want: "$DIR/state/herdr-core/work/work.sock",
		},
		{
			name: "defaults",
			want: "$DIR/home/.local/state/herdr-core/default/default.sock",
		},
		{
			name:    "unreadable config",
			files:   map[string]string{"home/.config/herdr-core/config.toml": "socket = "},
			wantErr: "read $DIR/home/.config/herdr-core/config.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, contents := range tt.files {
				path := filepath.Join(dir, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
			}
			values := make(map[string]string, len(tt.env))
			for name, value := range tt.env {
				values[name] = strings.ReplaceAll(value, "$DIR", dir)
			}
			h := &herdrCore{env: env.StaticProvider{Values: values, HomeDir: filepath.Join(dir, "home")}}

			socket, err := h.socket()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, strings.ReplaceAll(tt.wantErr, "$DIR", dir))
				return
			}
			require.NoError(t, err)
			require.Equal(t, strings.ReplaceAll(tt.want, "$DIR", dir), socket)
		})
	}
}

func TestHerdrCoreNotRunning(t *testing.T) {
	t.Parallel()

	sockets := []struct {
		name   string
		socket func(t *testing.T) string
	}{
		{name: "no socket", socket: shortSocketPath},
		{
			name: "stale socket",
			socket: func(t *testing.T) string {
				t.Helper()
				socket := shortSocketPath(t)
				listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
				require.NoError(t, err)
				listener.SetUnlinkOnClose(false)
				require.NoError(t, listener.Close())
				return socket
			},
		},
	}
	operations := []struct {
		name string
		run  func(*herdrCore) error
	}{
		{name: "start", run: func(h *herdrCore) error {
			return h.StartAgent(AgentStart{SessionName: herdrCoreTestSession, Command: "codex"})
		}},
		{name: "list", run: func(h *herdrCore) error { _, err := h.List(); return err }},
		{name: "find", run: func(h *herdrCore) error { _, err := h.Find(herdrCoreTestSession); return err }},
		{name: "attach", run: func(h *herdrCore) error { return h.Attach(herdrCoreTestSession) }},
		{name: "read", run: func(h *herdrCore) error { _, err := h.ReadBuffer(herdrCoreTestSession, 20); return err }},
		{name: "send", run: func(h *herdrCore) error { return h.Send(herdrCoreTestSession, "continue", true) }},
		{name: "kill", run: func(h *herdrCore) error { return h.Kill(herdrCoreTestSession) }},
	}

	for _, tt := range sockets {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			socket := tt.socket(t)
			for _, operation := range operations {
				t.Run(operation.name, func(t *testing.T) {
					t.Parallel()

					// Running hc would start a core, so any run of the fake
					// hc is recorded.
					dir := t.TempDir()
					h := &herdrCore{logger: zerolog.Nop(), env: env.StaticProvider{Values: map[string]string{"HC_SOCKET": socket}}, hc: filepath.Join(dir, "hc")}
					ran := filepath.Join(dir, "ran")
					writeExecutable(t, h.hc, "#!/bin/sh\ntouch "+shellutil.SingleQuote(ran)+"\n")

					require.ErrorIs(t, operation.run(h), ErrHerdrCoreNotRunning)
					require.NoFileExists(t, ran)
				})
			}
		})
	}
}

func TestHerdrCoreChecksGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		generation        int
		ignoresGeneration bool
		wantErr           error
	}{
		{name: "current", generation: 1},
		{name: "refused", generation: 2, wantErr: UnsupportedHerdrCoreGenerationError{Message: "unsupported generation 1"}},
		{name: "answered with another", generation: 2, ignoresGeneration: true, wantErr: UnsupportedHerdrCoreGenerationError{Generation: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core := startFakeHerdrCore(t, &fakeHerdrCore{generation: tt.generation, ignoresGeneration: tt.ignoresGeneration})
			_, err := core.backend().List()
			require.Equal(t, tt.wantErr, err)
			// A core speaking another generation is asked nothing past hello.
			want := []string{herdrCoreTestHello}
			if tt.wantErr == nil {
				want = append(want, `terminal.list {}`)
			}
			require.Equal(t, want, core.calls())
		})
	}
}

func TestHerdrCoreFind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		terminals []fakeHerdrCoreTerminal
		find      string
		want      SessionInfo
		wantErr   error
	}{
		{
			name:      "exact label",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", herdrCoreTestSession), liveHerdrCoreTerminal("t2", herdrCoreTestSession+"-2")},
			find:      herdrCoreTestSession,
			want:      herdrCoreTestInfo,
		},
		{
			name:      "shared label",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", herdrCoreTestSession), liveHerdrCoreTerminal("t2", herdrCoreTestSession)},
			find:      herdrCoreTestSession,
			wantErr:   AmbiguousSessionError{Name: herdrCoreTestSession, IDs: []string{"t1", "t2"}},
		},
		{
			name:      "finished terminal",
			terminals: []fakeHerdrCoreTerminal{finishedHerdrCoreTerminal("t1", herdrCoreTestSession)},
			find:      herdrCoreTestSession,
			wantErr:   ErrSessionNotFound,
		},
		{
			name:      "live terminal beside a finished one",
			terminals: []fakeHerdrCoreTerminal{finishedHerdrCoreTerminal("t1", herdrCoreTestSession), liveHerdrCoreTerminal("t2", herdrCoreTestSession)},
			find:      herdrCoreTestSession,
			want:      herdrCoreTestInfo,
		},
		{
			name:      "label outside the Remuda naming scheme",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", "Research")},
			find:      "Research",
			wantErr:   ErrSessionNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core := newFakeHerdrCore(t, tt.terminals...)
			info, err := core.backend().Find(tt.find)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, info)
		})
	}

	t.Run("filters by label in the core", func(t *testing.T) {
		t.Parallel()

		core := newFakeHerdrCore(t, liveHerdrCoreTerminal("t1", herdrCoreTestSession))
		_, err := core.backend().Find(herdrCoreTestSession)
		require.NoError(t, err)
		require.Equal(t, []string{herdrCoreTestHello, herdrCoreTestLookup}, core.calls())
	})
}

func TestHerdrCoreList(t *testing.T) {
	t.Parallel()

	unlabeled := liveHerdrCoreTerminal("t3", "")
	unlabeled.Labels = map[string]string{"herdr.space": "s1"}
	core := newFakeHerdrCore(t,
		liveHerdrCoreTerminal("t1", herdrCoreTestSession),
		finishedHerdrCoreTerminal("t2", "yendo/remuda/exited"),
		unlabeled,
		liveHerdrCoreTerminal("t4", "Research"),
		liveHerdrCoreTerminal("t5", "yendo/remuda/other"),
	)

	sessions, err := core.backend().List()
	require.NoError(t, err)
	require.Equal(t, []SessionInfo{
		herdrCoreTestInfo,
		{Name: "yendo/remuda/other", CreatedAt: herdrCoreTestInfo.CreatedAt, Multiplexer: "herdr-core"},
	}, sessions)
	require.Equal(t, []string{herdrCoreTestHello, `terminal.list {}`}, core.calls())
}

func TestHerdrCoreStartChecksExistingSessions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		terminals []fakeHerdrCoreTerminal
		wantErr   error
	}{
		{
			name:      "live terminal",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", herdrCoreTestSession)},
			wantErr:   SessionAlreadyExistsError{Name: herdrCoreTestSession},
		},
		{
			name:      "shared label",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", herdrCoreTestSession), liveHerdrCoreTerminal("t2", herdrCoreTestSession)},
			wantErr:   AmbiguousSessionError{Name: herdrCoreTestSession, IDs: []string{"t1", "t2"}},
		},
		{
			name:      "finished terminal",
			terminals: []fakeHerdrCoreTerminal{finishedHerdrCoreTerminal("t1", herdrCoreTestSession)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core := newFakeHerdrCore(t, tt.terminals...)
			require.Equal(t, tt.wantErr, core.backend().StartAgent(AgentStart{SessionName: herdrCoreTestSession, Workspace: herdrCoreTestWorkspace, Command: "codex"}))
			if tt.wantErr != nil {
				require.Empty(t, core.params("terminal.run"))
				return
			}
			_, argv := expectRun(t, core)
			require.Equal(t, "codex", argv[2])
		})
	}
}

func TestHerdrCoreStartAgent(t *testing.T) {
	t.Parallel()

	core := newFakeHerdrCore(t)
	dir := t.TempDir()
	panePath := filepath.Join(dir, "pane")
	require.NoError(t, core.backend().StartAgent(AgentStart{
		SessionName: herdrCoreTestSession,
		Workspace:   herdrCoreTestWorkspace,
		Command:     `printf '%s|%s|%s|%s' "$OPENAI_API_KEY" "$HC_TERMINAL_ID" "${HC_CALLER-unset}" "$TERM" > ` + shellutil.SingleQuote(panePath),
		Env:         []string{"OPENAI_API_KEY=sk-test-secret", "HC_TERMINAL_ID=t9", "HC_CALLER=caller", "HOME=" + dir, "PATH=/usr/bin:/bin"},
	}))

	// Run params are saved to disk, so the secret must stay out of them.
	params, argv := expectRun(t, core)
	require.NotContains(t, params, "sk-test-secret")

	envFile := filepath.Join(os.TempDir(), regexp.MustCompile(`remuda-herdr-core-env-\d+`).FindString(argv[2]))
	t.Cleanup(func() { _ = os.Remove(envFile) })
	require.FileExists(t, envFile)

	// Run the argv as herdr-core would, in a terminal with its own HC_
	// variables: the agent sees the caller's secret but the terminal's
	// identity.
	//nolint:gosec // G204: runs the argv this test captured from terminal.run.
	terminal := exec.CommandContext(t.Context(), argv[0], argv[1:]...)
	terminal.Env = []string{"HOME=" + dir, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "HC_TERMINAL_ID=t1", "USER=agent", "LOGNAME=agent"}
	output, err := terminal.CombinedOutput()
	require.NoError(t, err, string(output))
	got, err := os.ReadFile(panePath)
	require.NoError(t, err)
	require.Equal(t, "sk-test-secret|t1|unset|xterm-256color", string(got))
	require.NoFileExists(t, envFile)
}

func TestHerdrCoreKill(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		terminals []fakeHerdrCoreTerminal
		wantErr   error
		// wantCalls follow the handshake and the lookup.
		wantCalls []string
		wantLeft  []string
	}{
		{
			name: "live terminal and finished ones under its name",
			terminals: []fakeHerdrCoreTerminal{
				finishedHerdrCoreTerminal("t1", herdrCoreTestSession),
				liveHerdrCoreTerminal("t2", herdrCoreTestSession),
				finishedHerdrCoreTerminal("t3", "yendo/remuda/other"),
			},
			wantCalls: []string{`terminal.rm {"terminal":"t1"}`, `terminal.kill {"terminal":"t2"}`, `terminal.rm {"terminal":"t2"}`},
			wantLeft:  []string{"t3"},
		},
		{
			name: "only finished terminals",
			terminals: []fakeHerdrCoreTerminal{
				finishedHerdrCoreTerminal("t1", herdrCoreTestSession),
				finishedHerdrCoreTerminal("t2", herdrCoreTestSession),
			},
			wantCalls: []string{`terminal.rm {"terminal":"t1"}`, `terminal.rm {"terminal":"t2"}`},
		},
		{
			name:      "no terminal",
			terminals: []fakeHerdrCoreTerminal{liveHerdrCoreTerminal("t1", "yendo/remuda/other")},
			wantErr:   ErrSessionNotFound,
			wantLeft:  []string{"t1"},
		},
		{
			name: "shared label",
			terminals: []fakeHerdrCoreTerminal{
				liveHerdrCoreTerminal("t1", herdrCoreTestSession),
				liveHerdrCoreTerminal("t2", herdrCoreTestSession),
				finishedHerdrCoreTerminal("t3", herdrCoreTestSession),
			},
			wantErr:  AmbiguousSessionError{Name: herdrCoreTestSession, IDs: []string{"t1", "t2"}},
			wantLeft: []string{"t1", "t2", "t3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core := newFakeHerdrCore(t, tt.terminals...)
			require.Equal(t, tt.wantErr, core.backend().Kill(herdrCoreTestSession))
			require.Equal(t, append([]string{herdrCoreTestHello, herdrCoreTestLookup}, tt.wantCalls...), core.calls())
			require.Equal(t, tt.wantLeft, core.addresses())
		})
	}
}

func TestHerdrCoreReadBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines int
		// history is how many lines of history terminal.read is asked for.
		history int
		want    string
	}{
		{name: "last lines", lines: 2, history: 2, want: "two\nthree"},
		{name: "full scrollback", lines: 0, history: herdrCoreFullHistory, want: "one\ntwo\nthree\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			terminal := liveHerdrCoreTerminal("t1", herdrCoreTestSession)
			terminal.Text = "one\ntwo\nthree\n\n"
			core := newFakeHerdrCore(t, terminal)

			buffer, err := core.backend().ReadBuffer(herdrCoreTestSession, tt.lines)
			require.NoError(t, err)
			require.Equal(t, tt.want, buffer)
			require.Equal(t, []string{`{"lines":` + strconv.Itoa(tt.history) + `,"terminal":"t1"}`}, core.params("terminal.read"))
		})
	}
}

func TestHerdrCoreSend(t *testing.T) {
	t.Parallel()

	const blocked = "t1 is blocked (permission): Claude Code is asking to run a command"
	tests := []struct {
		name          string
		blocked       string
		appendNewline bool
		wantErr       error
		wantSends     []string
	}{
		{
			name:          "submits",
			appendNewline: true,
			wantSends:     []string{`{"terminal":"t1","text":"continue"}`},
		},
		{
			name:          "blocked program",
			blocked:       blocked,
			appendNewline: true,
			wantErr:       BlockedSessionError{Name: herdrCoreTestSession, Message: blocked},
			wantSends:     []string{`{"terminal":"t1","text":"continue"}`},
		},
		{
			name:    "without a newline",
			wantErr: ErrHerdrCoreSendNeedsNewline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			terminal := liveHerdrCoreTerminal("t1", herdrCoreTestSession)
			terminal.Blocked = tt.blocked
			core := newFakeHerdrCore(t, terminal)

			require.Equal(t, tt.wantErr, core.backend().Send(herdrCoreTestSession, "continue", tt.appendNewline))
			require.Equal(t, tt.wantSends, core.params("terminal.send"))
		})
	}
}

func TestHerdrCoreAttach(t *testing.T) {
	t.Parallel()

	core := newFakeHerdrCore(t, liveHerdrCoreTerminal("t1", herdrCoreTestSession))
	dir := t.TempDir()
	h := core.backend()
	h.hc = filepath.Join(dir, "hc")
	attached := filepath.Join(dir, "attached")
	writeExecutable(t, h.hc, "#!/bin/sh\nprintf '%s %s' \"$HC_SOCKET\" \"$*\" > "+shellutil.SingleQuote(attached)+"\n")

	require.NoError(t, h.Attach(herdrCoreTestSession))
	got, err := os.ReadFile(attached)
	require.NoError(t, err)
	require.Equal(t, core.socket+" attach t1", string(got))
}

// herdrCoreTestRunParams is what terminal.run is sent for every session, with
// bash's path and the script as placeholders.
const herdrCoreTestRunParams = `{
	"argv": ["<bash>", "-lc", "<script>"],
	"cwd": "/src/remuda",
	"labels": {"remuda.session": "yendo/remuda/rm-u1r1"},
	"no_parent": true,
	"no_notify": true
}`

// expectRun checks that core was sent one terminal.run, with the params of
// herdrCoreTestRunParams, and returns its params and argv.
func expectRun(t *testing.T, core *fakeHerdrCore) (string, []string) {
	t.Helper()

	runs := core.params("terminal.run")
	require.Len(t, runs, 1)
	var run struct {
		Argv []string `json:"argv"`
	}
	require.NoError(t, json.Unmarshal([]byte(runs[0]), &run))
	require.Len(t, run.Argv, 3)
	require.True(t, filepath.IsAbs(run.Argv[0]), run.Argv[0])
	require.Equal(t, "bash", filepath.Base(run.Argv[0]))
	params := runs[0]
	for placeholder, value := range map[string]string{"<bash>": run.Argv[0], "<script>": run.Argv[2]} {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		params = strings.Replace(params, string(encoded), `"`+placeholder+`"`, 1)
	}
	require.JSONEq(t, herdrCoreTestRunParams, params)
	return runs[0], run.Argv
}

// fakeHerdrCore stands in for a core: a JSON-RPC server on a Unix socket
// that keeps terminals in memory and logs each request.
type fakeHerdrCore struct {
	socket string
	// generation is the protocol generation the core speaks. A hello for
	// another is refused, unless ignoresGeneration is set, in which case it's
	// answered with generation.
	generation        int
	ignoresGeneration bool

	mu        sync.Mutex
	terminals []fakeHerdrCoreTerminal
	requests  []fakeHerdrCoreRequest
}

// fakeHerdrCoreTerminal is a terminal as the fake core reports it.
type fakeHerdrCoreTerminal struct {
	Address   string            `json:"address"`
	Labels    map[string]string `json:"labels"`
	State     string            `json:"state"`
	CreatedMS int64             `json:"created_ms"`
	// Text is what terminal.read returns.
	Text string `json:"-"`
	// Blocked, when set, is the message of terminal.send's blocked error.
	Blocked string `json:"-"`
}

type fakeHerdrCoreRequest struct {
	method string
	params string
}

type fakeHerdrCoreError struct {
	kind    string
	message string
}

// newFakeHerdrCore starts a fake core holding terminals.
func newFakeHerdrCore(t *testing.T, terminals ...fakeHerdrCoreTerminal) *fakeHerdrCore {
	t.Helper()

	return startFakeHerdrCore(t, &fakeHerdrCore{generation: herdrCoreGeneration, terminals: terminals})
}

func startFakeHerdrCore(t *testing.T, core *fakeHerdrCore) *fakeHerdrCore {
	t.Helper()

	core.socket = shortSocketPath(t)
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "unix", core.socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go core.answer(conn)
		}
	}()
	return core
}

// shortSocketPath returns a socket path in a new directory under the system
// temp dir, since macOS caps socket paths at 103 bytes and test directories
// can be longer.
func shortSocketPath(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "hc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

// backend returns a herdr-core backend that dials c.
func (c *fakeHerdrCore) backend() *herdrCore {
	return &herdrCore{logger: zerolog.Nop(), env: env.StaticProvider{Values: map[string]string{"HC_SOCKET": c.socket}}}
}

// answer replies to the requests on conn, which like a real core's are one
// JSON-RPC message per line.
func (c *fakeHerdrCore) answer(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	lines := bufio.NewReader(conn)
	replies := json.NewEncoder(conn)
	for {
		line, err := lines.ReadBytes('\n')
		if err != nil {
			return
		}
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		reply := map[string]any{"jsonrpc": "2.0"}
		if err := json.Unmarshal(line, &request); err != nil || request.JSONRPC != "2.0" {
			reply["error"] = map[string]any{"code": -32600, "message": "invalid request", "data": map[string]string{"kind": "invalid_request"}}
		} else if result, failure := c.handle(request.Method, request.Params); failure != nil {
			reply["id"] = request.ID
			reply["error"] = map[string]any{"code": -32000, "message": failure.message, "data": map[string]string{"kind": failure.kind}}
		} else {
			reply["id"] = request.ID
			reply["result"] = result
		}
		if err := replies.Encode(reply); err != nil {
			return
		}
	}
}

func (c *fakeHerdrCore) handle(method string, raw json.RawMessage) (any, *fakeHerdrCoreError) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requests = append(c.requests, fakeHerdrCoreRequest{method: method, params: string(raw)})
	var params struct {
		Generation int               `json:"generation"`
		All        bool              `json:"all"`
		Labels     map[string]string `json:"labels"`
		Terminal   string            `json:"terminal"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, &fakeHerdrCoreError{kind: "invalid_params", message: err.Error()}
	}
	switch method {
	case "hello":
		if params.Generation != c.generation && !c.ignoresGeneration {
			return nil, &fakeHerdrCoreError{kind: "unsupported_generation", message: "unsupported generation " + strconv.Itoa(params.Generation)}
		}
		return map[string]any{"generation": c.generation, "version": "0.1.13"}, nil
	case "terminal.list":
		terminals := []fakeHerdrCoreTerminal{}
		for _, terminal := range c.terminals {
			if (params.All || terminal.State == "live") && carriesLabels(terminal, params.Labels) {
				terminals = append(terminals, terminal)
			}
		}
		return map[string]any{"terminals": terminals, "hidden_finished": 0, "machines": []any{}}, nil
	case "terminal.run":
		terminal := liveHerdrCoreTerminal("t"+strconv.Itoa(len(c.terminals)+1), "")
		terminal.Labels = params.Labels
		c.terminals = append(c.terminals, terminal)
		return terminal, nil
	}

	i := slices.IndexFunc(c.terminals, func(terminal fakeHerdrCoreTerminal) bool {
		return terminal.Address == params.Terminal
	})
	if i < 0 {
		return nil, &fakeHerdrCoreError{kind: "not_found", message: "no terminal " + params.Terminal}
	}
	terminal := &c.terminals[i]
	switch method {
	case "terminal.read":
		return map[string]any{"text": terminal.Text, "complete": true, "how": "history"}, nil
	case "terminal.send":
		if terminal.Blocked != "" {
			return nil, &fakeHerdrCoreError{kind: "blocked", message: terminal.Blocked}
		}
		return map[string]any{"turn": 1}, nil
	case "terminal.kill":
		terminal.State = "finished"
		return *terminal, nil
	case "terminal.rm":
		if terminal.State == "live" {
			return nil, &fakeHerdrCoreError{kind: "live", message: terminal.Address + " is live"}
		}
		c.terminals = slices.Delete(c.terminals, i, i+1)
		return map[string]any{"removed": params.Terminal}, nil
	default:
		return nil, &fakeHerdrCoreError{kind: "method_not_found", message: "no method " + method}
	}
}

// calls returns each request the core was sent, as its method and params.
func (c *fakeHerdrCore) calls() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	calls := make([]string, len(c.requests))
	for i, request := range c.requests {
		calls[i] = request.method + " " + request.params
	}
	return calls
}

// params returns the params of each request for method.
func (c *fakeHerdrCore) params(method string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var params []string
	for _, request := range c.requests {
		if request.method == method {
			params = append(params, request.params)
		}
	}
	return params
}

// addresses returns the addresses of the terminals the core holds.
func (c *fakeHerdrCore) addresses() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var addresses []string
	for _, terminal := range c.terminals {
		addresses = append(addresses, terminal.Address)
	}
	return addresses
}

func carriesLabels(terminal fakeHerdrCoreTerminal, labels map[string]string) bool {
	for key, value := range labels {
		if terminal.Labels[key] != value {
			return false
		}
	}
	return true
}

func liveHerdrCoreTerminal(address, session string) fakeHerdrCoreTerminal {
	return fakeHerdrCoreTerminal{
		Address:   address,
		Labels:    map[string]string{herdrCoreSessionLabel: session},
		State:     "live",
		CreatedMS: herdrCoreTestCreatedMS,
	}
}

func finishedHerdrCoreTerminal(address, session string) fakeHerdrCoreTerminal {
	terminal := liveHerdrCoreTerminal(address, session)
	terminal.State = "finished"
	return terminal
}
