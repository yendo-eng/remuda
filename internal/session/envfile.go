package session

import (
	"os"
	"slices"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/util"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

// paneEnv names the variables a multiplexer sets in each of its panes, by
// exact name or by prefix. An env file never overrides them, and
// commandWithEnvFile keeps the pane's own values when it resets the
// environment.
type paneEnv struct {
	multiplexer string
	names       []string
	prefix      string
}

func (p paneEnv) owns(name string) bool {
	return slices.Contains(p.names, name) || (p.prefix != "" && strings.HasPrefix(name, p.prefix))
}

// writeEnvFile writes env as shell exports to a private temp file and returns
// its path, or "" when there is nothing to export.
func (p paneEnv) writeEnvFile(env []string) (string, error) {
	contents := p.envFileContents(env)
	if contents == "" {
		return "", nil
	}

	file, err := os.CreateTemp("", "remuda-"+p.multiplexer+"-env-")
	if err != nil {
		return "", pkgerrors.Wrapf(err, "create %s environment file", p.multiplexer)
	}
	path := file.Name()
	removeFile := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(0o600); err != nil {
		removeFile()
		return "", pkgerrors.Wrapf(err, "set %s environment file permissions", p.multiplexer)
	}
	if _, err := file.WriteString(contents); err != nil {
		removeFile()
		return "", pkgerrors.Wrapf(err, "write %s environment file", p.multiplexer)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", pkgerrors.Wrapf(err, "close %s environment file", p.multiplexer)
	}
	return path, nil
}

func (p paneEnv) envFileContents(env []string) string {
	var contents strings.Builder
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		if !util.IsValidEnvVarName(key) || p.owns(key) {
			continue
		}
		contents.WriteString("export ")
		contents.WriteString(key)
		contents.WriteByte('=')
		contents.WriteString(shellutil.SingleQuote(value))
		contents.WriteByte('\n')
	}
	return contents.String()
}

// commandWithEnvFile runs command in a fresh environment loaded from envFile,
// deleting the file once it has been read.
func (p paneEnv) commandWithEnvFile(envFile, command string) string {
	quotedFile := shellutil.SingleQuote(envFile)
	cleanup := "rm -f -- " + quotedFile
	inner := strings.Join([]string{
		"trap " + shellutil.SingleQuote(cleanup) + " EXIT",
		". " + quotedFile,
		cleanup,
		command,
	}, "; ")
	prelude := ""
	kept := []string{`TERM="$TERM"`}
	for _, name := range p.names {
		kept = append(kept, name+`="$`+name+`"`)
	}
	if p.prefix != "" {
		// ${!PREFIX@} expands to the name of every variable with the prefix.
		prelude = `pane_env=(); for name in "${!` + p.prefix + `@}"; do pane_env+=("$name=${!name}"); done; `
		kept = append(kept, `"${pane_env[@]}"`)
	}
	kept = append(kept, `USER="$USER"`, `LOGNAME="$LOGNAME"`)
	return prelude + "exec env -i " + strings.Join(kept, " ") + " bash -lc " + shellutil.SingleQuote(inner)
}
