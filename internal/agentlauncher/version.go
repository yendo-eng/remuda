package agentlauncher

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/mod/semver"
)

var reSemver = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?(?:\+[0-9A-Za-z\.-]+)?`)

// ParseSemver extracts the first semantic version token from a string, ensures
// a leading 'v', and validates it using x/mod/semver.
func ParseSemver(s string) (string, error) {
	m := reSemver.FindString(s)
	if m == "" {
		return "", errors.New("no semver found")
	}
	if !strings.HasPrefix(m, "v") {
		m = "v" + m
	}
	if !semver.IsValid(m) {
		return "", errors.New("invalid semver: " + m)
	}
	return semver.Canonical(m), nil
}

// MeetsMinimum returns true if got is >= min according to semantic versioning.
// If either value is empty, it returns true (nothing to enforce).
func MeetsMinimum(got, min string) bool {
	if got == "" || min == "" {
		return true
	}
	// Expect valid v-prefixed semver strings here.
	return semver.Compare(got, min) >= 0
}

func CheckMinVersion(
	logger zerolog.Logger,
	launcher AgentLauncher,
) error {
	min := launcher.MinimumVersion()
	if strings.TrimSpace(min) != "" {
		if installed, err := launcher.Version(); err != nil {
			logger.Warn().Err(err).Msg("unable to detect agent version; proceeding")
		} else if !MeetsMinimum(installed, min) {
			return fmt.Errorf("agent version too old: require %s, detected %s (use --skip-version-check to bypass)", min, installed)
		}
	}
	return nil
}
