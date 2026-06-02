package jira

import (
	"context"
	"encoding/json"
	"time"
)

// Client retrieves Jira issue data from an external source.
type Client interface {
	GetIssue(ctx context.Context, key string) (Issue, error)
	GetComments(ctx context.Context, key string) ([]Comment, error)
}

type Issue struct {
	Key                 string
	Summary             string
	Status              *Status
	Assignee            *User
	Reporter            *User
	Priority            *Priority
	IssueType           *IssueType
	Created             time.Time
	Updated             time.Time
	Description         json.RawMessage
	RenderedDescription string
}

type Comment struct {
	ID      string
	Author  *User
	Created time.Time
	Updated time.Time
	Body    json.RawMessage
}

type User struct {
	AccountID   string
	DisplayName string
}

type Status struct {
	Name string
}

type Priority struct {
	Name string
}

type IssueType struct {
	Name string
}
