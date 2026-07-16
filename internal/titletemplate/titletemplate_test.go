package titletemplate_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/titletemplate"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{name: "empty is valid", template: ""},
		{name: "default template", template: titletemplate.Default},
		{name: "reordered literal", template: "{repo}/{name}"},
		{name: "off case-insensitive", template: "OFF"},
		{name: "off with surrounding space", template: "  off  "},
		{name: "unknown placeholder", template: "{branch}", wantErr: true},
		{name: "mixed known and unknown", template: "{org}/{branch}", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := titletemplate.Validate(tt.template)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		template    string
		sessionName string
		wantTitle   string
		wantOK      bool
	}{
		{
			name:        "unset template matches current behavior",
			template:    "",
			sessionName: "acme/example-repo/feat",
			wantTitle:   "acme/example-repo/feat",
			wantOK:      true,
		},
		{
			name:        "custom template with reordering",
			template:    "{repo}/{name}",
			sessionName: "acme/example-repo/feat",
			wantTitle:   "example-repo/feat",
			wantOK:      true,
		},
		{
			name:        "literal text",
			template:    "⧉ {name}",
			sessionName: "acme/example-repo/feat",
			wantTitle:   "⧉ feat",
			wantOK:      true,
		},
		{
			name:        "off disables title",
			template:    "off",
			sessionName: "acme/example-repo/feat",
			wantOK:      false,
		},
		{
			name:        "off case-insensitive",
			template:    "Off",
			sessionName: "acme/example-repo/feat",
			wantOK:      false,
		},
		{
			name:        "non-remuda session falls back to raw name",
			template:    "{repo}/{name}",
			sessionName: "not-a-remuda-session",
			wantTitle:   "not-a-remuda-session",
			wantOK:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			title, ok := titletemplate.Render(tt.template, tt.sessionName)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.wantTitle, title)
			}
		})
	}
}
