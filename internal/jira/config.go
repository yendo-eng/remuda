package jira

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yendo-eng/remuda/internal/configfile"
)

const (
	remudaConfigEnvVar     = "REMUDA_CONFIG"
	xdgConfigHomeEnvVar    = "XDG_CONFIG_HOME"
	configPathSuffix       = "remuda/config.yaml"
	legacyConfigPathSuffix = ".remuda/config.yaml"
	missingEndpointHint = "REMUDA_JIRA_ENDPOINT"
	missingUserHint     = "REMUDA_JIRA_USER"
	missingAPITokenHint = "REMUDA_JIRA_API_TOKEN (or REMUDA_JIRA_TOKEN)" //nolint:gosec // G101: user-facing env-var hint, not a credential.
	missingConfigKeysHint  = "jira.endpoint, jira.user, jira.api_token"
)

// AuthConfig is the resolved Jira Cloud authentication configuration.
type AuthConfig struct {
	Endpoint string
	User     string
	Token    string
}

type resolvedFileConfig struct {
	Endpoint string
	User     string
	Token    string
}

// LoadAuthConfig resolves Jira endpoint/user/token from Remuda config + env.
// Precedence is env > config.
func LoadAuthConfig() (AuthConfig, error) {
	configPath, strict, err := discoverRemudaConfigPath(os.LookupEnv, os.UserHomeDir)
	if err != nil {
		return AuthConfig{}, err
	}
	return loadAuthConfig(configPath, strict, os.LookupEnv, os.ReadFile)
}

func loadAuthConfig(
	configPath string,
	strictConfig bool,
	lookupEnv func(string) (string, bool),
	readFile func(string) ([]byte, error),
) (AuthConfig, error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if readFile == nil {
		readFile = os.ReadFile
	}

	fileCfg, err := readConfigFile(configPath, strictConfig, readFile)
	if err != nil {
		return AuthConfig{}, err
	}

	endpoint := normalizeEndpoint(fileCfg.Endpoint)
	user := strings.TrimSpace(fileCfg.User)
	token := strings.TrimSpace(fileCfg.Token)

	if endpointOverride, ok := lookupFirstNonEmptyEnv(lookupEnv, "REMUDA_JIRA_ENDPOINT"); ok {
		endpoint = normalizeEndpoint(endpointOverride)
	}
	if userOverride, ok := lookupFirstNonEmptyEnv(lookupEnv, "REMUDA_JIRA_USER"); ok {
		user = strings.TrimSpace(userOverride)
	}
	if tokenOverride, ok := lookupFirstNonEmptyEnv(lookupEnv, "REMUDA_JIRA_API_TOKEN", "REMUDA_JIRA_TOKEN"); ok {
		token = tokenOverride
	}

	missing := missingFields(endpoint, user, token)
	if len(missing) > 0 {
		return AuthConfig{}, missingConfigError(missing, configPath)
	}

	return AuthConfig{
		Endpoint: endpoint,
		User:     user,
		Token:    token,
	}, nil
}

func discoverRemudaConfigPath(
	lookupEnv func(string) (string, bool),
	userHomeDir func() (string, error),
) (path string, strict bool, err error) {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if userHomeDir == nil {
		userHomeDir = os.UserHomeDir
	}

	if override, ok := lookupFirstNonEmptyEnv(lookupEnv, remudaConfigEnvVar); ok {
		expanded, expandErr := expandHomePath(override, userHomeDir)
		if expandErr != nil {
			return "", true, fmt.Errorf("expand %s path %q: %w", remudaConfigEnvVar, override, expandErr)
		}
		return expanded, true, nil
	}

	if xdgHome, ok := lookupFirstNonEmptyEnv(lookupEnv, xdgConfigHomeEnvVar); ok {
		xdgPath := filepath.Join(xdgHome, configPathSuffix)
		exists, existsErr := pathExists(xdgPath)
		if existsErr != nil {
			return "", false, fmt.Errorf("check Remuda config %q: %w", xdgPath, existsErr)
		}
		if exists {
			return xdgPath, false, nil
		}
	} else if home, homeErr := userHomeDir(); homeErr == nil && strings.TrimSpace(home) != "" {
		fallbackPath := filepath.Join(home, ".config", configPathSuffix)
		exists, existsErr := pathExists(fallbackPath)
		if existsErr != nil {
			return "", false, fmt.Errorf("check Remuda config %q: %w", fallbackPath, existsErr)
		}
		if exists {
			return fallbackPath, false, nil
		}
	}

	home, homeErr := userHomeDir()
	if homeErr != nil || strings.TrimSpace(home) == "" {
		return "", false, nil
	}
	legacyPath := filepath.Join(home, legacyConfigPathSuffix)
	exists, existsErr := pathExists(legacyPath)
	if existsErr != nil {
		return "", false, fmt.Errorf("check Remuda config %q: %w", legacyPath, existsErr)
	}
	if exists {
		return legacyPath, false, nil
	}

	return "", false, nil
}

func expandHomePath(raw string, userHomeDir func() (string, error)) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/")), nil
	}
	if trimmed == "~" {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(home) == "" {
			return "", errors.New("home directory is empty")
		}
		return home, nil
	}
	return trimmed, nil
}

func pathExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func readConfigFile(path string, strict bool, readFile func(string) ([]byte, error)) (resolvedFileConfig, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return resolvedFileConfig{}, nil
	}

	raw, err := readFile(trimmedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !strict {
			return resolvedFileConfig{}, nil
		}
		return resolvedFileConfig{}, fmt.Errorf("read Remuda config %q: %w", trimmedPath, err)
	}

	cfg, err := configfile.ParseV1(raw)
	if err != nil {
		return resolvedFileConfig{}, fmt.Errorf("parse Remuda config %q: %w", trimmedPath, err)
	}
	if cfg.Jira == nil {
		return resolvedFileConfig{}, nil
	}

	fileCfg := resolvedFileConfig{}
	if cfg.Jira.Endpoint != nil {
		fileCfg.Endpoint = strings.TrimSpace(*cfg.Jira.Endpoint)
	}
	if cfg.Jira.User != nil {
		fileCfg.User = strings.TrimSpace(*cfg.Jira.User)
	}
	if cfg.Jira.APIToken != nil {
		fileCfg.Token = strings.TrimSpace(*cfg.Jira.APIToken)
	}

	return fileCfg, nil
}

func normalizeEndpoint(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	return strings.TrimRight(trimmed, "/")
}

func lookupFirstNonEmptyEnv(lookupEnv func(string) (string, bool), keys ...string) (string, bool) {
	for _, key := range keys {
		val, ok := lookupEnv(key)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			continue
		}
		return trimmed, true
	}
	return "", false
}

func missingFields(endpoint string, user string, token string) []string {
	var missing []string
	if endpoint == "" {
		missing = append(missing, "endpoint")
	}
	if user == "" {
		missing = append(missing, "user")
	}
	if token == "" {
		missing = append(missing, "token")
	}
	return missing
}

func missingConfigError(missing []string, configPath string) error {
	var sb strings.Builder
	sb.WriteString("missing Jira configuration fields: ")
	sb.WriteString(strings.Join(missing, ", "))
	sb.WriteString(". Set ")
	sb.WriteString(missingEndpointHint)
	sb.WriteString(", ")
	sb.WriteString(missingUserHint)
	sb.WriteString(", and ")
	sb.WriteString(missingAPITokenHint)
	sb.WriteString(".")
	sb.WriteString(" You can also set ")
	sb.WriteString(missingConfigKeysHint)
	sb.WriteString(" in remuda config.yaml.")
	if strings.TrimSpace(configPath) != "" {
		sb.WriteString(" Config path: ")
		sb.WriteString(configPath)
		sb.WriteString(".")
	}
	return errors.New(sb.String())
}
