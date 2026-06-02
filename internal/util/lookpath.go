package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func lookupPathWithEnv(file string, env []string) (string, error) {
	if file == "" {
		return "", exec.ErrNotFound
	}

	if hasPathSeparator(file) || hasVolume(file) {
		return lookupExecutablePath(file, env)
	}

	pathEnv := envValue(env, "PATH")
	if pathEnv == "" {
		return "", exec.ErrNotFound
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, file)
		if path, err := lookupExecutablePath(candidate, env); err == nil {
			return path, nil
		}
	}

	return "", exec.ErrNotFound
}

func lookupExecutablePath(path string, env []string) (string, error) {
	if runtime.GOOS != "windows" {
		if isExecutable(path) {
			return path, nil
		}
		return "", exec.ErrNotFound
	}

	if filepath.Ext(path) != "" {
		if isExecutable(path) {
			return path, nil
		}
		return "", exec.ErrNotFound
	}

	for _, ext := range splitPathExt(env) {
		candidate := path + ext
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	return "", exec.ErrNotFound
}

func splitPathExt(env []string) []string {
	raw := envValue(env, "PATHEXT")
	if raw == "" {
		raw = ".com;.exe;.bat;.cmd"
	}
	parts := filepath.SplitList(raw)
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return parts
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func envValue(env []string, key string) string {
	var value string
	for _, entry := range env {
		envKey, envValue, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if envKeyMatches(envKey, key) {
			value = envValue
		}
	}
	return value
}

func envKeyMatches(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func hasPathSeparator(path string) bool {
	if strings.ContainsRune(path, os.PathSeparator) {
		return true
	}
	return runtime.GOOS == "windows" && strings.ContainsRune(path, '/')
}

func hasVolume(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return filepath.VolumeName(path) != ""
}
