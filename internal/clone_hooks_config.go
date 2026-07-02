package internal

import (
	"context"

	"os/exec"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/env"
)

// NewConfigCloneHook returns a CloneHook that runs a configured argv command.
// Commands run on the host with argv semantics (no implicit shell).
func NewConfigCloneHook(name string, argv []string) CloneHook {
	copiedArgv := append([]string(nil), argv...)
	return NewCloneHook(name, func(ctx CloneHookContext) error {
		return runConfigCloneHook(ctx, copiedArgv)
	})
}

func runConfigCloneHook(ctx CloneHookContext, argv []string) error {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return pkgerrors.Errorf("configured clone hook argv is empty")
	}

	//nolint:gosec // G204: clone hooks are explicit trusted config and intentionally executed.
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Dir = ctx.WorktreeDir
	cmd.Env = withInjectedCloneHookEnv(ctx)

	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return pkgerrors.Wrapf(err, "%s", strings.Join(argv, " "))
	}
	return pkgerrors.Wrapf(err, "%s: %s", strings.Join(argv, " "), msg)
}

func withInjectedCloneHookEnv(ctx CloneHookContext) []string {
	base := env.Environ(ctx.Env)
	injected := []string{
		"REMUDA_REPO_URL=" + ctx.RepoURL,
		"REMUDA_REPO_ORG=" + ctx.Org,
		"REMUDA_REPO_NAME=" + ctx.Repo,
		"REMUDA_REPO_SLUG=" + strings.ToLower(ctx.Org) + "/" + strings.ToLower(ctx.Repo),
		"REMUDA_CACHE_DIR=" + ctx.CacheDir,
		"REMUDA_WORKTREE_DIR=" + ctx.WorktreeDir,
	}
	return mergeEnvVars(base, injected)
}

func mergeEnvVars(base, overrides []string) []string {
	envMap := make(map[string]string, len(base)+len(overrides))
	order := make([]string, 0, len(base)+len(overrides))

	addPair := func(kv string) {
		key, value := splitEnvPair(kv)
		if key == "" {
			return
		}
		if _, seen := envMap[key]; !seen {
			order = append(order, key)
		}
		envMap[key] = value
	}

	for _, kv := range base {
		addPair(kv)
	}
	for _, kv := range overrides {
		addPair(kv)
	}

	out := make([]string, 0, len(order))
	for _, key := range order {
		out = append(out, key+"="+envMap[key])
	}
	return out
}

func splitEnvPair(kv string) (string, string) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return kv, ""
	}
	return kv[:idx], kv[idx+1:]
}
