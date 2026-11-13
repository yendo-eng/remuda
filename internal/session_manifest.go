package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/git"
)

// SessionManifestSchemaVersion identifies the shape of SessionManifest. Bump
// it whenever a field is added, removed, or reinterpreted.
const SessionManifestSchemaVersion = 1

// SessionManifestFileName is the manifest written to the workspace root by
// vibe and read back by session resume.
const SessionManifestFileName = ".remuda.json"

// SessionManifest holds just enough of a vibe launch's settings to
// reconstruct the command on session resume. It is written once per
// workspace and never contains secrets.
type SessionManifest struct {
	SchemaVersion  int                      `json:"schema_version"`
	Agent          string                   `json:"agent,omitempty"`
	Model          string                   `json:"model,omitempty"`
	ReasoningLevel string                   `json:"reasoning_level,omitempty"`
	Yolo           bool                     `json:"yolo,omitempty"`
	AgentCmd       string                   `json:"agent_cmd,omitempty"`
	AgentArgs      []string                 `json:"agent_args,omitempty"`
	UsePrompts     []string                 `json:"use_prompts,omitempty"`
	UsePosition    string                   `json:"use_position,omitempty"`
	Container      SessionManifestContainer `json:"container"`
}

// SessionManifestContainer mirrors the container settings of a launch.
type SessionManifestContainer struct {
	Enabled    bool     `json:"enabled,omitempty"`
	Image      string   `json:"image,omitempty"`
	Opts       []string `json:"opts,omitempty"`
	InheritEnv []string `json:"inherit_env,omitempty"`
}

// WriteSessionManifest writes the manifest if absent and registers it in the
// workspace's git exclude file. Existing manifests are preserved.
func WriteSessionManifest(g git.Git, workspace string, manifest SessionManifest) error {
	path := filepath.Join(workspace, SessionManifestFileName)

	manifest.SchemaVersion = SessionManifestSchemaVersion
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return pkgerrors.Wrap(err, "failed to marshal session manifest")
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return excludeFromGit(g, workspace, SessionManifestFileName)
		}
		return pkgerrors.Wrapf(err, "failed to write %s", path)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return pkgerrors.Wrapf(err, "failed to write %s", path)
	}
	if err := f.Close(); err != nil {
		return pkgerrors.Wrapf(err, "failed to close %s", path)
	}

	return excludeFromGit(g, workspace, SessionManifestFileName)
}

// ReadSessionManifest reads the manifest from the workspace root. It returns
// ok=false (with a nil error) when no manifest is present.
func ReadSessionManifest(workspace string) (manifest SessionManifest, ok bool, err error) {
	path := filepath.Join(workspace, SessionManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionManifest{}, false, nil
		}
		return SessionManifest{}, false, pkgerrors.Wrapf(err, "failed to read %s", path)
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return SessionManifest{}, false, pkgerrors.Wrapf(err, "failed to parse %s", path)
	}
	return manifest, true, nil
}

// excludeFromGit adds name to the workspace's git exclude file (local,
// untracked) so it's ignored without touching the repo's own .gitignore.
func excludeFromGit(g git.Git, workspace, name string) error {
	excludePath, err := g.RevParse(workspace, "info/exclude", "--git-path")
	if err != nil {
		return pkgerrors.Wrap(err, "failed to resolve git exclude path")
	}
	excludePath = strings.TrimSpace(excludePath)
	if excludePath == "" {
		return pkgerrors.New("git rev-parse --git-path info/exclude returned an empty path")
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(workspace, excludePath)
	}

	pattern := "/" + name
	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return pkgerrors.Wrapf(err, "failed to read %s", excludePath)
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return pkgerrors.Wrapf(err, "failed to create %s", filepath.Dir(excludePath))
	}
	//nolint:gosec // G302: git itself creates info/exclude world-readable; it holds no secrets.
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to open %s", excludePath)
	}
	defer func() { _ = f.Close() }()

	prefix := ""
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n"
	}
	if _, err := f.WriteString(prefix + pattern + "\n"); err != nil {
		return pkgerrors.Wrapf(err, "failed to write %s", excludePath)
	}
	return nil
}
