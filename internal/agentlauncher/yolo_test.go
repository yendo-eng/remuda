package agentlauncher_test

import (
	"testing"

	"github.com/yendo-eng/remuda/internal/agentlauncher"

	"github.com/stretchr/testify/assert"
)

// Simple unit tests for agent command composition around --yolo behavior.

func TestCodexLauncher_YoloAddsFlag(t *testing.T) {
	l := agentlauncher.Codex("", true, "")
	cmd := l.Command("do stuff")
	// Expect the dangerous bypass flag to be present for Codex when --yolo is set.
	assert.Contains(t, cmd, "--dangerously-bypass-approvals-and-sandbox", "missing yolo flag")
	// Expect env passthrough config to be present as well.
	assert.Contains(t, cmd, "shell_environment_policy.ignore_default_excludes=\"true\"", "missing env passthrough config")
	// And ensure --full-auto is not present when yolo is set.
	assert.NotContains(t, cmd, "--full-auto", "full-auto should not be set with yolo")
}

func TestCodexLauncher_NoYoloNoFlag(t *testing.T) {
	l := agentlauncher.Codex("", false, "")
	cmd := l.Command("do stuff")
	// Expect the dangerous bypass flag to be absent by default.
	assert.NotContains(t, cmd, "--dangerously-bypass-approvals-and-sandbox", "unexpected yolo flag")
	// Ensure unsupported full-auto is never emitted in default mode.
	assert.NotContains(t, cmd, "--full-auto", "full-auto should never be emitted")
}

func TestCodexLauncher_AgentDefaultOmitsModelFlag(t *testing.T) {
	l := agentlauncher.Codex(agentlauncher.ModelAgentDefault, false, "")
	cmd := l.Command("do stuff")
	assert.NotContains(t, cmd, "--model", "model flag should be omitted when agent-default is requested")
}
