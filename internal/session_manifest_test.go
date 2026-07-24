package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeExcludeGit struct {
	excludePath string
	dirSeen     string
}

func (g *fakeExcludeGit) Clone(repoURL, dir string) error                        { return nil }
func (g *fakeExcludeGit) Pull(dir string) error                                  { return nil }
func (g *fakeExcludeGit) WorktreeAdd(dir, branch string, args ...string) error   { return nil }
func (g *fakeExcludeGit) WorktreeRemove(dir string, args ...string) error        { return nil }
func (g *fakeExcludeGit) Checkout(dir string, args ...string) error              { return nil }
func (g *fakeExcludeGit) ShowRef(dir, ref string, opts ...string) error          { return nil }
func (g *fakeExcludeGit) Branch(dir string, args ...string) error                { return nil }
func (g *fakeExcludeGit) RevParse(dir, rev string, opts ...string) (string, error) {
	g.dirSeen = dir
	return g.excludePath, nil
}

func TestWriteSessionManifest(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	excludePath := filepath.Join(workspace, ".git", "info", "exclude")
	g := &fakeExcludeGit{excludePath: excludePath}

	manifest := SessionManifest{
		Agent:          "claude",
		Model:          "claude-opus-4-8",
		ReasoningLevel: "high",
		Yolo:           true,
		UsePrompts:     []string{"make-pr"},
		UsePosition:    "before",
		Container: SessionManifestContainer{
			Enabled: true,
			Image:   "ghcr.io/acme/vibe-dev:latest",
		},
	}
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
	if got.Agent != "claude" || got.Model != "claude-opus-4-8" || !got.Yolo {
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

func TestWriteSessionManifestRefusesExisting(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	g := &fakeExcludeGit{excludePath: filepath.Join(workspace, ".git", "info", "exclude")}

	if err := WriteSessionManifest(g, workspace, SessionManifest{Agent: "codex"}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	err := WriteSessionManifest(g, workspace, SessionManifest{Agent: "claude"})
	if err == nil {
		t.Fatal("expected an error writing over an existing manifest")
	}

	got, _, readErr := ReadSessionManifest(workspace)
	if readErr != nil {
		t.Fatalf("ReadSessionManifest: %v", readErr)
	}
	if got.Agent != "codex" {
		t.Errorf("existing manifest was overwritten: %+v", got)
	}
}

func TestExcludeFromGitIdempotent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	excludePath := filepath.Join(workspace, ".git", "info", "exclude")
	g := &fakeExcludeGit{excludePath: excludePath}

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
