package main

import (
	"runtime/debug"
	"strings"
)

const (
	develBuildVersion   = "(devel)"
	unknownBuildVersion = "unknown"
)

func resolveVersion(stamped string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if stamped = strings.TrimSpace(stamped); stamped != "" {
		return stamped
	}

	if readBuildInfo == nil {
		return unknownBuildVersion
	}

	info, ok := readBuildInfo()
	if !ok || info == nil {
		return unknownBuildVersion
	}

	mainVersion := strings.TrimSpace(info.Main.Version)
	if mainVersion != "" && mainVersion != develBuildVersion {
		return mainVersion
	}

	revision := strings.TrimSpace(buildSetting(info.Settings, "vcs.revision"))
	if revision != "" {
		if strings.EqualFold(strings.TrimSpace(buildSetting(info.Settings, "vcs.modified")), "true") {
			return revision + "-dirty"
		}
		return revision
	}

	if mainVersion != "" {
		return mainVersion
	}

	return unknownBuildVersion
}

func buildSetting(settings []debug.BuildSetting, key string) string {
	for _, setting := range settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}
