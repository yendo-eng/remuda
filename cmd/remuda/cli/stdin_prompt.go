package cli

import (
	"io"
	"strings"

	"github.com/pkg/errors"
)

func resolvePromptFromStdin(in io.Reader, promptArg string) (string, bool, error) {
	if strings.TrimSpace(promptArg) != "-" {
		return promptArg, false, nil
	}
	if in == nil {
		return "", true, errors.New("cannot read prompt from STDIN: stdin is nil")
	}

	b, err := io.ReadAll(in)
	if err != nil {
		return "", true, errors.Wrap(err, "read prompt from STDIN")
	}

	// Trim trailing newlines (common when piping or using heredocs) while
	// preserving leading whitespace and internal formatting.
	prompt := strings.TrimRight(string(b), "\r\n")
	if strings.TrimSpace(prompt) == "" {
		return "", true, errors.New("prompt from STDIN is empty")
	}

	return prompt, true, nil
}
