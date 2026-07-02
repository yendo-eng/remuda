package jira

import pkgerrors "github.com/pkg/errors"

type Mock struct {
	Tickets map[string]string
}

func (m Mock) GetTicket(id string) (string, error) {
	text, ok := m.Tickets[id]
	if !ok {
		return "", pkgerrors.Errorf("ticket not found: %s", id)
	}
	return text, nil
}
