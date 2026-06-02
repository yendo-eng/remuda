package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZellijManagerStart_UnsupportedCreateBackground(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script stub")
	}

	tmp := t.TempDir()
	writeStubZellij(t, tmp, `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "attach" ]]; then
  shift
  echo "error: Found argument '--create-background' which wasn't expected, or isn't valid in this context" >&2
  exit 2
fi
echo "unexpected args: $*" >&2
exit 1
`)

	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))

	mgr := NewZellijManager()
	err := mgr.Start("org/repo/feature", "echo hi")
	require.Error(t, err)
	require.Contains(t, err.Error(), "create-background")
	require.Contains(t, err.Error(), "upgrade")
}

func TestZellijManagerStart_RetriesUntilSessionReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script stub")
	}

	tmp := t.TempDir()
	writeStubZellij(t, tmp, `#!/usr/bin/env bash
set -euo pipefail
state_dir="${ZELLIJ_STUB_STATE_DIR:?}"

if [[ "${1:-}" == "attach" && "${2:-}" == "--create-background" ]]; then
  session="${3:-}"
  echo "${session}" > "${state_dir}/session"
  exit 0
fi

if [[ "${1:-}" == "--session" && "${3:-}" == "action" && "${4:-}" == "write-chars" ]]; then
  session="${2:-}"
  payload="${5:-}"
  expected="$(cat "${state_dir}/session" 2>/dev/null || true)"
  if [[ "${session}" != "${expected}" ]]; then
    echo "No active sessions found" >&2
    exit 1
  fi

  count_file="${state_dir}/write_attempts"
  count=0
  if [[ -f "${count_file}" ]]; then
    count="$(cat "${count_file}")"
  fi
  count="$((count+1))"
  echo "${count}" > "${count_file}"

  if (( count < 3 )); then
    echo "No active sessions found" >&2
    exit 1
  fi

  printf "%s" "${payload}" > "${state_dir}/payload"
  exit 0
fi

echo "unexpected args: $*" >&2
exit 1
`)

	t.Setenv("PATH", tmp+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ZELLIJ_STUB_STATE_DIR", tmp)

	mgr := NewZellijManager()
	err := mgr.Start("org/repo/feature", "echo hi")
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(tmp, "payload"))
	require.NoError(t, err)
	require.Equal(t, "echo hi\n", string(got))
}

func writeStubZellij(t *testing.T, dir string, contents string) {
	t.Helper()

	path := filepath.Join(dir, "zellij")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o755))
}
