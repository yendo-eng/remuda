package internal

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
)

// CloneHookContext provides context about a freshly created worktree to hooks.
type CloneHookContext struct {
	RepoURL     string
	Org         string
	Repo        string
	CacheDir    string
	WorktreeDir string
	Env         env.Provider
	Logger      *zerolog.Logger
}

// CloneHook represents a post-clone hook that can run against certain repos.
type CloneHook interface {
	Name() string
	Run(ctx CloneHookContext) error
}

// cloneHookFunc adapts a function to the CloneHook interface.
type cloneHookFunc struct {
	name string
	fn   func(CloneHookContext) error
}

func (f cloneHookFunc) Name() string { return f.name }

func (f cloneHookFunc) Run(ctx CloneHookContext) error { return f.fn(ctx) }

// NewCloneHook returns a CloneHook backed by the provided function.
func NewCloneHook(name string, fn func(CloneHookContext) error) CloneHook {
	return cloneHookFunc{name: name, fn: fn}
}

// CloneHookRegistry stores hooks keyed by org/repo.
type CloneHookRegistry struct {
	mu          sync.RWMutex
	hooks       map[string][]CloneHook
	configHooks map[string][]CloneHook
}

func NewCloneHookRegistry() *CloneHookRegistry {
	return &CloneHookRegistry{
		hooks:       make(map[string][]CloneHook),
		configHooks: make(map[string][]CloneHook),
	}
}

func (r *CloneHookRegistry) Register(org, repo string, hooks ...CloneHook) {
	if len(hooks) == 0 {
		return
	}
	key := registryKey(org, repo)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[key] = append(r.hooks[key], hooks...)
}

func (r *CloneHookRegistry) get(org, repo string) []CloneHook {
	key := registryKey(org, repo)
	r.mu.RLock()
	defer r.mu.RUnlock()
	hooks := r.hooks[key]
	configHooks := r.configHooks[key]
	if len(hooks) == 0 && len(configHooks) == 0 {
		return nil
	}
	out := make([]CloneHook, 0, len(hooks)+len(configHooks))
	out = append(out, hooks...)
	out = append(out, configHooks...)
	return out
}

// SetConfigHooks replaces config-defined hooks for all repos.
// Built-in and programmatic hooks registered via Register() are preserved.
func (r *CloneHookRegistry) SetConfigHooks(hooksByRepo map[string][]CloneHook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configHooks = make(map[string][]CloneHook, len(hooksByRepo))
	for slug, hooks := range hooksByRepo {
		key := strings.ToLower(strings.TrimSpace(slug))
		if key == "" || len(hooks) == 0 {
			continue
		}
		copied := make([]CloneHook, len(hooks))
		copy(copied, hooks)
		r.configHooks[key] = copied
	}
}

func registryKey(org, repo string) string {
	return strings.ToLower(org) + "/" + strings.ToLower(repo)
}

func (r *CloneHookRegistry) RunCloneHooks(ctx CloneHookContext) error {
	hooks := r.get(ctx.Org, ctx.Repo)
	if len(hooks) == 0 {
		return nil
	}

	logger := logging.DefaultLogger()
	if ctx.Logger != nil {
		logger = *ctx.Logger
	}
	repoSlug := fmt.Sprintf("%s/%s", ctx.Org, ctx.Repo)
	for _, hook := range hooks {
		logger.Debug().
			Str("repo", repoSlug).
			Str("hook", hook.Name()).
			Str("worktree", ctx.WorktreeDir).
			Msg("running clone hook")

		if err := hook.Run(ctx); err != nil {
			return fmt.Errorf("hook %s failed: %w", hook.Name(), err)
		}
	}
	return nil
}
