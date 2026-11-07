package agentlauncher

import (
	"errors"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
)

// bindIfExists returns a docker bind mount of src at target, or nil when src
// is missing, since a missing source would otherwise be created as an empty
// directory.
func bindIfExists(logger zerolog.Logger, src, target, mode string) []string {
	if _, err := os.Stat(src); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn().Err(err).Str("path", src).Msg("failed checking container mount source; skipping mount")
		}
		return nil
	}
	return []string{"-v", src + ":" + target + ":" + mode}
}

// hostHome returns the host's home directory, or false when it can't be
// resolved or is blank.
func hostHome(provider env.Provider) (string, bool) {
	home, err := provider.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", false
	}
	return home, true
}
