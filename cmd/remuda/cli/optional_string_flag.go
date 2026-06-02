package cli

import (
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

// OptionalStringFlag parses --flag (optionally: --flag=VALUE).
//
// Kong does not support optional values for string flags. This models the flag
// as a boolean and, if an attached "=value" token is present, consumes it.
//
// If VALUE looks like a bool (eg. "true"/"false"), it is treated as a boolean
// value for the flag (and clears Value when false).
type OptionalStringFlag struct {
	set     bool
	enabled bool
	value   string
}

func (f *OptionalStringFlag) Decode(ctx *kong.DecodeContext) error {
	f.set = true
	f.enabled = true

	if ctx.Scan.Peek().InferredType() != kong.FlagValueToken {
		return nil
	}

	var value string
	if err := ctx.Scan.PopValueInto("value", &value); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if b, err := strconv.ParseBool(trimmed); err == nil {
		f.enabled = b
		if !b {
			f.value = ""
		}
		return nil
	}

	f.value = value
	return nil
}

func (f *OptionalStringFlag) IsBool() bool { return true }

func (f OptionalStringFlag) Enabled() bool { return f.set && f.enabled }

func (f OptionalStringFlag) Value() string { return f.value }
