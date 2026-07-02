package internal

import (
	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/session"
)

func (k Remuda) SessionList() ([]session.SessionInfo, error) {
	sessions, err := k.Session.List()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "list sessions")
	}

	filtered := make([]session.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		if s.IsRemudaSession() {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	return filtered, nil
}
