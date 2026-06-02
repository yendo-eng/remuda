package llm

import (
	"context"
	"regexp"
	"strings"

	"github.com/yendo-eng/remuda/internal/logging"
)

// localService implements a deterministic, offline slugify fallback.
type localService struct{}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// LocalSlugify returns the local fallback slug without network calls.
func LocalSlugify(prompt string) (string, error) {
	return (&localService{}).Slugify(context.Background(), prompt)
}

func (l *localService) Slugify(ctx context.Context, prompt string) (string, error) {
	logger := logging.FromContext(ctx)
	logger.Debug().
		Int("prompt_len", len(prompt)).
		Msg("local slugify requested")
	s := strings.ToLower(strings.TrimSpace(prompt))
	s = strings.ReplaceAll(s, "'", "")
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = "slug"
	}
	return s, nil
}
