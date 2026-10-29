package session

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/util"
)

const (
	// rexAgentLabel labels the window and terminal block that run the agent,
	// so splits a user adds later never change which block is primary.
	rexAgentLabel = "agent"
	// rexMinAPIRevision is the oldest server API this backend was verified
	// against.
	rexMinAPIRevision = 7
	// rexServerBinary is where the Mac app installs Rex Server's CLI,
	// relative to the home directory.
	rexServerBinary = "Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"
)

var rexPaneEnv = paneEnv{multiplexer: string(MultiplexerRex), prefix: "REX_"}

var rexAPIRevisionPattern = regexp.MustCompile(`(?m)^api:\s*revision\s+(\d+)`)

// ErrRexNotFound means no rex CLI is installed where Remuda looks for one.
var ErrRexNotFound = pkgerrors.New("rex CLI not found; install Rex or set REX_BIN_DIR")

// ErrRexAppNotConnected means no Rex app is connected to show a session in.
var ErrRexAppNotConnected = pkgerrors.New("no Rex app is connected to the Rex server; open Rex and retry")

// errRexSessionGone means a session that rex ls listed no longer exists.
var errRexSessionGone = pkgerrors.New("rex session is gone")

// RexCommandError is a failed rex invocation, carrying Rex's own error text.
type RexCommandError struct {
	Output string
	Err    error
}

func (e RexCommandError) Error() string {
	return fmt.Sprintf("rex: %s: %v", e.Output, e.Err)
}

func (e RexCommandError) Unwrap() error {
	return e.Err
}

// AmbiguousSessionError reports a name that labels more than one live session.
type AmbiguousSessionError struct {
	Name string
	IDs  []string
}

func (e AmbiguousSessionError) Error() string {
	return fmt.Sprintf("session %q is ambiguous: %d sessions have that label (%s)", e.Name, len(e.IDs), strings.Join(e.IDs, ", "))
}

// rex drives Rex, Superlogical's multiplexer, through its CLI. Rex labels are
// neither unique nor case-sensitive, so sessions are matched by exact label
// here and addressed by session ID afterwards.
type rex struct {
	logger zerolog.Logger
	// bin is the rex CLI, resolved on first use.
	bin string
	// open is macOS open(1), which brings the Rex app forward on attach.
	open     string
	verified bool
}

type rexSession struct {
	ID    string `json:"session_id"`
	Label string `json:"label"`
	// Windows comes from session inspect, since rex ls doesn't report it.
	Windows int `json:"-"`
}

type rexClient struct {
	ID   string `json:"client_id"`
	Info struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"info"`
}

func NewRexWithLogger(logger zerolog.Logger) Multiplexer {
	return &rex{logger: logger, open: "open"}
}

func (r *rex) SetLogger(logger zerolog.Logger) {
	r.logger = logger
}

func (r *rex) Name() string {
	return string(MultiplexerRex)
}

func (r *rex) Start(sessionName, command string) error {
	return r.StartWithEnv(sessionName, command, nil)
}

func (r *rex) StartWithEnv(sessionName, command string, env []string) error {
	if _, err := r.Find(sessionName); err == nil {
		return SessionAlreadyExistsError{Name: sessionName}
	} else if !errors.Is(err, ErrSessionNotFound) {
		return err
	}

	envFile, err := rexPaneEnv.writeEnvFile(env)
	if err != nil {
		return err
	}
	if envFile != "" {
		command = rexPaneEnv.commandWithEnvFile(envFile, command)
	}
	if err := r.create(sessionName, command); err != nil {
		if envFile != "" {
			_ = os.Remove(envFile)
		}
		return err
	}
	return nil
}

// create starts a session holding one window with one terminal block.
func (r *rex) create(label, command string) error {
	spec, err := json.Marshal(map[string]any{
		"label": label,
		"initial_windows": []any{map[string]any{
			"window_label": rexAgentLabel,
			"layout": map[string]any{"block": map[string]any{
				"flavor": "com.superlogical.terminal.shell",
				"label":  rexAgentLabel,
				"options": map[string]any{
					// Without shell "none" Rex runs the command through the
					// user's $SHELL, which may be fish, and without login false
					// it runs under /usr/bin/login.
					"shell":   "none",
					"login":   false,
					"command": []string{"bash", "-lc", command},
					// A block keeps its creation size until a client takes it
					// over, and agents wrap long paths at Rex's 80x24 default.
					"size": map[string]int{"columns": 200, "rows": 50},
				},
			}},
		}},
	})
	if err != nil {
		return pkgerrors.Wrap(err, "encode rex session spec")
	}
	if _, err := r.runWithInput(bytes.NewReader(spec), "api", "call", "session.create", "-"); err != nil {
		return pkgerrors.Wrapf(err, "rex session.create %s", label)
	}
	return nil
}

func (r *rex) List() ([]SessionInfo, error) {
	sessions, err := r.sessions(remudaLabel)
	if err != nil {
		return nil, err
	}
	infos := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		if session.live() {
			infos = append(infos, r.sessionInfo(session))
		}
	}
	return infos, nil
}

func (r *rex) Find(name string) (SessionInfo, error) {
	session, err := r.lookup(name)
	if err != nil {
		return SessionInfo{}, err
	}
	return r.sessionInfo(session), nil
}

// Attach selects the session in the Rex app, the only client that can show
// one.
func (r *rex) Attach(name string) error {
	session, err := r.lookup(name)
	if err != nil {
		return err
	}
	app, err := r.appClient()
	if err != nil {
		return err
	}
	if err := util.RunCmdWithLogger(r.logger, r.open, "-a", app.Info.Name); err != nil {
		return err
	}
	// The app refuses actions until Remote Control is turned on in its
	// settings, and Rex's error says so.
	if _, err := r.run("-C", app.ID, "do", "session.select", "session_id="+session.ID); err != nil {
		return pkgerrors.Wrapf(err, "select rex session %s in %s", session.ID, app.Info.Name)
	}
	return nil
}

func (r *rex) ReadBuffer(name string, lines int) (string, error) {
	session, err := r.lookup(name)
	if err != nil {
		return "", err
	}
	output, err := r.run("capture", "-s", session.ID, "-b", rexAgentLabel, "--trim", "--unwrap")
	if err != nil {
		return "", err
	}
	return lastLines(output, lines), nil
}

func (r *rex) Send(name, payload string, appendNewline bool) error {
	session, err := r.lookup(name)
	if err != nil {
		return err
	}
	if payload != "" {
		if _, err := r.run("send", "-s", session.ID, "-b", rexAgentLabel, payload); err != nil {
			return err
		}
	}
	if appendNewline && !strings.HasSuffix(payload, "\n") && !strings.HasSuffix(payload, "\r") {
		// Codex has a paste burst detector in its TUI; a short delay helps it
		// treat the follow-up Enter as a submit instead of more pasted text.
		time.Sleep(200 * time.Millisecond)
		_, err := r.run("send-key", "-s", session.ID, "-b", rexAgentLabel, "enter")
		return err
	}
	return nil
}

// Kill removes the live session labeled name and the empty ones left under
// that label by agents that exited.
func (r *rex) Kill(name string) error {
	sessions, err := r.labeled(name)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return ErrSessionNotFound
	}
	if _, err := liveSession(sessions, name); errors.As(err, new(AmbiguousSessionError)) {
		return err
	}
	for _, session := range sessions {
		if _, err := r.run("kill", session.ID); err != nil {
			return err
		}
	}
	return nil
}

// live reports whether a session still has windows. An agent that exits
// leaves its session behind with none until Kill removes it.
func (s rexSession) live() bool {
	return s.Windows > 0
}

func (s rexSession) id() string {
	return s.ID
}

func remudaLabel(label string) bool {
	return SessionInfo{Name: label}.IsRemudaSession()
}

func (r *rex) lookup(name string) (rexSession, error) {
	sessions, err := r.labeled(name)
	if err != nil {
		return rexSession{}, err
	}
	return liveSession(sessions, name)
}

// labeled returns the Remuda sessions labeled exactly name.
func (r *rex) labeled(name string) ([]rexSession, error) {
	return r.sessions(func(label string) bool {
		return label == name && remudaLabel(label)
	})
}

// labeledSession is a session that a backend matches by label, so more than
// one can carry a name.
type labeledSession interface {
	live() bool
	id() string
}

// liveSession returns the one live session among sessions labeled name.
func liveSession[S labeledSession](sessions []S, name string) (S, error) {
	var match S
	var ids []string
	for _, session := range sessions {
		if session.live() {
			match = session
			ids = append(ids, session.id())
		}
	}
	switch len(ids) {
	case 0:
		return *new(S), ErrSessionNotFound
	case 1:
		return match, nil
	default:
		return *new(S), AmbiguousSessionError{Name: name, IDs: ids}
	}
}

// sessions lists the sessions whose labels match, with their window counts.
// Each match costs a session inspect call.
func (r *rex) sessions(match func(label string) bool) ([]rexSession, error) {
	output, err := r.run("ls", "--json")
	if err != nil {
		return nil, err
	}
	var listed struct {
		Sessions []rexSession `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(output), &listed); err != nil {
		return nil, pkgerrors.Wrap(err, "decode rex ls response")
	}
	var sessions []rexSession
	for _, session := range listed.Sessions {
		if !match(session.Label) {
			continue
		}
		session.Windows, err = r.windows(session.ID)
		if errors.Is(err, errRexSessionGone) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (r *rex) windows(id string) (int, error) {
	output, err := r.run("session", "inspect", id, "--json")
	// Rex exits 3 when a target is missing or ambiguous, and a session ID
	// can't be ambiguous.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 3 {
		return 0, errRexSessionGone
	}
	if err != nil {
		return 0, err
	}
	// Rex leaves windows out for a session that has none.
	var inspected struct {
		Windows []json.RawMessage `json:"windows"`
	}
	if err := json.Unmarshal([]byte(output), &inspected); err != nil {
		return 0, pkgerrors.Wrapf(err, "decode rex session inspect response for %s", id)
	}
	return len(inspected.Windows), nil
}

func (r *rex) sessionInfo(session rexSession) SessionInfo {
	createdAt := rexSessionCreatedAt(session.ID)
	if createdAt.IsZero() {
		r.logger.Debug().Str("session", session.ID).Msg("rex session ID is not a UUIDv7; created-at unknown")
	}
	return SessionInfo{Name: session.Label, CreatedAt: createdAt, Multiplexer: string(MultiplexerRex)}
}

func (r *rex) appClient() (rexClient, error) {
	output, err := r.run("client", "ls", "--json")
	if err != nil {
		return rexClient{}, err
	}
	var listed struct {
		Clients []rexClient `json:"clients"`
	}
	if err := json.Unmarshal([]byte(output), &listed); err != nil {
		return rexClient{}, pkgerrors.Wrap(err, "decode rex client ls response")
	}
	var apps []rexClient
	for _, client := range listed.Clients {
		if client.Info.Kind == "app" {
			apps = append(apps, client)
		}
	}
	switch len(apps) {
	case 0:
		return rexClient{}, ErrRexAppNotConnected
	case 1:
		return apps[0], nil
	default:
		return rexClient{}, pkgerrors.Errorf("%d Rex apps are connected to the Rex server; quit all but one and retry", len(apps))
	}
}

func (r *rex) run(args ...string) (string, error) {
	return r.runWithInput(nil, args...)
}

func (r *rex) runWithInput(input io.Reader, args ...string) (string, error) {
	if err := r.verify(); err != nil {
		return "", err
	}
	return r.output(input, args...)
}

// verify resolves the rex CLI and checks its API revision on first use.
func (r *rex) verify() error {
	if r.verified {
		return nil
	}
	if r.bin == "" {
		bin, err := findRexBinary()
		if err != nil {
			return err
		}
		r.bin = bin
	}
	version, err := r.output(nil, "version")
	if err != nil {
		return err
	}
	if err := checkRexAPIRevision(version); err != nil {
		return err
	}
	r.verified = true
	return nil
}

// output runs the rex CLI with autostart off, so Remuda never launches Rex
// Server, not even to list sessions for the aggregate multiplexer.
func (r *rex) output(input io.Reader, args ...string) (string, error) {
	// Log only the command family: send argv carries prompt text.
	r.logger.Debug().Str("cmd", "rex "+strings.Join(args[:min(2, len(args))], " ")).Msg("command")
	//nolint:gosec,noctx // G204: args are passed as argv to the resolved rex CLI.
	cmd := exec.Command(r.bin, append([]string{"--autostart=false"}, args...)...)
	cmd.Stdin = input
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if message := strings.TrimSpace(string(exitErr.Stderr)); message != "" {
				return "", RexCommandError{Output: message, Err: err}
			}
		}
		return "", pkgerrors.Wrap(err, "rex")
	}
	return string(output), nil
}

// findRexBinary locates the rex CLI. Rex puts it on PATH only inside its own
// terminals, so fall back to where the Mac app installs Rex Server.
func findRexBinary() (string, error) {
	var candidates []string
	if dir := os.Getenv("REX_BIN_DIR"); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "rex"))
	}
	candidates = append(candidates, "rex")
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, rexServerBinary))
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrRexNotFound
}

func checkRexAPIRevision(version string) error {
	match := rexAPIRevisionPattern.FindStringSubmatch(version)
	if match == nil {
		return pkgerrors.Errorf("rex version reports no api revision: %q", strings.TrimSpace(version))
	}
	revision, err := strconv.Atoi(match[1])
	if err != nil {
		return pkgerrors.Wrapf(err, "parse rex api revision %q", match[1])
	}
	if revision < rexMinAPIRevision {
		return pkgerrors.Errorf("rex api revision %d is older than %d, the oldest Remuda supports; update Rex", revision, rexMinAPIRevision)
	}
	return nil
}

// rexSessionCreatedAt reads the creation time from a session ID such as
// "session:01a10cf3-2655-7e48-9b9c-43dd6612bef5", a UUIDv7 whose first 48
// bits are Unix milliseconds. Any other ID yields the zero time.
func rexSessionCreatedAt(id string) time.Time {
	raw, err := hex.DecodeString(strings.ReplaceAll(strings.TrimPrefix(id, "session:"), "-", ""))
	if err != nil || len(raw) != 16 || raw[6]>>4 != 7 {
		return time.Time{}
	}
	var millis int64
	for _, b := range raw[:6] {
		millis = millis<<8 | int64(b)
	}
	return time.UnixMilli(millis).UTC()
}
