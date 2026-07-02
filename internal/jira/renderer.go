package jira

import (
	"encoding/json"

	stdhtml "html"
	"strconv"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"golang.org/x/net/html"
)

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

// RenderADFToText renders a subset of Jira ADF JSON into readable plain text.
func RenderADFToText(raw json.RawMessage) (string, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "", nil
	}

	var root adfNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", pkgerrors.Wrap(err, "parse ADF")
	}

	text := strings.TrimSpace(renderADFBlock(root))
	return text, nil
}

// RenderADFToTextWithHTMLFallback renders ADF and falls back to rendered HTML
// when ADF parsing fails or when non-empty ADF content renders as empty.
func RenderADFToTextWithHTMLFallback(raw json.RawMessage, renderedHTML string) string {
	rendered, err := RenderADFToText(raw)
	if err == nil {
		if rendered != "" {
			htmlRendered := extractHTMLText(renderedHTML)
			if shouldPreferHTMLCodeBlocks(rendered, htmlRendered) {
				return htmlRendered
			}
			return rendered
		}
		if !hasADFContent(raw) {
			return ""
		}
	}

	return extractHTMLText(renderedHTML)
}

func shouldPreferHTMLCodeBlocks(adfRendered, htmlRendered string) bool {
	return strings.Contains(htmlRendered, "```") && !strings.Contains(adfRendered, "```")
}

func renderADFBlock(node adfNode) string {
	switch node.Type {
	case "doc":
		return renderBlockChildren(node.Content)
	case "paragraph":
		return strings.TrimSpace(renderADFInline(node.Content))
	case "heading":
		text := strings.TrimSpace(renderADFInline(node.Content))
		if text == "" {
			return ""
		}
		return "# " + text
	case "bulletList":
		return renderList(node.Content, false)
	case "orderedList":
		return renderList(node.Content, true)
	case "listItem":
		return renderListItem(node)
	case "expand", "panel", "blockquote", "table", "tableRow", "tableCell":
		return renderBlockChildren(node.Content)
	case "codeBlock":
		return renderCodeBlock(node)
	case "text":
		return node.Text
	case "hardBreak":
		return "\n"
	default:
		return ""
	}
}

func renderBlockChildren(nodes []adfNode) string {
	blocks := make([]string, 0, len(nodes))
	for _, node := range nodes {
		block := strings.TrimSpace(renderADFBlock(node))
		if block == "" {
			continue
		}
		blocks = append(blocks, block)
	}
	return strings.Join(blocks, "\n\n")
}

func renderADFInline(nodes []adfNode) string {
	var sb strings.Builder
	for _, node := range nodes {
		switch node.Type {
		case "text":
			sb.WriteString(node.Text)
		case "hardBreak":
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func renderList(nodes []adfNode, ordered bool) string {
	lines := make([]string, 0, len(nodes))
	index := 1
	for _, node := range nodes {
		if node.Type != "listItem" {
			continue
		}

		itemText := strings.TrimSpace(renderListItem(node))
		if itemText == "" {
			continue
		}

		prefix := "- "
		if ordered {
			prefix = strconv.Itoa(index) + ". "
		}
		lines = append(lines, prefixMultiline(prefix, itemText))
		index++
	}
	return strings.Join(lines, "\n")
}

func renderListItem(node adfNode) string {
	parts := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		part := strings.TrimSpace(renderADFBlock(child))
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n")
}

func renderCodeBlock(node adfNode) string {
	text := strings.TrimRight(renderADFInline(node.Content), "\n")
	if text == "" {
		return ""
	}
	return "```\n" + text + "\n```"
}

func prefixMultiline(prefix, text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return prefix
	}

	if len(lines) == 1 {
		return prefix + lines[0]
	}

	pad := strings.Repeat(" ", len(prefix))
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(lines[0])
	for _, line := range lines[1:] {
		sb.WriteString("\n")
		if line != "" {
			sb.WriteString(pad)
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func hasADFContent(raw json.RawMessage) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}

	var root adfNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return true
	}
	return adfNodeHasContent(root)
}

func adfNodeHasContent(node adfNode) bool {
	if strings.TrimSpace(node.Text) != "" {
		return true
	}
	if node.Type == "hardBreak" {
		return true
	}

	for _, child := range node.Content {
		if adfNodeHasContent(child) {
			return true
		}
	}
	return false
}

func extractHTMLText(in string) string {
	if strings.TrimSpace(in) == "" {
		return ""
	}

	root, err := html.Parse(strings.NewReader(in))
	if err != nil {
		return ""
	}

	var sb strings.Builder
	renderHTMLText(root, &sb)
	return normalizeRenderedText(sb.String())
}

func renderHTMLText(node *html.Node, sb *strings.Builder) {
	if node == nil {
		return
	}

	switch node.Type {
	case html.TextNode:
		sb.WriteString(stdhtml.UnescapeString(node.Data))
	case html.ElementNode:
		tag := strings.ToLower(node.Data)
		switch tag {
		case "script", "style":
			return
		case "pre":
			code := strings.Trim(extractPreformattedText(node), "\n")
			if code == "" {
				return
			}
			sb.WriteString("\n```\n")
			sb.WriteString(code)
			sb.WriteString("\n```\n\n")
			return
		case "br":
			sb.WriteString("\n")
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderHTMLText(child, sb)
	}

	if node.Type != html.ElementNode {
		return
	}

	switch strings.ToLower(node.Data) {
	case "p", "div", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote":
		sb.WriteString("\n")
	}
}

func normalizeRenderedText(in string) string {
	rawLines := strings.Split(strings.ReplaceAll(in, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	inFence := false
	for _, rawLine := range rawLines {
		if strings.TrimSpace(rawLine) == "```" {
			lines = append(lines, "```")
			inFence = !inFence
			continue
		}
		if inFence {
			lines = append(lines, strings.TrimRight(rawLine, "\r"))
			continue
		}

		line := strings.Join(strings.Fields(rawLine), " ")
		if line == "" {
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
			continue
		}
		lines = append(lines, line)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractPreformattedText(node *html.Node) string {
	var sb strings.Builder

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}

		switch n.Type {
		case html.TextNode:
			sb.WriteString(stdhtml.UnescapeString(n.Data))
		case html.ElementNode:
			switch strings.ToLower(n.Data) {
			case "script", "style":
				return
			case "br":
				sb.WriteString("\n")
				return
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child)
	}

	return strings.ReplaceAll(sb.String(), "\r\n", "\n")
}
