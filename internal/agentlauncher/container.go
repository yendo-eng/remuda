package agentlauncher

import (
	"errors"
	"os"

	"github.com/rs/zerolog"
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
