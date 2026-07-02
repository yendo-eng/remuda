package configfile

import (
	"bytes"
	"fmt"
	"net/url"
	pathpkg "path"
	"slices"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/enums"
	"github.com/yendo-eng/remuda/internal/util"
	"gopkg.in/yaml.v3"
)

const Version1 = 1

type V1 struct {
	Version int `yaml:"version"`

	Repos      *ReposV1              `yaml:"repos,omitempty"`
	Session    *SessionV1            `yaml:"session,omitempty"`
	Workspaces *WorkspacesV1         `yaml:"workspaces,omitempty"`
	Jira       *JiraV1               `yaml:"jira,omitempty"`
	Defaults   *DefaultsV1           `yaml:"defaults,omitempty"`
	Profiles   map[string]DefaultsV1 `yaml:"profiles,omitempty"`
	PerRepo    map[string]OverlayV1  `yaml:"per_repo,omitempty"`
}

type ReposV1 struct {
	BaseDir        *string           `yaml:"base_dir,omitempty"`
	DefaultRepo    *string           `yaml:"default_repo,omitempty"`
	DefaultRepoURL *string           `yaml:"default_repo_url,omitempty"`
	Aliases        map[string]string `yaml:"aliases,omitempty"`
}

type SessionV1 struct {
	Manager *string         `yaml:"manager,omitempty"`
	Prune   *SessionPruneV1 `yaml:"prune,omitempty"`
}

type SessionPruneV1 struct {
	Ignore *[]string `yaml:"ignore,omitempty"`
}

type WorkspacesV1 struct {
	Ignore *[]string `yaml:"ignore,omitempty"`
}

type JiraV1 struct {
	Endpoint *string `yaml:"endpoint,omitempty"`
	User     *string `yaml:"user,omitempty"`
	APIToken *string `yaml:"api_token,omitempty"`
}

type DefaultsV1 struct {
	Agent                 *string             `yaml:"agent,omitempty"`
	Model                 *string             `yaml:"model,omitempty"`
	ReasoningLevel        *string             `yaml:"reasoning_level,omitempty"`
	SlugifyReasoningLevel *string             `yaml:"slugify_reasoning_level,omitempty"`
	AgentCmd              *string             `yaml:"agent_cmd,omitempty"`
	AgentArgs             map[string][]string `yaml:"agent_args,omitempty"`
	UsePrompts            *[]string           `yaml:"use_prompts,omitempty"`
	NoUse                 *[]string           `yaml:"no_use,omitempty"`
	Experiments           *[]string           `yaml:"experiments,omitempty"`
	Yolo                  *bool               `yaml:"yolo,omitempty"`
	Merge                 *MergeV1            `yaml:"merge,omitempty"`
	Container             *ContainerV1        `yaml:"container,omitempty"`
}

type MergeV1 struct {
	GHFlags *[]string `yaml:"gh_flags,omitempty"`
}

type ContainerV1 struct {
	Enabled    *bool     `yaml:"enabled,omitempty"`
	Image      *string   `yaml:"image,omitempty"`
	Opts       *[]string `yaml:"opts,omitempty"`
	InheritEnv *[]string `yaml:"inherit_env,omitempty"`
}

func (c *ContainerV1) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind == yaml.AliasNode && value.Alias != nil {
		value = value.Alias
	}

	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			*c = ContainerV1{}
			return nil
		}
		var enabled bool
		if err := value.Decode(&enabled); err != nil {
			return pkgerrors.Errorf("container: expected mapping or boolean")
		}
		*c = ContainerV1{Enabled: &enabled}
		return nil
	case yaml.MappingNode:
		if err := validateContainerMappingKeys(value); err != nil {
			return err
		}
		type plain ContainerV1
		var out plain
		if err := value.Decode(&out); err != nil {
			return err
		}
		*c = ContainerV1(out)
		return nil
	default:
		return pkgerrors.Errorf("container: expected mapping or boolean")
	}
}

func validateContainerMappingKeys(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return pkgerrors.Errorf("container: expected mapping or boolean")
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		switch key {
		case "enabled", "image", "opts", "inherit_env":
			continue
		default:
			return pkgerrors.Errorf("container.%s: unknown field", key)
		}
	}
	return nil
}

// OverlayV1 is the per_repo overlay applied when a repo slug can be inferred.
// All fields are optional to support field-wise merges.
type OverlayV1 struct {
	Repos      *ReposV1      `yaml:"repos,omitempty"`
	Session    *SessionV1    `yaml:"session,omitempty"`
	Defaults   *DefaultsV1   `yaml:"defaults,omitempty"`
	Profile    *string       `yaml:"profile,omitempty"`
	CloneHooks []CloneHookV1 `yaml:"clone_hooks,omitempty"`
}

type CloneHookV1 struct {
	Name string   `yaml:"name,omitempty"`
	Argv []string `yaml:"argv,omitempty"`
}

func ParseV1(yamlBytes []byte) (*V1, error) {
	type header struct {
		Version int `yaml:"version"`
	}

	var h header
	if err := yaml.Unmarshal(yamlBytes, &h); err != nil {
		return nil, pkgerrors.Wrap(err, "parse config yaml")
	}

	if h.Version == 0 {
		return nil, pkgerrors.Errorf("config version is required (expected %d)", Version1)
	}
	if h.Version != Version1 {
		return nil, pkgerrors.Errorf("unsupported config version %d (expected %d)", h.Version, Version1)
	}

	// Use strict YAML decoding to reject unknown keys.
	var cfg V1
	dec := yaml.NewDecoder(bytes.NewReader(yamlBytes))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, pkgerrors.Wrap(err, "parse config v1")
	}
	if err := cfg.normalizePerRepoKeys(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *V1) normalizePerRepoKeys() error {
	if c == nil || len(c.PerRepo) == 0 {
		return nil
	}

	normalized := make(map[string]OverlayV1, len(c.PerRepo))
	seen := map[string]string{}
	for slug, overlay := range c.PerRepo {
		canon := strings.ToLower(strings.TrimSpace(slug))
		if prev, ok := seen[canon]; ok {
			return pkgerrors.Errorf("per_repo key %q duplicates %q (case-insensitive)", slug, prev)
		}
		seen[canon] = slug
		normalized[canon] = overlay
	}

	c.PerRepo = normalized
	return nil
}

func (c V1) Validate() error {
	if c.Version != Version1 {
		return pkgerrors.Errorf("unsupported config version %d (expected %d)", c.Version, Version1)
	}

	// Validate repos.aliases if present.
	if c.Repos != nil {
		if err := c.Repos.validate("repos"); err != nil {
			return err
		}
	}

	// Validate top-level enum fields.
	if c.Session != nil {
		if err := c.Session.validate("session"); err != nil {
			return err
		}
	}
	if c.Workspaces != nil {
		if err := c.Workspaces.validate("workspaces"); err != nil {
			return err
		}
	}
	if c.Jira != nil {
		if err := c.Jira.validate("jira"); err != nil {
			return err
		}
	}
	if c.Defaults != nil {
		if err := c.Defaults.validate("defaults"); err != nil {
			return err
		}
	}

	// Profile names are case-sensitive; no normalization is applied.
	for name, defaults := range c.Profiles {
		if err := validateProfileName(name); err != nil {
			return pkgerrors.Wrapf(err, "profiles[%q]", name)
		}
		if err := defaults.validate(fmt.Sprintf("profiles[%q]", name)); err != nil {
			return err
		}
	}

	// Validate per_repo keys and their overlays.
	for slug, overlay := range c.PerRepo {
		if err := validateRepoSlug(slug); err != nil {
			return err
		}
		prefix := fmt.Sprintf("per_repo[%q]", slug)
		if overlay.Repos != nil {
			if err := overlay.Repos.validate(prefix + ".repos"); err != nil {
				return err
			}
		}
		if overlay.Session != nil {
			if overlay.Session.Prune != nil {
				return pkgerrors.Errorf("%s.session.prune: not supported (session prune ignores are global-only)", prefix)
			}
			if err := overlay.Session.validate(prefix + ".session"); err != nil {
				return err
			}
		}
		if overlay.Defaults != nil {
			if err := overlay.Defaults.validate(prefix + ".defaults"); err != nil {
				return err
			}
		}
		if overlay.Profile != nil {
			if err := validateProfileName(strings.TrimSpace(*overlay.Profile)); err != nil {
				return pkgerrors.Wrapf(err, "%s.profile", prefix)
			}
		}
		if err := validateCloneHooks(prefix, overlay.CloneHooks); err != nil {
			return err
		}
	}

	return nil
}

func (s SessionV1) validate(path string) error {
	if s.Manager != nil {
		if !slices.Contains(enums.ValidSessionManagers, *s.Manager) {
			return pkgerrors.Errorf("%s.manager: invalid value %q (valid: %s)",
				path, *s.Manager, strings.Join(enums.ValidSessionManagers, ", "))
		}
	}
	if s.Prune != nil {
		if err := s.Prune.validate(path + ".prune"); err != nil {
			return err
		}
	}
	return nil
}

func (s SessionPruneV1) validate(path string) error {
	if s.Ignore == nil {
		return nil
	}
	if err := validateIgnorePatternList(path+".ignore", *s.Ignore); err != nil {
		return err
	}
	return nil
}

func (w WorkspacesV1) validate(path string) error {
	if w.Ignore == nil {
		return nil
	}
	if err := validateIgnorePatternList(path+".ignore", *w.Ignore); err != nil {
		return err
	}
	return nil
}

func validateIgnorePatternList(path string, patterns []string) error {
	for i, pattern := range patterns {
		entryPath := fmt.Sprintf("%s[%d]", path, i)
		if strings.TrimSpace(pattern) == "" {
			return pkgerrors.Errorf("%s: ignore pattern cannot be empty", entryPath)
		}
		if _, err := pathpkg.Match(pattern, "workspace"); err != nil {
			return pkgerrors.Wrapf(err, "%s: invalid pattern %q", entryPath, pattern)
		}
	}
	return nil
}

func (j JiraV1) validate(path string) error {
	if j.Endpoint != nil {
		trimmed := strings.TrimSpace(*j.Endpoint)
		if trimmed == "" {
			return pkgerrors.Errorf("%s.endpoint: cannot be empty", path)
		}
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return pkgerrors.Wrapf(err, "%s.endpoint: invalid URL %q", path, *j.Endpoint)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return pkgerrors.Errorf("%s.endpoint: must include scheme and host", path)
		}
	}
	if j.User != nil && strings.TrimSpace(*j.User) == "" {
		return pkgerrors.Errorf("%s.user: cannot be empty", path)
	}
	return nil
}

func (r ReposV1) validate(path string) error {
	for alias, url := range r.Aliases {
		aliasPath := fmt.Sprintf("%s.aliases[%q]", path, alias)
		trimmedURL := strings.TrimSpace(url)
		if trimmedURL == "" {
			return pkgerrors.Errorf("%s: alias URL cannot be empty", aliasPath)
		}
		if strings.HasPrefix(trimmedURL, "-") {
			return pkgerrors.Errorf("%s: alias URL %q is invalid (cannot start with '-')", aliasPath, url)
		}
	}
	return nil
}

func (d DefaultsV1) validate(path string) error {
	if d.Agent != nil {
		if !slices.Contains(enums.ValidAgents, *d.Agent) {
			return pkgerrors.Errorf("%s.agent: invalid value %q (valid: %s)",
				path, *d.Agent, strings.Join(enums.ValidAgents, ", "))
		}
	}
	if d.SlugifyReasoningLevel != nil {
		if !slices.Contains(enums.ValidSlugifyReasoningLevels, *d.SlugifyReasoningLevel) {
			return pkgerrors.Errorf("%s.slugify_reasoning_level: invalid value %q (valid: %s)",
				path, *d.SlugifyReasoningLevel, strings.Join(enums.ValidSlugifyReasoningLevels, ", "))
		}
	}
	for agent, args := range d.AgentArgs {
		agentPath := fmt.Sprintf("%s.agent_args[%q]", path, agent)
		if !slices.Contains(enums.ValidAgents, agent) {
			return pkgerrors.Errorf("%s: invalid value %q (valid: %s)", agentPath, agent, strings.Join(enums.ValidAgents, ", "))
		}
		for i, arg := range args {
			argPath := fmt.Sprintf("%s[%d]", agentPath, i)
			if strings.TrimSpace(arg) == "" {
				return pkgerrors.Errorf("%s: agent arg cannot be empty", argPath)
			}
		}
	}
	if d.Experiments != nil {
		for i, exp := range *d.Experiments {
			expPath := fmt.Sprintf("%s.experiments[%d]", path, i)
			if strings.TrimSpace(exp) == "" {
				return pkgerrors.Errorf("%s: experiment name cannot be empty", expPath)
			}
		}
	}
	if d.Merge != nil {
		if err := d.Merge.validate(path + ".merge"); err != nil {
			return err
		}
	}
	if d.Container != nil {
		if err := d.Container.validate(path + ".container"); err != nil {
			return err
		}
	}
	return nil
}

func (m MergeV1) validate(path string) error {
	if m.GHFlags == nil {
		return nil
	}
	for i, flag := range *m.GHFlags {
		flagPath := fmt.Sprintf("%s.gh_flags[%d]", path, i)
		if strings.TrimSpace(flag) == "" {
			return pkgerrors.Errorf("%s: merge flag cannot be empty", flagPath)
		}
	}
	return nil
}

func (c ContainerV1) validate(path string) error {
	if c.InheritEnv == nil {
		return nil
	}
	for i, env := range *c.InheritEnv {
		envPath := fmt.Sprintf("%s.inherit_env[%d]", path, i)
		if err := validateEnvVarName(envPath, env); err != nil {
			return err
		}
	}
	return nil
}

func validateCloneHooks(path string, hooks []CloneHookV1) error {
	for i, hook := range hooks {
		hookPath := fmt.Sprintf("%s.clone_hooks[%d]", path, i)
		if len(hook.Argv) == 0 {
			return pkgerrors.Errorf("%s.argv: cannot be empty", hookPath)
		}
		if strings.TrimSpace(hook.Argv[0]) == "" {
			return pkgerrors.Errorf("%s.argv[0]: executable cannot be empty", hookPath)
		}
	}
	return nil
}

func validateRepoSlug(slug string) error {
	if strings.TrimSpace(slug) == "" {
		return pkgerrors.Errorf("per_repo key must be a non-empty repo slug (expected owner/repo)")
	}
	if strings.Count(slug, "/") != 1 {
		return pkgerrors.Errorf("per_repo key %q is not a valid repo slug (expected owner/repo)", slug)
	}
	parts := strings.SplitN(slug, "/", 2)
	if parts[0] == "" || parts[1] == "" {
		return pkgerrors.Errorf("per_repo key %q is not a valid repo slug (expected owner/repo)", slug)
	}
	return nil
}

func validateProfileName(name string) error {
	if strings.TrimSpace(name) == "" {
		return pkgerrors.Errorf("profile name must be non-empty")
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return pkgerrors.Errorf("profile name %q cannot start or end with '/'", name)
	}
	if strings.Contains(name, "//") {
		return pkgerrors.Errorf("profile name %q cannot contain empty segments", name)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_',
			r == '-',
			r == '/':
			continue
		default:
			return pkgerrors.Errorf("profile name %q contains invalid character %q (allowed: ASCII letters, digits, '_', '-', '/')", name, r)
		}
	}
	return nil
}

func validateEnvVarName(path string, name string) error {
	if err := util.ValidateEnvVarName(name); err != nil {
		return pkgerrors.Wrapf(err, "%s", path)
	}
	return nil
}
