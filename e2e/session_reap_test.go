package e2e_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/e2e/testutils"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionReapNoCandidates(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	mgr := requireMockSessionManager(t, h)

	addSession(t, mgr, "org/repo/new", time.Now())

	res := h.RunOK("session", "reap", "--older-than", "24h")

	require.Contains(t, res.Stdout, "No sessions to reap.")
	require.NotContains(t, res.Stdout, "would kill")
	requireSessionExists(t, mgr, "org/repo/new")
}

func TestSessionReapDryRun(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	mgr := requireMockSessionManager(t, h)
	now := time.Now()

	addSession(t, mgr, "org/repo/old", now.Add(-48*time.Hour))
	addSession(t, mgr, "org/repo/new", now.Add(-1*time.Hour))

	res := h.RunOK("session", "reap", "--older-than", "24h")

	require.Contains(t, res.Stdout, "would kill org/repo/old\n")
	require.Contains(t, res.Stdout, "would kill 1 session\n")
	require.NotContains(t, res.Stdout, "org/repo/new")
	require.NotContains(t, res.Stdout, "killed org/repo/old")
	require.NotContains(t, res.Stdout, "would remove")
	requireSessionExists(t, mgr, "org/repo/old")
	requireSessionExists(t, mgr, "org/repo/new")
}

func TestSessionReapKillsWhenDryRunFalse(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	mgr := requireMockSessionManager(t, h)
	now := time.Now()

	addSession(t, mgr, "org/repo/old", now.Add(-48*time.Hour))
	addSession(t, mgr, "org/repo/new", now.Add(-1*time.Hour))

	res := h.RunOK("session", "reap", "--older-than", "24h", "--dry-run=false")

	require.Contains(t, res.Stdout, "killed org/repo/old\n")
	require.Contains(t, res.Stdout, "killed 1 session\n")
	require.NotContains(t, res.Stdout, "org/repo/new")
	requireSessionMissing(t, mgr, "org/repo/old")
	requireSessionExists(t, mgr, "org/repo/new")
}

func TestSessionReapCleanupRemovesWorkspace(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	mgr := requireMockSessionManager(t, h)
	h.SetEnv("REMUDA_CONTAINER", "false")

	remoteURL := testutils.InitTestRemote(t)
	name := "test-session"

	org, repo, err := github.ParseRepo(remoteURL)
	require.NoError(t, err)

	workspacePath := filepath.Join(h.RemudaConfig.ReposBaseDir, org, repo, name)
	sessionName := session.SessionNameFromWorkspaceName(workspacePath)

	h.RunOK("vibe", "--name", name, "--repo-url", remoteURL)

	setSessionCreatedAt(t, mgr, sessionName, time.Now().Add(-2*time.Hour))

	res := h.RunOK("session", "reap", "--older-than", "1s", "--dry-run=false", "--cleanup")

	require.Contains(t, res.Stdout, "killed "+sessionName+"\n")
	require.Contains(t, res.Stdout, "removed "+workspacePath+"\n")
	require.Contains(t, res.Stdout, "killed 1 session\n")
	require.NoDirExists(t, workspacePath)
	requireSessionMissing(t, mgr, sessionName)
}

func TestSessionReapSkipsUnknownAge(t *testing.T) {
	t.Parallel()
	h := testutils.NewHarness(t)
	mgr := requireMockSessionManager(t, h)

	addSession(t, mgr, "org/repo/unknown", time.Time{})

	res := h.RunOK("session", "reap", "--older-than", "24h")

	require.Contains(t, res.Stdout, "skipped org/repo/unknown: unknown age\n")
	requireSessionExists(t, mgr, "org/repo/unknown")
}

func requireMockSessionManager(t *testing.T, h *testutils.Harness) *testutils.MockSessionManager {
	t.Helper()

	mgr, ok := h.Session.(*testutils.MockSessionManager)
	require.True(t, ok)
	return mgr
}

func addSession(t *testing.T, mgr *testutils.MockSessionManager, name string, createdAt time.Time) {
	t.Helper()

	require.NoError(t, mgr.Start(name, "cmd"))
	setSessionCreatedAt(t, mgr, name, createdAt)
}

func setSessionCreatedAt(t *testing.T, mgr *testutils.MockSessionManager, name string, createdAt time.Time) {
	t.Helper()

	sess := mgr.FindSession(name)
	require.NotNil(t, sess)
	sess.CreatedAt = createdAt
}

func requireSessionExists(t *testing.T, mgr *testutils.MockSessionManager, name string) {
	t.Helper()

	_, err := mgr.Find(name)
	require.NoError(t, err)
}

func requireSessionMissing(t *testing.T, mgr *testutils.MockSessionManager, name string) {
	t.Helper()

	_, err := mgr.Find(name)
	require.ErrorIs(t, err, session.ErrSessionNotFound)
}
