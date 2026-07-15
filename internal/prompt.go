package internal

import "strings"

func assemblePrompt(before []string, prompt string, after []string) string {
	var fullPrompt strings.Builder
	for _, p := range before {
		fullPrompt.WriteString(p)
		fullPrompt.WriteString("\n")
	}
	fullPrompt.WriteString(prompt)
	for _, p := range after {
		fullPrompt.WriteString("\n")
		fullPrompt.WriteString(p)
	}
	return fullPrompt.String()
}
