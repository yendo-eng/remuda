package util

// A collection of weird shit that LLMs wrote to solve various bugs or implement
// functionality in a completely inscrutable way. Will try to chip away at this
// over time.

// SSHRewriteSnippet returns a small shell snippet that, when executed inside the
// repository working directory, rewrites an HTTPS GitHub origin URL to SSH if
// an SSH agent/socket is available. It is safe to run multiple times.
//
// - Only affects github.com HTTPS origins
// - Derives git@github.com:org/repo.git
func SSHRewriteSnippet() string {
	return `
set -euo pipefail
# If no SSH agent, skip quietly
if [ -z "${SSH_AUTH_SOCK:-}" ]; then
  :
else
  cur=$(git remote get-url origin 2>/dev/null || true)
  case "$cur" in
    https://github.com/*)
      # Extract org/repo (strip .git and trailing slashes)
      repo=$(printf "%s" "$cur" | sed -E 's#^https://github.com/([^/]+/[^/.]+)(\.git)?/*$#\1#')
      if [ -n "$repo" ]; then
        git remote set-url origin "git@github.com:${repo}.git" >/dev/null 2>&1 || true
      fi
      ;;
    *) ;;
  esac
fi
`
}
