package jira

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Jira interface {
	// GetTicket retrieves the details of a Jira ticket by its ID.
	GetTicket(id string) (string, error)
}

// LoggerSetter allows wiring a per-invocation logger into Jira implementations.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

// AuthConfigSetter allows CLI commands to provide per-invocation Jira auth
// values resolved through Remuda's flag/env/config precedence.
type AuthConfigSetter interface {
	SetAuthConfigOverride(cfg AuthConfig)
}

// BuildContext downloads each ticket and returns a concatenated block suitable for
// prepending to an LLM prompt.
func BuildContext(jira Jira, ids []string) (string, error) {
	var sb strings.Builder
	for _, id := range ids {
		text, err := jira.GetTicket(id)
		if err != nil {
			return "", pkgerrors.Wrapf(err, "get ticket %s", id)
		}
		sb.WriteString("---------- Ticket ")
		sb.WriteString(id)
		sb.WriteString(" ----------\n")
		sb.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}
