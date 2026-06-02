package cli

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yendo-eng/remuda/internal/prompts"
)

const promptListDescriptionMaxChars = 88

type PromptsCmd struct {
	List ListPromptsCmd `cmd:"" help:"List available prompts."`
	Show ShowPromptCmd  `cmd:"" help:"Show the content of a prompt."`
}

type ListPromptsCmd struct{}

func (c *ListPromptsCmd) Run(ctx Context) error {
	promptList, err := ctx.Remuda.ListPrompts()
	if err != nil {
		return err
	}

	builtinNames := map[string]struct{}{}
	for _, p := range promptList {
		if p.Builtin {
			builtinNames[p.Name] = struct{}{}
		}
	}

	resolvedByName := map[string]prompts.Prompt{}
	customOverridesBuiltin := map[string]bool{}
	for _, p := range promptList {
		if !p.Builtin {
			_, shadowsBuiltin := builtinNames[p.Name]
			customOverridesBuiltin[p.Name] = shadowsBuiltin
		}
		resolvedByName[p.Name] = p
	}

	builtins := make([]prompts.Prompt, 0, len(builtinNames))
	seenBuiltin := map[string]bool{}
	for _, p := range promptList {
		if !p.Builtin {
			continue
		}
		if seenBuiltin[p.Name] {
			continue
		}
		seenBuiltin[p.Name] = true
		resolved := resolvedByName[p.Name]
		if resolved.Builtin {
			builtins = append(builtins, resolved)
		}
	}

	custom := make([]prompts.Prompt, 0, len(resolvedByName))
	for _, p := range resolvedByName {
		if !p.Builtin {
			custom = append(custom, p)
		}
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })

	// Calculate the maximum name width for alignment
	maxNameWidth := 0
	for _, p := range builtins {
		if len(p.Name) > maxNameWidth {
			maxNameWidth = len(p.Name)
		}
	}
	for _, p := range custom {
		if len(p.Name) > maxNameWidth {
			maxNameWidth = len(p.Name)
		}
	}

	ctx.Remuda.IO.Outln("Built-in prompts:")
	for _, p := range builtins {
		content := listDescription(p.Description)
		ctx.Remuda.IO.Outf("  %-*s %s\n", maxNameWidth, p.Name, content)
	}

	if len(custom) > 0 {
		ctx.Remuda.IO.Outln("")
		ctx.Remuda.IO.Outln("Custom prompts:")
		for _, p := range custom {
			content := listDescription(p.Description)
			if customOverridesBuiltin[p.Name] {
				if content == "" {
					content = "overrides built-in"
				} else {
					content += " (overrides built-in)"
				}
			}
			ctx.Remuda.IO.Outf("  %-*s %s\n", maxNameWidth, p.Name, content)
		}
	}

	return nil
}

func listDescription(desc string) string {
	oneLine := strings.TrimSpace(strings.ReplaceAll(desc, "\n", " "))
	return truncateWithEllipsis(oneLine, promptListDescriptionMaxChars)
}

func truncateWithEllipsis(value string, maxChars int) string {
	if maxChars <= 0 || value == "" {
		return ""
	}
	if utf8.RuneCountInString(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		out := []rune(value)
		if maxChars > len(out) {
			maxChars = len(out)
		}
		return string(out[:maxChars])
	}

	trimmed := string([]rune(value)[:maxChars-3])
	trimmed = strings.TrimRight(trimmed, " ")
	if trimmed == "" {
		return "..."
	}
	return trimmed + "..."
}

type ShowPromptCmd struct {
	Name PromptName `kong:"arg,help:'Name of the prompt to show (custom overrides built-in on name collision).',predictor='prompt-name'"`
}

func (c *ShowPromptCmd) Run(ctx Context) error {
	content, err := ctx.Remuda.ShowPrompt(c.Name.String())
	if err != nil {
		return err
	}

	ctx.Remuda.IO.Outln(content)
	return nil
}
