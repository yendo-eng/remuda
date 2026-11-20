// Package smolvm runs containerized Remuda sessions as smolvm machines: one
// microVM per session, created, started, exec'd into and deleted by a script
// that runs in the session's multiplexer pane.
package smolvm

import (
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util/shell"
)

// Egress is the machine's outbound allowlist. An empty Egress leaves the
// network open.
type Egress struct {
	AllowHosts        []string
	AllowHostPatterns []string
	AllowCIDRs        []string
	BlockPrivate      bool
	AllowPrivate      []string
}

// Credential binds host env var Env so the guest holds only a placeholder,
// which smolvm swaps for the real value on HTTPS requests to Hosts.
type Credential struct {
	Name      string
	Env       string
	Hosts     []string
	Body      bool
	SetHeader string
}

// Empty reports whether e restricts nothing.
func (e Egress) Empty() bool {
	return len(e.AllowHosts)+len(e.AllowHostPatterns)+len(e.AllowCIDRs)+len(e.AllowPrivate) == 0 && !e.BlockPrivate
}

func (c Credential) flagValue() string {
	v := c.Name + "=" + c.Env + "@" + strings.Join(c.Hosts, ",")
	if c.Body {
		v += ":body"
	}
	if c.SetHeader != "" {
		v += ":set-header=" + c.SetHeader
	}
	return v
}

// Volume is a host directory shared into the guest.
type Volume struct {
	Host     string
	Guest    string
	ReadOnly bool
}

// FileCopy is a single host file copied into the guest after start. smolvm
// shares directories only, so guest writes to these files never reach the
// host.
type FileCopy struct {
	Host  string
	Guest string
}

// EnvVar is a literal guest env var. Its value appears in the machine
// record and argv, so it must never carry a secret.
type EnvVar struct {
	Name  string
	Value string
}

// Machine describes one session machine.
type Machine struct {
	Name           string
	Image          string
	WorkspaceHost  string
	WorkspaceGuest string
	Volumes        []Volume
	Files          []FileCopy
	// GuestDirs are created empty before Files are copied (docker --tmpfs
	// targets, which smolvm has no flag for).
	GuestDirs []string
	Env       []EnvVar
	// SecretEnv names host env vars passed to the agent's exec by reference;
	// smolvm resolves them on the host and never persists the values. They
	// are not set at create because smolvm re-resolves create-time secrets on
	// every later exec, which would make `session shell` fail wherever they
	// are unset.
	SecretEnv   []string
	Egress      Egress
	Credentials []Credential
	// CPUs is the vCPU count; zero keeps smolvm's default.
	CPUs int
}

// LookupEnvFunc reports a host env var's value, as os.LookupEnv does.
type LookupEnvFunc func(name string) (string, bool)

// AddDockerOpts realizes docker run options (-v, --tmpfs, -e) on m. Any other
// option fails, because smolvm has no equivalent Remuda can map it to.
func (m *Machine) AddDockerOpts(opts []string, lookupEnv LookupEnvFunc) error {
	for i := 0; i < len(opts); i++ {
		opt := opts[i]
		flag, value, inline := strings.Cut(opt, "=")
		if !inline {
			if i+1 >= len(opts) {
				return pkgerrors.Errorf("container option %q is missing its value", opt)
			}
			i++
			value = opts[i]
		}
		var err error
		switch flag {
		case "-v", "--volume":
			err = m.addMount(value)
		case "--tmpfs":
			guest, _, _ := strings.Cut(value, ":")
			m.GuestDirs = append(m.GuestDirs, guest)
		case "-e", "--env":
			m.addEnv(value, lookupEnv)
		case "--cpus":
			m.CPUs, err = strconv.Atoi(value)
			if err != nil || m.CPUs < 1 {
				err = pkgerrors.Errorf("container option --cpus=%s: smolvm runtime needs a whole number of CPUs", value)
			}
		default:
			err = pkgerrors.Errorf("container option %q is not supported by the smolvm runtime", opt)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) addMount(spec string) error {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 || !path.IsAbs(parts[0]) || !path.IsAbs(parts[1]) {
		return pkgerrors.Errorf("volume %q: smolvm runtime needs /host/path:/guest/path[:ro|rw]", spec)
	}
	readOnly := false
	if len(parts) == 3 {
		switch parts[2] {
		case "ro":
			readOnly = true
		case "rw":
		default:
			return pkgerrors.Errorf("volume %q: unsupported mode %q", spec, parts[2])
		}
	}
	st, err := os.Stat(parts[0])
	if err != nil {
		return pkgerrors.Wrapf(err, "volume %q", spec)
	}
	switch {
	case st.IsDir():
		m.Volumes = append(m.Volumes, Volume{Host: parts[0], Guest: parts[1], ReadOnly: readOnly})
	case st.Mode().IsRegular():
		m.Files = append(m.Files, FileCopy{Host: parts[0], Guest: parts[1]})
	default:
		return pkgerrors.Errorf("volume %q: host path is neither a directory nor a regular file", spec)
	}
	return nil
}

func (m *Machine) addEnv(spec string, lookupEnv LookupEnvFunc) {
	if name, value, ok := strings.Cut(spec, "="); ok {
		m.Env = append(m.Env, EnvVar{Name: name, Value: value})
		return
	}
	// docker's -e NAME leaves NAME unset when the host has none; smolvm
	// rejects a --secret-env whose host var is unset.
	if _, ok := lookupEnv(spec); ok && !slices.Contains(m.SecretEnv, spec) {
		m.SecretEnv = append(m.SecretEnv, spec)
	}
}

// BindCredentials drops passthrough of credential-bound env vars, so the
// guest sees only their placeholders. It fails when another passthrough var
// carries the same value as a bound one (eg. GITHUB_TOKEN beside a bound
// GH_TOKEN), since that would hand the real secret to the guest anyway.
func (m *Machine) BindCredentials(creds []Credential, lookupEnv LookupEnvFunc) error {
	m.Credentials = creds
	bound := map[string]string{}
	for _, c := range creds {
		value, ok := lookupEnv(c.Env)
		if !ok || value == "" {
			return pkgerrors.Errorf("credential %q: host env var %s is not set", c.Name, c.Env)
		}
		bound[c.Env] = value
	}
	m.SecretEnv = slices.DeleteFunc(m.SecretEnv, func(name string) bool {
		_, ok := bound[name]
		return ok
	})
	for _, name := range m.SecretEnv {
		value, _ := lookupEnv(name)
		for boundName, boundValue := range bound {
			if value == boundValue {
				return pkgerrors.Errorf(
					"env var %s carries the same value as credential-bound %s; bind it as a credential too or drop it",
					name, boundName,
				)
			}
		}
	}
	for _, e := range m.Env {
		if _, ok := bound[e.Name]; ok {
			return pkgerrors.Errorf("env var %s is set literally but is also credential-bound", e.Name)
		}
	}
	return nil
}

func (m Machine) createArgv() []string {
	argv := []string{"smolvm", "machine", "create", "--name", m.Name}
	if strings.HasSuffix(m.Image, ".smolmachine") {
		argv = append(argv, "--from", m.Image)
	} else {
		argv = append(argv, "--image", m.Image)
	}
	argv = append(argv, "-v", m.WorkspaceHost+":"+m.WorkspaceGuest, "-w", m.WorkspaceGuest)
	for _, v := range m.Volumes {
		spec := v.Host + ":" + v.Guest
		if v.ReadOnly {
			spec += ":ro"
		}
		argv = append(argv, "-v", spec)
	}
	if m.CPUs > 0 {
		argv = append(argv, "--cpus", strconv.Itoa(m.CPUs))
	}
	argv = append(argv, "--ssh-agent", "--net")
	for _, h := range m.Egress.AllowHosts {
		argv = append(argv, "--allow-host", h)
	}
	for _, p := range m.Egress.AllowHostPatterns {
		argv = append(argv, "--allow-host-pattern", p)
	}
	for _, c := range m.Egress.AllowCIDRs {
		argv = append(argv, "--allow-cidr", c)
	}
	if m.Egress.BlockPrivate {
		argv = append(argv, "--block-private")
	}
	for _, c := range m.Egress.AllowPrivate {
		argv = append(argv, "--allow-private", c)
	}
	for _, c := range m.Credentials {
		argv = append(argv, "--credential", c.flagValue())
	}
	for _, e := range m.Env {
		argv = append(argv, "-e", e.Name+"="+e.Value)
	}
	return argv
}

// LaunchScript returns the bash script that runs the session in its pane:
// create and start the machine, copy in single files, run inner
// interactively, then delete the machine whether inner succeeded or not.
// Teardown is armed only once create succeeds, so a name collision never
// deletes another session's machine.
//
// It ends with the exec's exit status and never calls exit on the normal
// path, so callers can append commands such as the crash-recovery sleep.
func (m Machine) LaunchScript(inner string) string {
	del := "smolvm machine delete --force --name " + shell.SingleQuote(m.Name) + " >/dev/null 2>&1"
	steps := []string{
		// Host→guest inotify watching walks every shared tree at boot; with
		// the Go module cache mounted that alone took ~20s.
		"SMOL_NO_HOT_RELOAD=1 smolvm machine start --name " + shell.SingleQuote(m.Name) + " >/dev/null",
	}
	dirs := slices.Clone(m.GuestDirs)
	for _, f := range m.Files {
		if dir := path.Dir(f.Guest); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) > 0 {
		steps = append(steps, quoteArgv(append([]string{"smolvm", "machine", "exec", "--name", m.Name, "--", "mkdir", "-p"}, dirs...)))
	}
	for _, f := range m.Files {
		steps = append(steps, quoteArgv([]string{"smolvm", "machine", "cp", f.Host, m.Name + ":" + f.Guest})+" >/dev/null")
	}
	execArgv := []string{"smolvm", "machine", "exec", "-it", "--name", m.Name, "-w", m.WorkspaceGuest}
	for _, name := range m.SecretEnv {
		execArgv = append(execArgv, "--secret-env", name+"="+name)
	}
	steps = append(steps, quoteArgv(append(execArgv, "--", "bash", "-lc", inner)))

	return fmt.Sprintf(
		"if %s >/dev/null; then\ntrap '%s; exit 129' HUP TERM\n%s\nrc=$?\n%s\ntrap - HUP TERM\nelse rc=$?; fi\n(exit $rc)",
		quoteArgv(m.createArgv()), shell.EscapeSingleQuotes(del), strings.Join(steps, " &&\n"), del,
	)
}

func quoteArgv(argv []string) string {
	quoted := make([]string, len(argv))
	for i, tok := range argv {
		quoted[i] = shell.SingleQuote(tok)
	}
	return strings.Join(quoted, " ")
}
