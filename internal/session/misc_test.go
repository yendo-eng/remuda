package session_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestSessionNameFromWorkspaceName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/home/tester/code/repos/acme/widgets/feature_task-1", "acme/widgets/feature_task-1"},
		{"repos/acme/widgets/feature_2", "acme/widgets/feature_2"},
		{"acme/widgets/feature_3", "acme/widgets/feature_3"},
		{"solo", "solo"},
	}
	for _, c := range cases {
		got := session.SessionNameFromWorkspaceName(c.in)
		require.Equal(t, c.want, got, "deriveSessionName(%q) = %q; want %q", c.in, got, c.want)
	}
}

func TestWithoutOrgPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"yendo-eng/death-star/ds-16x_5_1", "death-star/ds-16x_5_1"},
		{"death-star/ds-16x_5_1", "ds-16x_5_1"},
		{"solo", "solo"},
		{"", ""},
	}

	for _, c := range cases {
		got := session.WithoutOrgPrefix(c.in)
		require.Equal(t, c.want, got, "WithoutOrgPrefix(%q) = %q; want %q", c.in, got, c.want)
	}
}
