package cli

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/yendo-eng/remuda/internal/github"
)

type RepoListCmd struct {
	JSON bool `name:"json" help:"Emit JSON instead of plain text."`
}

func (c *RepoListCmd) Run(ctx Context) error {
	groups := repoAliasGroups()

	if c.JSON {
		enc := json.NewEncoder(ctx.Remuda.IO.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(groups)
	}
	if len(groups) == 0 {
		ctx.Remuda.IO.Outln("No repository aliases configured.")
		return nil
	}

	maxWidth := 0
	for _, g := range groups {
		if len(g.Primary) > maxWidth {
			maxWidth = len(g.Primary)
		}
	}

	for _, g := range groups {
		ctx.Remuda.IO.Outf("%-*s %s", maxWidth, g.Primary, g.URL)
		if len(g.Aliases) > 0 {
			ctx.Remuda.IO.Outf(" (aliases: %s)", strings.Join(g.Aliases, ", "))
		}
		ctx.Remuda.IO.Outln()
	}

	return nil
}

type repoAliasGroup struct {
	Primary string   `json:"primary"`
	URL     string   `json:"url"`
	Aliases []string `json:"aliases,omitempty"`
}

func repoAliasGroups() []repoAliasGroup {
	aliases := github.RepoAliases()
	byURL := make(map[string][]string, len(aliases))
	for key, url := range aliases {
		byURL[url] = append(byURL[url], key)
	}

	groups := make([]repoAliasGroup, 0, len(byURL))
	for _, primary := range canonicalAliasKeys() {
		url, ok := aliases[primary]
		if !ok {
			continue
		}
		others := make([]string, 0, len(byURL[url]))
		for _, alias := range byURL[url] {
			if alias == primary {
				continue
			}
			others = append(others, alias)
		}
		sort.Strings(others)
		groups = append(groups, repoAliasGroup{
			Primary: primary,
			URL:     url,
			Aliases: others,
		})
	}

	return groups
}
