package internal

import (
	"errors"
	"time"

	"github.com/yendo-eng/remuda/internal/session"
)

type ReapedSession struct {
	Name               string
	DryRun             bool
	Cleanup            bool
	WorkspacePath      string
	WorkspacePathError string
	SkippedReason      string
}

func (k Remuda) SessionReapCandidates(
	olderThan time.Duration,
	now time.Time,
) ([]session.SessionInfo, []ReapedSession, error) {
	if olderThan <= 0 {
		return nil, nil, errors.New("olderThan must be positive")
	}

	sessions, err := k.Session.List()
	if err != nil {
		return nil, nil, err
	}

	candidates := make([]session.SessionInfo, 0, len(sessions))
	skipped := []ReapedSession{}
	for _, sess := range sessions {
		if !sess.IsRemudaSession() {
			continue
		}
		if sess.CreatedAt.IsZero() {
			skipped = append(skipped, ReapedSession{
				Name:          sess.Name,
				SkippedReason: "unknown session age",
			})
			continue
		}
		if now.Sub(sess.CreatedAt) >= olderThan {
			candidates = append(candidates, sess)
		}
	}

	return candidates, skipped, nil
}

func (k Remuda) SessionReap(
	names []string,
	cleanup bool,
	dryRun bool,
) ([]ReapedSession, error) {
	results := make([]ReapedSession, 0, len(names))
	for _, name := range names {
		result := ReapedSession{
			Name:    name,
			DryRun:  dryRun,
			Cleanup: cleanup,
		}

		cleanupAllowed := cleanup
		if cleanup {
			workspace, err := session.SessionInfo{Name: name}.WorkspacePathFromRoots(k.workspaceRoots()...)
			if err != nil {
				result.WorkspacePathError = err.Error()
				cleanupAllowed = false
			} else {
				result.WorkspacePath = workspace
			}
		}

		results = append(results, result)

		if dryRun {
			continue
		}
		if err := k.SessionKill(name, cleanupAllowed, nil, false, nil, false); err != nil {
			return results, err
		}
	}

	return results, nil
}
