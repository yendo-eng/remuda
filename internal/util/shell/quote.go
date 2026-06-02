package shell

import "strings"

// EscapeSingleQuotes escapes single quotes for safe inclusion in a single-quoted shell string.
func EscapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

// SingleQuote wraps a value in single quotes, escaping embedded single quotes.
func SingleQuote(value string) string {
	return "'" + EscapeSingleQuotes(value) + "'"
}
