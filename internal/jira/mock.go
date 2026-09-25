package jira

import pkgerrors "github.com/pkg/errors"

type Mock struct {
	Tickets map[string]Issue
}

func (m Mock) GetTicket(id string, _ AuthConfig) (Issue, error) {
	issue, ok := m.Tickets[id]
	if !ok {
		return Issue{}, pkgerrors.Errorf("ticket not found: %s", id)
	}
	return issue, nil
}
