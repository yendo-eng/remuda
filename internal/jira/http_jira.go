package jira

import (
	"context"

	"strings"
	"sync"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
)

// NewHTTPJira builds a Jira implementation backed by Jira Cloud REST APIs.
func NewHTTPJira() Jira {
	return NewHTTPJiraWithLogger(logging.DefaultLogger())
}

func NewHTTPJiraWithLogger(logger zerolog.Logger) Jira {
	return &httpJira{
		logger:         logger,
		loadAuthConfig: LoadAuthConfig,
		newClient: func(cfg AuthConfig) (Client, error) {
			return NewHTTPClient(cfg)
		},
	}
}

type httpJira struct {
	logger         zerolog.Logger
	loadAuthConfig func() (AuthConfig, error)
	newClient      func(AuthConfig) (Client, error)

	mu           sync.Mutex
	client       Client
	authOverride AuthConfig
}

func (j *httpJira) SetLogger(logger zerolog.Logger) {
	j.logger = logger
}

func (j *httpJira) SetAuthConfigOverride(cfg AuthConfig) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.authOverride = AuthConfig{
		Endpoint: normalizeEndpoint(cfg.Endpoint),
		User:     strings.TrimSpace(cfg.User),
		Token:    strings.TrimSpace(cfg.Token),
	}
	// Force client reinitialization when auth inputs change.
	j.client = nil
}

func (j *httpJira) GetTicket(id string) (ticket string, err error) {
	key := strings.TrimSpace(id)
	if key == "" {
		return "", pkgerrors.New("jira issue key cannot be empty")
	}

	defer func() {
		if err != nil {
			err = pkgerrors.Wrapf(err, "jira ticket %s", key)
		}
	}()

	client, err := j.getClient()
	if err != nil {
		return "", err
	}

	issue, err := client.GetIssue(context.Background(), key)
	if err != nil {
		return "", pkgerrors.Wrap(err, "get issue")
	}
	if strings.TrimSpace(issue.Key) == "" {
		issue.Key = key
	}

	comments, err := client.GetComments(context.Background(), key)
	if err != nil {
		return "", pkgerrors.Wrap(err, "get comments")
	}

	formatted, err := FormatIssue(issue, comments)
	if err != nil {
		return "", pkgerrors.Wrap(err, "format issue")
	}

	return formatted, nil
}

func (j *httpJira) getClient() (Client, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.client != nil {
		return j.client, nil
	}
	if j.loadAuthConfig == nil {
		j.loadAuthConfig = LoadAuthConfig
	}
	if j.newClient == nil {
		j.newClient = func(cfg AuthConfig) (Client, error) {
			return NewHTTPClient(cfg)
		}
	}

	var cfg AuthConfig
	if isCompleteAuthConfig(j.authOverride) {
		cfg = j.authOverride
	} else {
		loadedCfg, err := j.loadAuthConfig()
		if err != nil {
			return nil, pkgerrors.Wrap(err, "load auth config")
		}
		cfg = mergeAuthConfig(loadedCfg, j.authOverride)
	}

	client, err := j.newClient(cfg)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "create jira client")
	}

	j.client = client
	return j.client, nil
}

func isCompleteAuthConfig(cfg AuthConfig) bool {
	return strings.TrimSpace(cfg.Endpoint) != "" &&
		strings.TrimSpace(cfg.User) != "" &&
		strings.TrimSpace(cfg.Token) != ""
}

func mergeAuthConfig(base AuthConfig, override AuthConfig) AuthConfig {
	merged := base
	if endpoint := strings.TrimSpace(override.Endpoint); endpoint != "" {
		merged.Endpoint = endpoint
	}
	if user := strings.TrimSpace(override.User); user != "" {
		merged.User = user
	}
	if token := strings.TrimSpace(override.Token); token != "" {
		merged.Token = token
	}
	return merged
}
