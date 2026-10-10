package jira

import (
	"context"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// NewHTTPJira builds a Jira implementation backed by Jira Cloud REST APIs.
func NewHTTPJira(logger zerolog.Logger) Jira {
	return &httpJira{
		logger: logger,
		newClient: func(cfg AuthConfig) (Client, error) {
			return NewHTTPClient(cfg)
		},
	}
}

type httpJira struct {
	logger    zerolog.Logger
	newClient func(AuthConfig) (Client, error)
}

func (j *httpJira) GetTicket(id string, auth AuthConfig) (issue Issue, err error) {
	key := strings.TrimSpace(id)
	if key == "" {
		return Issue{}, pkgerrors.New("jira issue key cannot be empty")
	}

	defer func() {
		if err != nil {
			err = pkgerrors.Wrapf(err, "jira ticket %s", key)
		}
	}()

	client, err := j.newClient(auth)
	if err != nil {
		return Issue{}, pkgerrors.Wrap(err, "create jira client")
	}

	issue, err = client.GetIssue(context.Background(), key)
	if err != nil {
		return Issue{}, pkgerrors.Wrap(err, "get issue")
	}
	if strings.TrimSpace(issue.Key) == "" {
		issue.Key = key
	}

	issue.Comments, err = client.GetComments(context.Background(), key)
	if err != nil {
		return Issue{}, pkgerrors.Wrap(err, "get comments")
	}

	return issue, nil
}
