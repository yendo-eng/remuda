package smolvm

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/util"
)

var ErrMachineNotFound = pkgerrors.New("smolvm machine not found")

// Client manages session machines after launch.
type Client interface {
	// MachineRunning returns ErrMachineNotFound when no machine has name.
	MachineRunning(name string) (bool, error)

	// Exec runs command interactively in the running machine.
	Exec(name string, command string) error

	// Delete stops and deletes the machine, returning ErrMachineNotFound
	// when it does not exist.
	Delete(name string) error
}

type shellClient struct {
	logger zerolog.Logger
}

// NewShellClient returns a Client backed by the smolvm CLI, or false when
// smolvm is not on PATH (no session can have a machine then).
func NewShellClient(logger zerolog.Logger) (Client, bool) {
	if _, err := exec.LookPath("smolvm"); err != nil {
		return nil, false
	}
	return shellClient{logger: logger}, true
}

func (c shellClient) MachineRunning(name string) (bool, error) {
	out, err := c.run("machine", "status", "--name", name, "--json")
	if err != nil {
		return false, err
	}
	var status struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return false, pkgerrors.Wrapf(err, "parse smolvm machine status for %q", name)
	}
	return status.State == "running", nil
}

func (c shellClient) Exec(name string, command string) error {
	cmd := util.Cmd(c.logger, "smolvm", "machine", "exec", "-it", "--name", name, "--", "bash", "-lc", command)
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stdout, os.Stdin, os.Stderr
	return cmd.Run()
}

func (c shellClient) Delete(name string) error {
	_, err := c.run("machine", "delete", "--force", "--name", name)
	return err
}

// run invokes smolvm with JSON errors so NOT_FOUND maps to
// ErrMachineNotFound without matching message text.
func (c shellClient) run(args ...string) ([]byte, error) {
	cmd := util.Cmd(c.logger, "smolvm", append([]string{"--error-format", "json"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var failure struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(stderr.Bytes(), &failure); jsonErr == nil && failure.Error.Code != "" {
		if failure.Error.Code == "NOT_FOUND" {
			return nil, pkgerrors.Wrap(ErrMachineNotFound, failure.Error.Message)
		}
		return nil, pkgerrors.Errorf("smolvm %s: %s: %s", strings.Join(args, " "), failure.Error.Code, failure.Error.Message)
	}
	return nil, pkgerrors.Wrapf(err, "smolvm %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
}
