package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/session"
	"github.com/yendo-eng/remuda/internal/util"
)

type DurationFlag struct {
	time.Duration
}

func (d *DurationFlag) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		return errors.New("duration is required")
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

// SessionReapCmd kills active sessions older than a threshold (safe with --dry-run).
type SessionReapCmd struct {
	OlderThan DurationFlag `name:"older-than" required:"" help:"Reap sessions older than this duration (Go-style, e.g. 72h, 336h)."`
	DryRun    bool         `default:"true" name:"dry-run" help:"Print what would be killed without deleting anything (default: true; set --dry-run=false to act)."`
	Cleanup   bool         `name:"cleanup" help:"Also remove the workspace and git worktree for killed sessions."`
	Pick      bool         `name:"pick" help:"Use fzf to pick sessions from the candidates."`
}

func (c *SessionReapCmd) Validate() error {
	if c.OlderThan.Duration <= 0 {
		return errors.New("--older-than must be positive")
	}
	return nil
}

func (c *SessionReapCmd) Run(ctx Context) error {
	if c.Pick && !ctx.Remuda.IO.IsTerminal() && !hasTTY() {
		return errors.New("--pick requires an interactive TTY")
	}

	candidates, skipped, err := ctx.Remuda.SessionReapCandidates(c.OlderThan.Duration, time.Now())
	if err != nil {
		return errors.Wrap(err, "list reap candidates")
	}

	if len(candidates) == 0 {
		writeReapSkipped(ctx, skipped)
		if len(skipped) == 0 {
			ctx.Remuda.IO.Outln("No sessions to reap.")
		}
		return nil
	}

	names := make([]string, 0, len(candidates))
	for _, sess := range candidates {
		names = append(names, sess.Name)
	}

	if c.Pick {
		selected, err := pickSessionNamesWithFZF(logging.FromContext(ctx.ctx), names, ctx.Remuda.Session, true)
		if err != nil {
			return errors.Wrap(err, "pick sessions")
		}
		if len(selected) == 0 {
			return errors.New("no sessions selected")
		}
		names = selected
	}

	results, err := ctx.Remuda.SessionReap(names, c.Cleanup, c.DryRun)
	if err != nil {
		return errors.Wrap(err, "session reap")
	}

	writeReapSkipped(ctx, skipped)

	for _, result := range results {
		if result.DryRun {
			ctx.Remuda.IO.Outf("would kill %s\n", result.Name)
		} else {
			ctx.Remuda.IO.Outf("killed %s\n", result.Name)
		}
		if c.Cleanup && result.WorkspacePathError != "" {
			ctx.Remuda.IO.Outf("warning: cleanup skipped for %s: %s\n", result.Name, result.WorkspacePathError)
		}
		if c.Cleanup && result.WorkspacePath != "" {
			if result.DryRun {
				ctx.Remuda.IO.Outf("would remove %s\n", result.WorkspacePath)
			} else {
				ctx.Remuda.IO.Outf("removed %s\n", result.WorkspacePath)
			}
		}
	}

	writeReapSummary(ctx, results)
	return nil
}

func writeReapSkipped(ctx Context, skipped []internal.ReapedSession) {
	for _, sess := range skipped {
		reason := normalizeReapSkipReason(sess.SkippedReason)
		ctx.Remuda.IO.Outf("skipped %s: %s\n", sess.Name, reason)
	}
}

func normalizeReapSkipReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "", "unknown session age":
		return "unknown age"
	default:
		return reason
	}
}

func writeReapSummary(ctx Context, results []internal.ReapedSession) {
	if len(results) == 0 {
		return
	}
	action := "killed"
	if results[0].DryRun {
		action = "would kill"
	}
	label := "session"
	if len(results) != 1 {
		label = "sessions"
	}
	ctx.Remuda.IO.Outf("%s %d %s\n", action, len(results), label)
}

func pickSessionNamesWithFZF(
	logger zerolog.Logger,
	candidates []string,
	mgr session.SessionManager,
	multi bool,
) ([]string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, fmt.Errorf("fzf not found in PATH; please install fzf or omit --pick")
	}

	var b bytes.Buffer
	for _, name := range candidates {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		fmt.Fprintln(&b, name)
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("no sessions available to pick")
	}

	args := []string{}
	if multi {
		args = append(args, "--multi")
	}
	if preview := session.FZFPreviewCommand(mgr); preview != "" {
		args = append(args, "--preview", preview)
		args = append(args, "--preview-window", "up:66%")
	}

	cmd := util.CmdWithLogger(logger, "fzf", args...)
	cmd.Stdin = &b

	tty, ttyErr := openTTY()
	if ttyErr == nil {
		defer func() {
			_ = tty.Close()
		}()
		cmd.Stderr = tty
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fzf selection error: %w", err)
	}

	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		selected = append(selected, line)
	}

	return selected, nil
}
