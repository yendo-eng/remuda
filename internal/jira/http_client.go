package jira

import (
	"context"
	"encoding/json"
	"errors"

	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
)

const (
	defaultRequestTimeout  = 10 * time.Second
	defaultCommentPageSize = 50
	defaultErrorBodyLimit  = 8 * 1024
)

const issueFieldsParam = "summary,status,assignee,reporter,priority,created,updated,description,issuetype"

// HTTPClientOption allows overriding HTTP client behavior.
type HTTPClientOption func(*httpClient)

// WithHTTPClient overrides the underlying net/http client.
func WithHTTPClient(client *http.Client) HTTPClientOption {
	return func(c *httpClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithRequestTimeout overrides the per-request timeout.
func WithRequestTimeout(timeout time.Duration) HTTPClientOption {
	return func(c *httpClient) {
		if timeout > 0 {
			c.requestTimeout = timeout
		}
	}
}

// WithCommentPageSize overrides the page size used for comment pagination.
func WithCommentPageSize(pageSize int) HTTPClientOption {
	return func(c *httpClient) {
		if pageSize > 0 {
			c.commentPageSize = pageSize
		}
	}
}

type httpClient struct {
	endpoint        *url.URL
	user            string
	token           string
	httpClient      *http.Client
	requestTimeout  time.Duration
	commentPageSize int
}

// NewHTTPClient builds a Jira Cloud REST API v3 client.
func NewHTTPClient(cfg AuthConfig, opts ...HTTPClientOption) (Client, error) {
	endpoint, err := parseJiraEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	client := &httpClient{
		endpoint:        endpoint,
		user:            strings.TrimSpace(cfg.User),
		token:           strings.TrimSpace(cfg.Token),
		httpClient:      &http.Client{},
		requestTimeout:  defaultRequestTimeout,
		commentPageSize: defaultCommentPageSize,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}

	missing := missingFields(endpoint.String(), client.user, client.token)
	if len(missing) > 0 {
		return nil, missingConfigError(missing, "")
	}

	return client, nil
}

func parseJiraEndpoint(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, pkgerrors.New("missing Jira configuration fields: endpoint. Set REMUDA_JIRA_ENDPOINT")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "parse Jira endpoint %q", trimmed)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, pkgerrors.Errorf("jira endpoint must include scheme and host: %q", trimmed)
	}

	// Ensure ResolveReference treats endpoint path as a directory.
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}

	return parsed, nil
}

func (c *httpClient) GetIssue(ctx context.Context, key string) (Issue, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return Issue{}, pkgerrors.New("jira issue key cannot be empty")
	}

	query := url.Values{}
	query.Set("fields", issueFieldsParam)
	query.Set("expand", "renderedFields")

	var payload issueResponse
	if err := c.getJSON(ctx, issuePath(trimmedKey), query, trimmedKey, &payload); err != nil {
		return Issue{}, err
	}

	return mapIssue(payload)
}

func (c *httpClient) GetComments(ctx context.Context, key string) ([]Comment, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return nil, pkgerrors.New("jira issue key cannot be empty")
	}

	startAt := 0
	comments := make([]Comment, 0, defaultCommentPageSize)

	for {
		query := url.Values{}
		query.Set("startAt", strconv.Itoa(startAt))
		query.Set("maxResults", strconv.Itoa(c.commentPageSize))

		var page commentsPageResponse
		if err := c.getJSON(ctx, commentsPath(trimmedKey), query, trimmedKey, &page); err != nil {
			return nil, err
		}

		mapped, err := mapComments(page.Comments)
		if err != nil {
			return nil, err
		}
		comments = append(comments, mapped...)

		if page.MaxResults <= 0 || len(page.Comments) == 0 {
			break
		}

		nextOffset := page.StartAt + page.MaxResults
		if nextOffset >= page.Total {
			break
		}
		startAt = nextOffset
	}

	sort.SliceStable(comments, func(i, j int) bool {
		left := comments[i]
		right := comments[j]
		if left.Created.Equal(right.Created) {
			return left.ID < right.ID
		}
		return left.Created.Before(right.Created)
	})

	return comments, nil
}

func (c *httpClient) getJSON(ctx context.Context, path string, query url.Values, issueKey string, out any) error {
	endpoint, err := c.resolveURL(path, query)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return pkgerrors.Wrap(err, "create jira request")
	}

	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.user, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return mapTransportError(err, requestCtx.Err(), c.requestTimeout)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return mapStatusError(resp, issueKey)
	}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return pkgerrors.Wrap(err, "decode Jira response")
	}

	return nil
}

func mapTransportError(err error, requestErr error, requestTimeout time.Duration) error {
	if errors.Is(requestErr, context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return pkgerrors.Errorf("jira request timed out after %s", requestTimeout)
	}
	if errors.Is(requestErr, context.Canceled) || errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return pkgerrors.Wrap(err, "jira request failed")
}

func mapStatusError(resp *http.Response, issueKey string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, defaultErrorBodyLimit))
	detail := strings.TrimSpace(extractJiraErrorDetail(body))

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return pkgerrors.Errorf(
			"jira authentication failed (status %d): verify REMUDA_JIRA_API_TOKEN and permissions for %s",
			resp.StatusCode,
			issueKey,
		)
	case http.StatusNotFound:
		return pkgerrors.Errorf("jira issue %s not found or inaccessible (status 404)", issueKey)
	case http.StatusTooManyRequests:
		retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
		if retryAfter != "" {
			return pkgerrors.Errorf("jira API rate limited (status 429): retry after %s seconds", retryAfter)
		}
		return pkgerrors.New("jira API rate limited (status 429): retry later")
	default:
		if detail != "" {
			return pkgerrors.Errorf("jira API request failed with status %d: %s", resp.StatusCode, detail)
		}
		return pkgerrors.Errorf("jira API request failed with status %d", resp.StatusCode)
	}
}

func extractJiraErrorDetail(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return trimmed
	}

	if len(payload.ErrorMessages) > 0 {
		return strings.Join(payload.ErrorMessages, "; ")
	}
	if payload.Message != "" {
		return payload.Message
	}
	if len(payload.Errors) > 0 {
		pairs := make([]string, 0, len(payload.Errors))
		for field, message := range payload.Errors {
			field = strings.TrimSpace(field)
			message = strings.TrimSpace(message)
			if field == "" {
				pairs = append(pairs, message)
				continue
			}
			pairs = append(pairs, field+": "+message)
		}
		sort.Strings(pairs)
		return strings.Join(pairs, "; ")
	}

	return ""
}

func (c *httpClient) resolveURL(path string, query url.Values) (*url.URL, error) {
	relative, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "build Jira request URL for %q", path)
	}
	relative.RawQuery = query.Encode()
	return c.endpoint.ResolveReference(relative), nil
}

func issuePath(key string) string {
	return "rest/api/3/issue/" + url.PathEscape(key)
}

func commentsPath(key string) string {
	return issuePath(key) + "/comment"
}

type issueResponse struct {
	Key            string          `json:"key"`
	Fields         issueFields     `json:"fields"`
	RenderedFields issueHTMLFields `json:"renderedFields"`
}

type issueFields struct {
	Summary     string          `json:"summary"`
	Status      *nameField      `json:"status"`
	Assignee    *jiraUser       `json:"assignee"`
	Reporter    *jiraUser       `json:"reporter"`
	Priority    *nameField      `json:"priority"`
	IssueType   *nameField      `json:"issuetype"`
	Created     string          `json:"created"`
	Updated     string          `json:"updated"`
	Description json.RawMessage `json:"description"`
}

type issueHTMLFields struct {
	Description string `json:"description"`
}

type commentsPageResponse struct {
	StartAt    int               `json:"startAt"`
	MaxResults int               `json:"maxResults"`
	Total      int               `json:"total"`
	Comments   []commentResponse `json:"comments"`
}

type commentResponse struct {
	ID      string          `json:"id"`
	Author  *jiraUser       `json:"author"`
	Created string          `json:"created"`
	Updated string          `json:"updated"`
	Body    json.RawMessage `json:"body"`
}

type jiraUser struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

type nameField struct {
	Name string `json:"name"`
}

func mapIssue(payload issueResponse) (Issue, error) {
	created, err := parseJiraTimestamp(payload.Fields.Created)
	if err != nil {
		return Issue{}, pkgerrors.Wrap(err, "parse issue created timestamp")
	}
	updated, err := parseJiraTimestamp(payload.Fields.Updated)
	if err != nil {
		return Issue{}, pkgerrors.Wrap(err, "parse issue updated timestamp")
	}

	return Issue{
		Key:                 strings.TrimSpace(payload.Key),
		Summary:             payload.Fields.Summary,
		Status:              mapNameFieldToStatus(payload.Fields.Status),
		Assignee:            mapJiraUser(payload.Fields.Assignee),
		Reporter:            mapJiraUser(payload.Fields.Reporter),
		Priority:            mapNameFieldToPriority(payload.Fields.Priority),
		IssueType:           mapNameFieldToIssueType(payload.Fields.IssueType),
		Created:             created,
		Updated:             updated,
		Description:         payload.Fields.Description,
		RenderedDescription: payload.RenderedFields.Description,
	}, nil
}

func mapComments(payload []commentResponse) ([]Comment, error) {
	comments := make([]Comment, 0, len(payload))
	for _, raw := range payload {
		created, err := parseJiraTimestamp(raw.Created)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "parse comment %s created timestamp", raw.ID)
		}
		updated, err := parseJiraTimestamp(raw.Updated)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "parse comment %s updated timestamp", raw.ID)
		}

		comments = append(comments, Comment{
			ID:      raw.ID,
			Author:  mapJiraUser(raw.Author),
			Created: created,
			Updated: updated,
			Body:    raw.Body,
		})
	}
	return comments, nil
}

func parseJiraTimestamp(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, pkgerrors.Errorf("unsupported Jira timestamp %q", raw)
}

func mapJiraUser(user *jiraUser) *User {
	if user == nil {
		return nil
	}
	return &User{
		AccountID:   user.AccountID,
		DisplayName: user.DisplayName,
	}
}

func mapNameFieldToStatus(field *nameField) *Status {
	if field == nil {
		return nil
	}
	return &Status{Name: field.Name}
}

func mapNameFieldToPriority(field *nameField) *Priority {
	if field == nil {
		return nil
	}
	return &Priority{Name: field.Name}
}

func mapNameFieldToIssueType(field *nameField) *IssueType {
	if field == nil {
		return nil
	}
	return &IssueType{Name: field.Name}
}
