package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

type fakeGit struct {
	excludePath string
	dirSeen     string
}

func sessionManifestWithAgent(agent string) SessionManifest {
	return SessionManifest{Defaults: configfile.DefaultsV1{Agent: &agent}}
}

func (g *fakeGit) Clone(repoURL, dir string) error                      { return nil }
func (g *fakeGit) Pull(dir string) error                                { return nil }
func (g *fakeGit) WorktreeAdd(dir, branch string, args ...string) error { return nil }
func (g *fakeGit) WorktreeRemove(dir string, args ...string) error      { return nil }
func (g *fakeGit) WorktreeLock(dir, worktree, reason string) error      { return nil }
func (g *fakeGit) WorktreeUnlock(dir, worktree string) error            { return nil }
func (g *fakeGit) Checkout(dir string, args ...string) error            { return nil }
func (g *fakeGit) ShowRef(dir, ref string, opts ...string) error        { return nil }
func (g *fakeGit) Branch(dir string, args ...string) error              { return nil }
func (g *fakeGit) RevParse(dir, rev string, opts ...string) (string, error) {
	g.dirSeen = dir
	if rev == "info/exclude" && g.excludePath == "" {
		return filepath.Join(dir, ".git", "info", "exclude"), nil
	}
	return g.excludePath, nil
}

func TestWriteSessionManifest(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	excludePath := filepath.Join(workspace, ".git", "info", "exclude")
	g := &fakeGit{excludePath: excludePath}

	agent := "claude"
	model := "claude-opus-4-8"
	reasoningLevel := "high"
	yolo := true
	usePrompts := []string{"make-pr"}
	usePromptsPosition := "before"
	containerEnabled := true
	isolationRuntime := "docker"
	containerImage := "ghcr.io/acme/vibe-dev:latest"
	manifest := SessionManifest{Defaults: configfile.DefaultsV1{
		Agent:              &agent,
		Model:              &model,
		ReasoningLevel:     &reasoningLevel,
		Yolo:               &yolo,
		UsePrompts:         &usePrompts,
		UsePromptsPosition: &usePromptsPosition,
		Isolation: &configfile.IsolationV1{
			Enabled: &containerEnabled,
			Runtime: &isolationRuntime,
			Image:   &containerImage,
		},
	}}
	if err := WriteSessionManifest(g, workspace, manifest); err != nil {
		t.Fatalf("WriteSessionManifest: %v", err)
	}

	got, ok, err := ReadSessionManifest(workspace)
	if err != nil {
		t.Fatalf("ReadSessionManifest: %v", err)
	}
	if !ok {
		t.Fatal("expected manifest to be found")
	}
	if got.SchemaVersion != SessionManifestSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, SessionManifestSchemaVersion)
	}
	if got.Defaults.Agent == nil || *got.Defaults.Agent != "claude" ||
		got.Defaults.Model == nil || *got.Defaults.Model != "claude-opus-4-8" ||
		got.Defaults.Yolo == nil || !*got.Defaults.Yolo {
		t.Errorf("unexpected manifest: %+v", got)
	}

	excludeData, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("reading exclude file: %v", err)
	}
	if !strings.Contains(string(excludeData), "/"+SessionManifestFileName) {
		t.Errorf("exclude file %q missing manifest pattern:\n%s", excludePath, excludeData)
	}
}

func TestWriteSessionManifestPreservesExisting(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	g := &fakeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}

	require.NoError(t, WriteSessionManifest(g, workspace, sessionManifestWithAgent("codex")))
	require.NoError(t, WriteSessionManifest(g, workspace, sessionManifestWithAgent("claude")))

	got, ok, err := ReadSessionManifest(workspace)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, got.Defaults.Agent)
	require.Equal(t, "codex", *got.Defaults.Agent)
}

func TestExcludeFromGitIdempotent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	excludePath := filepath.Join(workspace, ".git", "info", "exclude")
	g := &fakeGit{excludePath: excludePath}

	if err := excludeFromGit(g, workspace, SessionManifestFileName); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := excludeFromGit(g, workspace, SessionManifestFileName); err != nil {
		t.Fatalf("second call: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("reading exclude file: %v", err)
	}
	if strings.Count(string(data), "/"+SessionManifestFileName) != 1 {
		t.Errorf("expected exactly one entry, got:\n%s", data)
	}
}

func TestReadSessionManifestMissing(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, ok, err := ReadSessionManifest(workspace)
	if err != nil {
		t.Fatalf("ReadSessionManifest: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for a workspace with no manifest")
	}
}

func TestReadSessionManifestRejectsSchemaOne(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, SessionManifestFileName)
	require.NoError(t, os.WriteFile(path, []byte(`{"schema_version":1,"agent":"claude","model":"claude-opus-4-8","yolo":true}`), 0o600))

	_, ok, err := ReadSessionManifest(workspace)
	require.Error(t, err)
	require.False(t, ok)
	require.ErrorContains(t, err, path)
}
