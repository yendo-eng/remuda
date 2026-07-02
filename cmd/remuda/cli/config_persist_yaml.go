package cli

import (
	"bytes"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/configfile"
	"gopkg.in/yaml.v3"
)

func renderConfigV1(cfg *configfile.V1, original []byte) ([]byte, error) {
	if len(original) == 0 {
		return yaml.Marshal(cfg)
	}

	doc, err := parseConfigDocument(original)
	if err != nil {
		return nil, err
	}
	if err := applyRepoDefaultsToDocument(doc, cfg); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseConfigDocument(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, pkgerrors.Wrap(err, "parse config yaml")
	}
	if doc.Kind == 0 {
		return nil, pkgerrors.Errorf("parse config yaml: empty document")
	}
	return &doc, nil
}

func applyRepoDefaultsToDocument(doc *yaml.Node, cfg *configfile.V1) error {
	if cfg == nil || cfg.Repos == nil {
		return nil
	}

	alias := ""
	url := ""
	if cfg.Repos.DefaultRepo != nil {
		alias = *cfg.Repos.DefaultRepo
	}
	if cfg.Repos.DefaultRepoURL != nil {
		url = *cfg.Repos.DefaultRepoURL
	}
	if alias == "" && url == "" {
		return nil
	}

	root := documentRoot(doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return pkgerrors.Errorf("parse config yaml: expected mapping at document root")
	}

	repos := ensureMappingChild(root, "repos")
	if repos == nil {
		return pkgerrors.Errorf("parse config yaml: unable to create repos mapping")
	}

	if alias != "" {
		setDefaultRepoValue(repos, "default_repo", "default_repo_url", alias)
		return nil
	}

	setDefaultRepoValue(repos, "default_repo_url", "default_repo", url)
	return nil
}

func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	if doc.Kind == yaml.MappingNode {
		return doc
	}
	return nil
}

func ensureMappingChild(parent *yaml.Node, key string) *yaml.Node {
	if parent == nil || parent.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			if parent.Content[i+1].Kind != yaml.MappingNode {
				parent.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			return parent.Content[i+1]
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valueNode)
	return valueNode
}

func setDefaultRepoValue(repos *yaml.Node, keepKey string, dropKey string, value string) {
	keepIdx := findMappingKey(repos, keepKey)
	dropIdx := findMappingKey(repos, dropKey)

	if keepIdx == -1 && dropIdx != -1 {
		repos.Content[dropIdx].Value = keepKey
		keepIdx = dropIdx
		dropIdx = -1
	}

	if keepIdx == -1 {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: keepKey}
		valueNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
		repos.Content = append(repos.Content, keyNode, valueNode)
	} else {
		setScalarValue(repos.Content[keepIdx+1], value)
	}

	if dropIdx != -1 {
		removeMappingPair(repos, dropIdx)
	}
}

func findMappingKey(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func removeMappingPair(node *yaml.Node, index int) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	if index < 0 || index+1 >= len(node.Content) {
		return
	}
	node.Content = append(node.Content[:index], node.Content[index+2:]...)
}

func setScalarValue(node *yaml.Node, value string) {
	if node == nil {
		return
	}
	if node.Kind != yaml.ScalarNode {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!str"
		node.Content = nil
	}
	node.Value = value
}
