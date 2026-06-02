package jira

import "fmt"

type Mock struct {
	Tickets map[string]string
}

func (m Mock) GetTicket(id string) (string, error) {
	text, ok := m.Tickets[id]
	if !ok {
		return "", fmt.Errorf("ticket not found: %s", id)
	}
	return text, nil
}
