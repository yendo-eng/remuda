package main

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	t.Run("uses ldflags stamped version when present", func(t *testing.T) {
		t.Parallel()

		version := resolveVersion(" v9.9.9 ", func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
		})

		require.Equal(t, "v9.9.9", version)
	})

	t.Run("uses module version for go install style builds", func(t *testing.T) {
		t.Parallel()

		version := resolveVersion("", func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true
		})

		require.Equal(t, "v1.2.3", version)
	})

	t.Run("uses vcs revision for local source builds", func(t *testing.T) {
		t.Parallel()

		version := resolveVersion("", func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{
				Main: debug.Module{Version: develBuildVersion},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "0123456789abcdef"},
					{Key: "vcs.modified", Value: "false"},
				},
			}, true
		})

		require.Equal(t, "0123456789abcdef", version)
	})

	t.Run("includes dirty suffix when working tree is modified", func(t *testing.T) {
		t.Parallel()

		version := resolveVersion("", func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{
				Main: debug.Module{Version: develBuildVersion},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "cafebabe"},
					{Key: "vcs.modified", Value: "true"},
				},
			}, true
		})

		require.Equal(t, "cafebabe-dirty", version)
	})

	t.Run("falls back to unknown when build metadata is unavailable", func(t *testing.T) {
		t.Parallel()

		version := resolveVersion("", func() (*debug.BuildInfo, bool) {
			return nil, false
		})

		require.Equal(t, unknownBuildVersion, version)
	})
}
