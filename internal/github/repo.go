package github

import (
	"fmt"
	"strings"
	"sync"

	pkgerrors "github.com/pkg/errors"
)

func ParseRepo(url string) (org, repo string, err error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(url, ".git"))

	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", "", pkgerrors.Errorf("unable to parse repo URL: %s", url)
	}
	repo = parts[len(parts)-1]
	org = parts[len(parts)-2]
	if colon := strings.LastIndex(org, ":"); colon >= 0 {
		org = org[colon+1:]
	}
	if org == "" || repo == "" {
		return "", "", pkgerrors.Errorf("unable to parse repo URL: %s", url)
	}
	return org, repo, err
}

// ExpandRepoURL normalizes user-provided repo URL input.
// It expands shorthand GitHub repos like "github.com/org/repo" into
// "https://github.com/org/repo.git" and trims surrounding whitespace.
func ExpandRepoURL(repoURL string) string {
	trimmed := strings.TrimSpace(repoURL)
	if trimmed == "" {
		return ""
	}

	const githubPrefix = "github.com/"
	if len(trimmed) <= len(githubPrefix) || !strings.EqualFold(trimmed[:len(githubPrefix)], githubPrefix) {
		return trimmed
	}

	slug := strings.TrimSuffix(trimmed[len(githubPrefix):], "/")
	if strings.Count(slug, "/") != 1 {
		return trimmed
	}

	owner, repo, ok := strings.Cut(slug, "/")
	if !ok {
		return trimmed
	}
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	repo = strings.TrimSuffix(repo, ".git")
	if owner == "" || repo == "" {
		return trimmed
	}
	if strings.ContainsAny(owner, "?#") || strings.ContainsAny(repo, "?#") {
		return trimmed
	}

	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
}

// repoAliases maps configured aliases to full clone URLs.
// Access is protected by repoAliasesMu.
var repoAliases = map[string]string{}

var repoAliasesMu sync.RWMutex

// builtinAliases holds a pristine copy of the initial aliases
// for use by ResetRepoAliases.
var builtinAliases = func() map[string]string {
	out := make(map[string]string, len(repoAliases))
	for k, v := range repoAliases {
		out[k] = v
	}
	return out
}()

func RepoOrURL(url string, alias string) (string, error) {
	if strings.TrimSpace(url) != "" {
		return url, nil
	}

	fullURL, ok := ExpandRepoAlias(alias)
	if !ok {
		return "", pkgerrors.Errorf("unknown repo alias: %s", alias)
	}

	return fullURL, nil
}

func RepoAliases() map[string]string {
	repoAliasesMu.RLock()
	defer repoAliasesMu.RUnlock()
	// Return a copy to avoid accidental mutation.
	out := make(map[string]string, len(repoAliases))
	for k, v := range repoAliases {
		out[k] = v
	}
	return out
}

// ExpandRepoAlias returns the full clone URL for a supported alias.
// It returns ("", false) when the alias is unknown.
func ExpandRepoAlias(alias string) (string, bool) {
	if alias == "" {
		return "", false
	}
	repoAliasesMu.RLock()
	defer repoAliasesMu.RUnlock()
	if url, ok := repoAliases[strings.ToLower(alias)]; ok {
		return url, true
	}
	return "", false
}

// MergeRepoAliases merges user-defined aliases into the current set.
// On key collision, user-defined values override existing entries.
// Empty keys/values are skipped. Values are trimmed and validated to reject
// URLs that could cause argument injection (e.g., starting with "-").
func MergeRepoAliases(user map[string]string) {
	if len(user) == 0 {
		return
	}
	repoAliasesMu.Lock()
	defer repoAliasesMu.Unlock()
	for k, v := range user {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if key == "" || val == "" {
			continue
		}
		// Reject URLs that could be interpreted as command-line options.
		if strings.HasPrefix(val, "-") {
			continue
		}
		repoAliases[key] = val
	}
}

// ResetRepoAliases restores the alias map to its initial state.
// Useful for tests to avoid order-dependent alias accumulation.
func ResetRepoAliases() {
	repoAliasesMu.Lock()
	defer repoAliasesMu.Unlock()
	repoAliases = make(map[string]string, len(builtinAliases))
	for k, v := range builtinAliases {
		repoAliases[k] = v
	}
}

// RepoSlugFromURL returns the "owner/repo" slug for a GitHub clone URL.
func RepoSlugFromURL(repoURL string) (string, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return "", pkgerrors.Errorf("repository URL is required to resolve GitHub issue context")
	}
	trimmed := strings.TrimSuffix(repoURL, ".git")
	trimmed = strings.TrimSuffix(trimmed, "/")
	switch {
	case strings.HasPrefix(trimmed, "git@"):
		colon := strings.Index(trimmed, ":")
		if colon == -1 || colon+1 >= len(trimmed) {
			return "", pkgerrors.Errorf("unable to parse git URL: %s", repoURL)
		}
		return trimmed[colon+1:], nil
	case strings.HasPrefix(trimmed, "ssh://"):
		// ssh://git@github.com/owner/repo
		parts := strings.Split(trimmed, "/")
		if len(parts) < 4 {
			return "", pkgerrors.Errorf("unable to parse ssh URL: %s", repoURL)
		}
		owner := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		return owner + "/" + repo, nil
	case strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://"):
		parts := strings.Split(trimmed, "/")
		if len(parts) < 3 {
			return "", pkgerrors.Errorf("unable to parse https URL: %s", repoURL)
		}
		owner := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		return owner + "/" + repo, nil
	default:
		if strings.Count(trimmed, "/") == 1 {
			return trimmed, nil
		}
	}
	return "", pkgerrors.Errorf("unable to derive owner/repo slug from %s", repoURL)
}

// ValidateRepoURL ensures a repository URL looks like a supported git remote URL.
func ValidateRepoURL(repoURL string) error {
	trimmed := strings.TrimSpace(repoURL)
	if trimmed == "" {
		return pkgerrors.Errorf("repository URL is required")
	}
	if !strings.HasPrefix(trimmed, "https://") &&
		!strings.HasPrefix(trimmed, "http://") &&
		!strings.HasPrefix(trimmed, "ssh://") &&
		!strings.HasPrefix(trimmed, "git@") {
		return pkgerrors.Errorf("repository URL must start with https://, http://, ssh://, or git@")
	}
	_, err := RepoSlugFromURL(trimmed)
	return err
}
