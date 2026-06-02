package jira_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

type rendererFixture struct {
	name string
	adf  string
	want string
}

func TestRenderADFToText(t *testing.T) {
	fixtures := []rendererFixture{
		{
			name: "paragraph and text concatenation",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "text", "text": "Hello "},
							{"type": "text", "text": "world"}
						]
					}
				]
			}`,
			want: "Hello world",
		},
		{
			name: "heading lists and hardBreak",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "heading",
						"content": [{"type": "text", "text": "Plan"}]
					},
					{
						"type": "bulletList",
						"content": [
							{
								"type": "listItem",
								"content": [
									{
										"type": "paragraph",
										"content": [
											{"type": "text", "text": "Alpha"},
											{"type": "hardBreak"},
											{"type": "text", "text": "beta"}
										]
									}
								]
							},
							{
								"type": "listItem",
								"content": [
									{
										"type": "paragraph",
										"content": [{"type": "text", "text": "Gamma"}]
									}
								]
							}
						]
					},
					{
						"type": "orderedList",
						"content": [
							{
								"type": "listItem",
								"content": [
									{
										"type": "paragraph",
										"content": [{"type": "text", "text": "One"}]
									}
								]
							},
							{
								"type": "listItem",
								"content": [
									{
										"type": "paragraph",
										"content": [{"type": "text", "text": "Two"}]
									}
								]
							}
						]
					}
				]
			}`,
			want: "# Plan\n\n- Alpha\n  beta\n- Gamma\n\n1. One\n2. Two",
		},
		{
			name: "code block handling",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "Code:"}]
					},
					{
						"type": "codeBlock",
						"content": [
							{"type": "text", "text": "line1"},
							{"type": "hardBreak"},
							{"type": "text", "text": "line2"}
						]
					}
				]
			}`,
			want: "Code:\n\n```\nline1\nline2\n```",
		},
		{
			name: "unknown nodes are skipped",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "before"}]
					},
					{
						"type": "mysteryNode",
						"content": [
							{
								"type": "paragraph",
								"content": [{"type": "text", "text": "hidden"}]
							}
						]
					},
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "after"}]
					}
				]
			}`,
			want: "before\n\nafter",
		},
		{
			name: "unknown container preserves supported descendants",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "expand",
						"content": [
							{
								"type": "paragraph",
								"content": [{"type": "text", "text": "Before block"}]
							},
							{
								"type": "codeBlock",
								"content": [
									{"type": "text", "text": "line1"},
									{"type": "hardBreak"},
									{"type": "text", "text": "line2"}
								]
							}
						]
					},
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "After block"}]
					}
				]
			}`,
			want: "Before block\n\n```\nline1\nline2\n```\n\nAfter block",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			got, err := jira.RenderADFToText(json.RawMessage(fixture.adf))
			require.NoError(t, err)
			assert.Equal(t, fixture.want, got)
		})
	}
}

type fallbackFixture struct {
	name string
	adf  string
	html string
	want string
}

func TestRenderADFToTextWithHTMLFallback(t *testing.T) {
	fixtures := []fallbackFixture{
		{
			name: "uses adf when render succeeds",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "from adf"}]
					}
				]
			}`,
			html: "<p>from html</p>",
			want: "from adf",
		},
		{
			name: "falls back when adf parsing fails",
			adf:  `{"type":"doc","content":[`,
			html: "<p>Hello <b>world</b> &amp; team</p>",
			want: "Hello world & team",
		},
		{
			name: "falls back when non-empty adf renders empty",
			adf: `{
				"type": "doc",
				"content": [
					{
						"type": "unsupportedBlock",
						"content": [{"type": "text", "text": "hidden"}]
					}
				]
			}`,
			html: "<div>Fallback <i>text</i></div>",
			want: "Fallback text",
		},
		{
			name: "does not fallback for empty adf content",
			adf: `{
				"type": "doc",
				"content": []
			}`,
			html: "<p>should not appear</p>",
			want: "",
		},
		{
			name: "preserves html preformatted code blocks",
			adf:  `{"type":"doc","content":[`,
			html: "<p>before</p><pre><code>line1\n  line2\nline3</code></pre><p>after</p>",
			want: "before\n\n```\nline1\n  line2\nline3\n```\n\nafter",
		},
		{
			name: "falls back to html when adf contains codeBlock but rendered output loses fences",
			adf: `{
				"type":"doc",
				"content":[
					{
						"type":"paragraph",
						"content":[{"type":"text","text":"Intro"}]
					},
					{
						"type":"mysteryWrapper",
						"content":[
							{
								"type":"codeBlock",
								"content":[{"type":"text","text":"fmt.Println(\"hello\")"}]
							}
						]
					}
				]
			}`,
			html: "<p>Intro</p><pre><code>fmt.Println(\"hello\")</code></pre>",
			want: "Intro\n\n```\nfmt.Println(\"hello\")\n```",
		},
		{
			name: "falls back to html when adf has extension macro code and no fences",
			adf: `{
				"type":"doc",
				"content":[
					{
						"type":"paragraph",
						"content":[{"type":"text","text":"Migration:"}]
					},
					{
						"type":"extension",
						"attrs":{
							"extensionType":"com.atlassian.ecosystem",
							"extensionKey":"com.atlassian.confluence.macro.core",
							"parameters":{
								"macroMetadata":{"title":"code"}
							}
						}
					},
					{
						"type":"paragraph",
						"content":[{"type":"text","text":"Likely queries"}]
					}
				]
			}`,
			html: "<p>Migration:</p><pre><code>CREATE TABLE payment_methods (\n  id UUID PRIMARY KEY,\n  status TEXT NOT NULL DEFAULT 'active'\n);</code></pre><p>Likely queries</p>",
			want: "Migration:\n\n```\nCREATE TABLE payment_methods (\n  id UUID PRIMARY KEY,\n  status TEXT NOT NULL DEFAULT 'active'\n);\n```\n\nLikely queries",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			got := jira.RenderADFToTextWithHTMLFallback(json.RawMessage(fixture.adf), fixture.html)
			assert.Equal(t, fixture.want, got)
		})
	}
}
