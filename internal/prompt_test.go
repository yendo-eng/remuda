package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAssemblePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before []string
		prompt string
		after  []string
		want   string
	}{
		{
			name:   "before and after sections",
			before: []string{"saved", "reference"},
			prompt: "main",
			after:  []string{"saved later"},
			want:   "saved\nreference\nmain\nsaved later",
		},
		{
			name:   "main prompt only",
			prompt: "main",
			want:   "main",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, assemblePrompt(test.before, test.prompt, test.after))
		})
	}
}
