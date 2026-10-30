package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
)

const (
	// herdrCoreGeneration is the protocol generation this backend speaks.
	herdrCoreGeneration = 1
	// herdrCoreSessionLabel is the key of the label holding a terminal's
	// session name.
	herdrCoreSessionLabel = "remuda.session"
	// herdrCoreSpaceLabel is the key of the label holding the ID of the
	// Herdr desktop app's space that a terminal appears in. A space is
	// created for an ID that names none.
	herdrCoreSpaceLabel = "herdr.space"
	// herdrCoreFullHistory is how many lines of history terminal.read is asked
	// for when the full scrollback is wanted, since it can't be asked for all
	// of it. It's assumed to be more than a core keeps.
	herdrCoreFullHistory = 1_000_000
)

var herdrCorePaneEnv = paneEnv{multiplexer: string(MultiplexerHerdrCore), prefix: "HC_"}

// ErrHerdrCoreNotRunning means no core is listening on herdr-core's socket.
// Remuda never starts one.
var ErrHerdrCoreNotRunning = pkgerrors.New("herdr-core is not running")

// ErrHCNotFound means no hc CLI is installed where Remuda looks for one.
var ErrHCNotFound = pkgerrors.New("hc CLI not found; install herdr-core")

// ErrHerdrCoreSendNeedsNewline means text was to be sent without a newline,
// which herdr-core can't do: Enter is pressed after every terminal.send.
var ErrHerdrCoreSendNeedsNewline = pkgerrors.New("herdr-core can't send text without a newline")

// UnsupportedHerdrCoreGenerationError means the core doesn't speak protocol
// generation herdrCoreGeneration. Message is the core's reason when hello is
// refused; otherwise Generation is the one in hello's reply.
type UnsupportedHerdrCoreGenerationError struct {
	Generation int
	Message    string
}

func (e UnsupportedHerdrCoreGenerationError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("herdr-core doesn't speak protocol generation %d: %s", herdrCoreGeneration, e.Message)
	}
	return fmt.Sprintf("herdr-core speaks protocol generation %d; Remuda supports only %d", e.Generation, herdrCoreGeneration)
}

// HerdrCoreError is an error reply from herdr-core. Kind is herdr-core's
// stable name for the error, such as not_found.
type HerdrCoreError struct {
	Method  string
	Kind    string
	Message string
}

func (e HerdrCoreError) Error() string {
	return fmt.Sprintf("herdr-core %s: %s (%s)", e.Method, e.Message, e.Kind)
}

// BlockedSessionError is a send refused because the session's program is
// blocked, such as on a permission prompt.
type BlockedSessionError struct {
	Name    string
	Message string
}

func (e BlockedSessionError) Error() string {
	return fmt.Sprintf("session %q is blocked: %s", e.Name, e.Message)
}

// herdrCore drives herdr-core, Herdr's terminal runtime, over its JSON-RPC
// socket. Terminals are matched to sessions by an exact label and addressed
// by their address afterwards.
type herdrCore struct {
	logger zerolog.Logger
	env    env.Provider
	// hc is the hc CLI, resolved on first attach.
	hc string
}

type herdrCoreTerminal struct {
	Address   string            `json:"address"`
	Labels    map[string]string `json:"labels"`
	State     string            `json:"state"`
	CreatedMS int64             `json:"created_ms"`
}

// herdrCoreClient is one connection to a core, carrying JSON-RPC 2.0 messages
// one per line.
type herdrCoreClient struct {
	socket  string
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	logger  zerolog.Logger
	lastID  int
}

func NewHerdrCoreWithLogger(logger zerolog.Logger) Multiplexer {
	return &herdrCore{logger: logger, env: env.Default()}
}

func (h *herdrCore) SetLogger(logger zerolog.Logger) {
	h.logger = logger
}

func (h *herdrCore) Name() string {
	return string(MultiplexerHerdrCore)
}

func (h *herdrCore) Start(_, _ string) error {
	return pkgerrors.New("herdr-core requires a structured agent start")
}

func (h *herdrCore) StartAgent(start AgentStart) error {
	client, err := h.dial()
	if err != nil {
		return err
	}
	defer client.close()

	if _, err := client.lookup(start.SessionName); err == nil {
		return SessionAlreadyExistsError{Name: start.SessionName}
	} else if !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	// bash goes by its path, since an argv[0] naming a template starts that
	// template instead.
	bash, err := exec.LookPath("bash")
	if err != nil {
		return pkgerrors.Wrap(err, "find bash")
	}

	// Run params are saved to disk, so the environment is passed only through
	// the env file.
	envFile, err := herdrCorePaneEnv.writeEnvFile(start.Env)
	if err != nil {
		return err
	}
	command := start.Command
	if envFile != "" {
		command = herdrCorePaneEnv.commandWithEnvFile(envFile, command)
	}
	err = client.call("terminal.run", map[string]any{
		"argv": []string{bash, "-lc", command},
		"cwd":  start.Workspace,
		"labels": map[string]string{
			herdrCoreSessionLabel: start.SessionName,
			herdrCoreSpaceLabel:   "remuda/" + start.SessionName,
		},
		// Otherwise, when Remuda runs inside an hc terminal, the new terminal
		// would become its child: ended along with it, with notes typed into it.
		"no_parent": true,
		"no_notify": true,
	}, nil)
	if err != nil && envFile != "" {
		_ = os.Remove(envFile)
	}
	return err
}

func (h *herdrCore) List() ([]SessionInfo, error) {
	client, err := h.dial()
	if err != nil {
		return nil, err
	}
	defer client.close()

	terminals, err := client.terminals(map[string]any{})
	if err != nil {
		return nil, err
	}
	infos := make([]SessionInfo, 0, len(terminals))
	for _, terminal := range terminals {
		infos = append(infos, terminal.sessionInfo())
	}
	return infos, nil
}

func (h *herdrCore) Find(name string) (SessionInfo, error) {
	client, terminal, err := h.session(name)
	if err != nil {
		return SessionInfo{}, err
	}
	client.close()
	return terminal.sessionInfo(), nil
}

// Attach runs hc attach on the socket the session was found on.
func (h *herdrCore) Attach(name string) error {
	client, terminal, err := h.session(name)
	if err != nil {
		return err
	}
	client.close()
	if h.hc == "" {
		if h.hc, err = findHC(h.env); err != nil {
			return err
		}
	}
	cmd := util.CmdWithEnvAndLogger(h.logger, append(env.Environ(h.env), "HC_SOCKET="+client.socket), h.hc, "attach", terminal.Address)
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stderr, os.Stdin, os.Stderr
	return cmd.Run()
}

func (h *herdrCore) ReadBuffer(name string, lines int) (string, error) {
	client, terminal, err := h.session(name)
	if err != nil {
		return "", err
	}
	defer client.close()

	history := lines
	if lines == 0 {
		history = herdrCoreFullHistory
	}
	var read struct {
		Text string `json:"text"`
	}
	if err := client.call("terminal.read", map[string]any{"terminal": terminal.Address, "lines": history}, &read); err != nil {
		return "", err
	}
	return lastLines(read.Text, lines), nil
}

func (h *herdrCore) Send(name, payload string, appendNewline bool) error {
	if !appendNewline {
		return ErrHerdrCoreSendNeedsNewline
	}
	client, terminal, err := h.session(name)
	if err != nil {
		return err
	}
	defer client.close()

	err = client.call("terminal.send", map[string]any{"terminal": terminal.Address, "text": payload}, nil)
	var reply HerdrCoreError
	if errors.As(err, &reply) && reply.Kind == "blocked" {
		return BlockedSessionError{Name: name, Message: reply.Message}
	}
	return err
}

// Kill ends the live terminal labeled name and removes it, along with the
// finished terminals left under that label.
func (h *herdrCore) Kill(name string) error {
	client, err := h.dial()
	if err != nil {
		return err
	}
	defer client.close()

	terminals, err := client.labeled(name)
	if err != nil {
		return err
	}
	if len(terminals) == 0 {
		return ErrSessionNotFound
	}
	if _, err := liveSession(terminals, name); errors.As(err, new(AmbiguousSessionError)) {
		return err
	}
	for _, terminal := range terminals {
		target := map[string]any{"terminal": terminal.Address}
		// terminal.rm fails on a live terminal.
		if terminal.live() {
			if err := client.call("terminal.kill", target, nil); err != nil {
				return err
			}
		}
		if err := client.call("terminal.rm", target, nil); err != nil {
			return err
		}
	}
	return nil
}

// session dials the core and looks up the live terminal labeled name. The
// caller closes the client.
func (h *herdrCore) session(name string) (*herdrCoreClient, herdrCoreTerminal, error) {
	client, err := h.dial()
	if err != nil {
		return nil, herdrCoreTerminal{}, err
	}
	terminal, err := client.lookup(name)
	if err != nil {
		client.close()
		return nil, herdrCoreTerminal{}, err
	}
	return client, terminal, nil
}

// dial connects to the core and checks that it speaks herdrCoreGeneration.
// Unlike any hc command, dialing never starts a core, so a missing or refused
// socket is ErrHerdrCoreNotRunning.
func (h *herdrCore) dial() (*herdrCoreClient, error) {
	socket, err := h.socket()
	if err != nil {
		return nil, err
	}
	//nolint:noctx // Multiplexer methods take no context.
	conn, err := net.Dial("unix", socket)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return nil, pkgerrors.Wrapf(ErrHerdrCoreNotRunning, "dial %s", socket)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(err, "dial herdr-core")
	}
	client := &herdrCoreClient{
		socket:  socket,
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
		logger:  h.logger,
	}
	var hello struct {
		Generation int `json:"generation"`
	}
	err = client.call("hello", map[string]any{"generation": herdrCoreGeneration}, &hello)
	var reply HerdrCoreError
	switch {
	case errors.As(err, &reply) && reply.Kind == "unsupported_generation":
		err = UnsupportedHerdrCoreGenerationError{Message: reply.Message}
	case err == nil && hello.Generation != herdrCoreGeneration:
		err = UnsupportedHerdrCoreGenerationError{Generation: hello.Generation}
	}
	if err != nil {
		client.close()
		return nil, err
	}
	return client, nil
}

// socket returns the socket hc itself would use: HC_SOCKET, else socket in
// herdr-core's config.toml, else the default socket of the HC_SESSION core.
func (h *herdrCore) socket() (string, error) {
	if socket := h.env.Getenv("HC_SOCKET"); socket != "" {
		return socket, nil
	}
	configHome, err := h.xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(configHome, "herdr-core", "config.toml")
	var config struct {
		Socket string `toml:"socket"`
	}
	if _, err := toml.DecodeFile(configPath, &config); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", pkgerrors.Wrapf(err, "read %s", configPath)
	}
	if config.Socket != "" {
		return config.Socket, nil
	}
	stateHome, err := h.xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state"))
	if err != nil {
		return "", err
	}
	hcSession := h.env.Getenv("HC_SESSION")
	if hcSession == "" {
		hcSession = "default"
	}
	return filepath.Join(stateHome, "herdr-core", hcSession, hcSession+".sock"), nil
}

// xdgDir returns the XDG base directory that variable names, else fallback
// under the home directory.
func (h *herdrCore) xdgDir(variable, fallback string) (string, error) {
	if dir := h.env.Getenv(variable); dir != "" {
		return dir, nil
	}
	home, err := h.env.UserHomeDir()
	if err != nil {
		return "", pkgerrors.Wrap(err, "find home directory")
	}
	return filepath.Join(home, fallback), nil
}

func (c *herdrCoreClient) call(method string, params map[string]any, result any) error {
	// Log only the method: params carry prompt text.
	c.logger.Debug().Str("method", method).Msg("herdr-core request")
	c.lastID++
	if err := c.encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": c.lastID, "method": method, "params": params}); err != nil {
		return pkgerrors.Wrapf(err, "send herdr-core %s", method)
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
			Data    struct {
				Kind string `json:"kind"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := c.decoder.Decode(&reply); err != nil {
		return pkgerrors.Wrapf(err, "read herdr-core %s reply", method)
	}
	if reply.Error != nil {
		return HerdrCoreError{Method: method, Kind: reply.Error.Data.Kind, Message: reply.Error.Message}
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(reply.Result, result); err != nil {
		return pkgerrors.Wrapf(err, "decode herdr-core %s reply", method)
	}
	return nil
}

func (c *herdrCoreClient) close() {
	_ = c.conn.Close()
}

// lookup returns the one live terminal labeled name.
func (c *herdrCoreClient) lookup(name string) (herdrCoreTerminal, error) {
	terminals, err := c.labeled(name)
	if err != nil {
		return herdrCoreTerminal{}, err
	}
	return liveSession(terminals, name)
}

// labeled returns the terminals labeled name, finished ones included.
func (c *herdrCoreClient) labeled(name string) ([]herdrCoreTerminal, error) {
	return c.terminals(map[string]any{"all": true, "labels": map[string]string{herdrCoreSessionLabel: name}})
}

// terminals lists the terminals that run Remuda sessions.
func (c *herdrCoreClient) terminals(params map[string]any) ([]herdrCoreTerminal, error) {
	var listed struct {
		Terminals []herdrCoreTerminal `json:"terminals"`
	}
	if err := c.call("terminal.list", params, &listed); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(listed.Terminals, func(terminal herdrCoreTerminal) bool {
		return !remudaLabel(terminal.Labels[herdrCoreSessionLabel])
	}), nil
}

func (t herdrCoreTerminal) live() bool {
	return t.State == "live"
}

func (t herdrCoreTerminal) id() string {
	return t.Address
}

func (t herdrCoreTerminal) sessionInfo() SessionInfo {
	return SessionInfo{
		Name:        t.Labels[herdrCoreSessionLabel],
		CreatedAt:   time.UnixMilli(t.CreatedMS).UTC(),
		Multiplexer: string(MultiplexerHerdrCore),
	}
}

// findHC locates the hc CLI on PATH, else in ~/.local/bin.
func findHC(provider env.Provider) (string, error) {
	candidates := []string{"hc"}
	if home, err := provider.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "hc"))
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrHCNotFound
}
