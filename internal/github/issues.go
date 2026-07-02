package github

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	pkgerrors "github.com/pkg/errors"
)

// Issue represents a GitHub issue fetched via gh issue view.
type Issue struct {
	Number    int          `json:"number"`
	Title     string       `json:"title"`
	Body      string       `json:"body"`
	URL       string       `json:"url"`
	State     string       `json:"state"`
	Author    IssueActor   `json:"author"`
	Assignees []IssueActor `json:"assignees"`
	Labels    []IssueLabel `json:"labels"`
}

// IssueActor describes the GitHub user info returned by gh.
type IssueActor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// IssueLabel is a simplified representation of GitHub labels.
type IssueLabel struct {
	Name string `json:"name"`
}

// BuildIssueContext fetches each GitHub issue reference and formats it for prompt injection.
func BuildIssueContext(client GitHub, defaultRepo string, refs []string) (string, error) {
	var sb strings.Builder
	for _, raw := range refs {
		ref, err := parseIssueReference(raw, defaultRepo)
		if err != nil {
			return "", err
		}
		issue, err := client.IssueView(ref.repoSlug, ref.query)
		if err != nil {
			return "", err
		}
		sb.WriteString(formatIssueContext(ref, issue))
	}
	return sb.String(), nil
}

type issueReference struct {
	repoSlug string
	query    string
	display  string
}

func parseIssueReference(raw, defaultRepo string) (issueReference, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return issueReference{}, pkgerrors.Errorf("github issue reference cannot be empty")
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return issueReference{}, pkgerrors.Wrapf(err, "invalid issue URL %q", raw)
		}
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segments) < 4 {
			return issueReference{}, pkgerrors.Errorf("issue URL %q does not look like https://github.com/<owner>/<repo>/issues/<number>", raw)
		}
		owner := segments[0]
		repo := segments[1]
		number := segments[len(segments)-1]
		if !isDigits(number) {
			return issueReference{}, pkgerrors.Errorf("issue number %q in %s is not numeric", number, raw)
		}
		slug := owner + "/" + repo
		return issueReference{
			repoSlug: slug,
			query:    trimmed,
			display:  fmt.Sprintf("%s#%s", slug, number),
		}, nil
	}
	if strings.Contains(trimmed, "#") {
		parts := strings.SplitN(trimmed, "#", 2)
		before := strings.TrimSpace(parts[0])
		after := strings.TrimPrefix(strings.TrimSpace(parts[1]), "#")
		if before == "" || after == "" {
			return issueReference{}, pkgerrors.Errorf("invalid issue reference %q", raw)
		}
		if strings.Count(before, "/") != 1 {
			return issueReference{}, pkgerrors.Errorf("issue reference %q must be owner/repo#number", raw)
		}
		if !isDigits(after) {
			return issueReference{}, pkgerrors.Errorf("issue number %q in %s is not numeric", after, raw)
		}
		return issueReference{
			repoSlug: before,
			query:    after,
			display:  fmt.Sprintf("%s#%s", before, after),
		}, nil
	}
	number := strings.TrimPrefix(trimmed, "#")
	if !isDigits(number) {
		return issueReference{}, pkgerrors.Errorf("issue reference %q must be a URL, owner/repo#number, or numeric id", raw)
	}
	if strings.TrimSpace(defaultRepo) == "" {
		return issueReference{}, pkgerrors.Errorf("--github-issue/--gh-issue %q requires --repo/--repo-url or a fully qualified issue URL", raw)
	}
	return issueReference{
		repoSlug: defaultRepo,
		query:    number,
		display:  fmt.Sprintf("%s#%s", defaultRepo, number),
	}, nil
}

func formatIssueContext(ref issueReference, issue *Issue) string {
	var sb strings.Builder
	display := ref.display
	if display == "" {
		switch {
		case issue != nil && issue.Number > 0 && ref.repoSlug != "":
			display = fmt.Sprintf("%s#%d", ref.repoSlug, issue.Number)
		case issue != nil && issue.Number > 0:
			display = fmt.Sprintf("#%d", issue.Number)
		default:
			display = ref.query
		}
	}
	sb.WriteString("---------- GitHub Issue ")
	sb.WriteString(display)
	sb.WriteString(" ----------\n")
	if issue != nil {
		if title := strings.TrimSpace(issue.Title); title != "" {
			sb.WriteString("Title: ")
			sb.WriteString(title)
			sb.WriteString("\n")
		}
		state := strings.TrimSpace(issue.State)
		if state != "" {
			sb.WriteString("State: ")
			sb.WriteString(strings.ToUpper(state))
			sb.WriteString("\n")
		}
		if author := formatActor(issue.Author); author != "" {
			sb.WriteString("Author: ")
			sb.WriteString(author)
			sb.WriteString("\n")
		}
		if assignees := formatActors(issue.Assignees); assignees != "" {
			sb.WriteString("Assignees: ")
			sb.WriteString(assignees)
			sb.WriteString("\n")
		}
		if labels := formatLabels(issue.Labels); labels != "" {
			sb.WriteString("Labels: ")
			sb.WriteString(labels)
			sb.WriteString("\n")
		}
		if url := strings.TrimSpace(issue.URL); url != "" {
			sb.WriteString("URL: ")
			sb.WriteString(url)
			sb.WriteString("\n")
		}
		if body := strings.TrimSpace(issue.Body); body != "" {
			sb.WriteString("Body:\n")
			sb.WriteString(issue.Body)
			if !strings.HasSuffix(issue.Body, "\n") {
				sb.WriteString("\n")
			}
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func formatActor(actor IssueActor) string {
	login := strings.TrimSpace(actor.Login)
	name := strings.TrimSpace(actor.Name)
	switch {
	case login != "" && name != "" && !strings.EqualFold(login, name):
		return fmt.Sprintf("%s (%s)", login, name)
	case login != "":
		return login
	case name != "":
		return name
	default:
		return ""
	}
}

func formatActors(actors []IssueActor) string {
	list := make([]string, 0, len(actors))
	for _, actor := range actors {
		if formatted := formatActor(actor); formatted != "" {
			list = append(list, formatted)
		}
	}
	return strings.Join(list, ", ")
}

func formatLabels(labels []IssueLabel) string {
	if len(labels) == 0 {
		return ""
	}
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if name := strings.TrimSpace(label.Name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
