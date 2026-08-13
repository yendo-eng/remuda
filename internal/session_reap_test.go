package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

type fakeMultiplexer struct {
	sessions []session.SessionInfo
	killed   []string
}

func (f *fakeMultiplexer) Name() string { return "fake" }

func (f *fakeMultiplexer) Start(sessionName, command string) error { return nil }

func (f *fakeMultiplexer) List() ([]session.SessionInfo, error) { return f.sessions, nil }

func (f *fakeMultiplexer) Find(name string) (session.SessionInfo, error) {
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (f *fakeMultiplexer) Attach(name string) error { return nil }

func (f *fakeMultiplexer) ReadBuffer(name string, lines int) (string, error) { return "", nil }

func (f *fakeMultiplexer) Send(name string, payload string, appendNewline bool) error {
	return nil
}

func (f *fakeMultiplexer) Kill(name string) error {
	f.killed = append(f.killed, name)
	return nil
}

func TestSessionReapCandidates_FiltersByAge(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	mgr := &fakeMultiplexer{
		sessions: []session.SessionInfo{
			{Name: "org/repo/old", CreatedAt: now.Add(-10 * time.Hour)},
			{Name: "org/repo/edge", CreatedAt: now.Add(-5 * time.Hour)},
			{Name: "org/repo/new", CreatedAt: now.Add(-4 * time.Hour)},
			{Name: "org/repo/unknown"},
			{Name: "not-remuda", CreatedAt: now.Add(-10 * time.Hour)},
		},
	}
	k := Remuda{Multiplexer: mgr}

	candidates, skipped, err := k.SessionReapCandidates(5*time.Hour, now)
	require.NoError(t, err)
	require.Equal(t, []session.SessionInfo{
		{Name: "org/repo/old", CreatedAt: now.Add(-10 * time.Hour)},
		{Name: "org/repo/edge", CreatedAt: now.Add(-5 * time.Hour)},
	}, candidates)
	require.Len(t, skipped, 1)
	require.Equal(t, "org/repo/unknown", skipped[0].Name)
	require.Equal(t, "unknown session age", skipped[0].SkippedReason)
}

func TestSessionReapCandidates_RejectsNonPositive(t *testing.T) {
	k := Remuda{Multiplexer: &fakeMultiplexer{}}
	_, _, err := k.SessionReapCandidates(0, time.Now())
	require.Error(t, err)
}

func TestSessionReap_DryRunCleanupWorkspacePath(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "work")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	k := Remuda{Config: Config{ReposBaseDir: base}}
	results, err := k.SessionReap([]string{"org/repo/work"}, true, true)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].DryRun)
	require.True(t, results[0].Cleanup)
	require.Equal(t, workspace, results[0].WorkspacePath)
	require.Empty(t, results[0].WorkspacePathError)
}

func TestSessionReap_NonDryRunCallsKill(t *testing.T) {
	mgr := &fakeMultiplexer{}
	k := Remuda{Multiplexer: mgr}

	results, err := k.SessionReap([]string{"org/repo/work"}, false, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, []string{"org/repo/work"}, mgr.killed)
}

func TestSessionReap_CleanupSkipsInvalidWorkspacePath(t *testing.T) {
	mgr := &fakeMultiplexer{}
	k := Remuda{Multiplexer: mgr, Config: Config{ReposBaseDir: t.TempDir()}}

	results, err := k.SessionReap([]string{"not-remuda"}, true, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, []string{"not-remuda"}, mgr.killed)
	require.True(t, results[0].Cleanup)
	require.Empty(t, results[0].WorkspacePath)
	require.NotEmpty(t, results[0].WorkspacePathError)
}
