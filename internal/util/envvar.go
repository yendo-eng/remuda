package util

import (
	"errors"
	"fmt"
	"strings"
)

func ValidateEnvVarName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("env var name cannot be empty")
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		isLetter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
		isDigit := ch >= '0' && ch <= '9'
		isUnderscore := ch == '_'
		if i == 0 {
			if !isLetter && !isUnderscore {
				return fmt.Errorf("invalid env var name %q", name)
			}
			continue
		}
		if !isLetter && !isDigit && !isUnderscore {
			return fmt.Errorf("invalid env var name %q", name)
		}
	}
	return nil
}

func IsValidEnvVarName(name string) bool {
	return ValidateEnvVarName(name) == nil
}
