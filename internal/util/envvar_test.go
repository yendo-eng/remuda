package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEnvVarName(t *testing.T) {
	valid := []string{"FOO", "_BAR", "FOO1"}
	for _, name := range valid {
		require.NoError(t, ValidateEnvVarName(name), "expected %q to be valid", name)
		require.True(t, IsValidEnvVarName(name), "expected %q to be valid", name)
	}

	invalid := []string{"1FOO", "FOO-BAR", ""}
	for _, name := range invalid {
		require.Error(t, ValidateEnvVarName(name), "expected %q to be invalid", name)
		require.False(t, IsValidEnvVarName(name), "expected %q to be invalid", name)
	}
}
