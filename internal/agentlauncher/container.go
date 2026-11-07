package agentlauncher

import (
	"errors"
	"io/fs"
	"os"

	"github.com/rs/zerolog"
)

// bindIfExists returns a docker bind mount of src at target, or nil when src
// is missing or isType rejects its mode. A missing source would otherwise be
// created as an empty directory.
func bindIfExists(logger zerolog.Logger, src, target, mode string, isType func(fs.FileMode) bool) []string {
	info, err := os.Stat(src)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn().Err(err).Str("path", src).Msg("failed checking container mount source; skipping mount")
		}
		return nil
	}
	if !isType(info.Mode()) {
		return nil
	}
	return []string{"-v", src + ":" + target + ":" + mode}
}
