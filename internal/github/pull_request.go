package github

import (
	"fmt"
	"regexp"
)

// RepoURLFromPR parses a GitHub PR URL and returns the repo clone URL.
func RepoURLFromPR(ref string) string {
	// Accept forms like https://github.com/org/repo/pull/123 and ssh URLs if present.
	re := regexp.MustCompile(`^https?://github.com/([^/]+)/([^/]+)/pull/\d+(?:/.*)?$`)
	m := re.FindStringSubmatch(ref)
	if len(m) == 3 {
		return fmt.Sprintf("https://github.com/%s/%s.git", m[1], m[2])
	}
	return ""
}
