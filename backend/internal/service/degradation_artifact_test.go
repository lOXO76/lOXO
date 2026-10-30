//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractDegradationArtifact_Rules(t *testing.T) {
	tests := []struct {
		name, answer, kind, content string
	}{
		{
			name:   "longest html block wins over earlier html and svg",
			answer: "```html\n<p>short</p>\n```\n```svg\n<svg/>\n```\n```HTML\n<html><body>longest document</body></html>\n```",
			kind:   "html", content: "<html><body>longest document</body></html>",
		},
		{
			name:   "equal length html keeps first block",
			answer: "```html\n<p>first</p>\n```\n```html\n<p>other</p>\n```",
			kind:   "html", content: "<p>first</p>",
		},
		{
			name: "htm alias", answer: "```htm\n<div>fragment</div>\n```",
			kind: "html", content: "<div>fragment</div>",
		},
		{
			name: "xhtml alias", answer: "```xhtml\n<div>fragment</div>\n```",
			kind: "html", content: "<div>fragment</div>",
		},
		{
			name: "unlabelled html content", answer: "```\n<HTML>case insensitive</HTML>\n```",
			kind: "html", content: "<HTML>case insensitive</HTML>",
		},
		{
			name: "other language doctype content", answer: "```xml\n<!DoCtYpE hTmL><p>document</p>\n```",
			kind: "html", content: "<!DoCtYpE hTmL><p>document</p>",
		},
		{
			name: "unclosed last html fence", answer: "Explanation\n```html\n<html><body>truncated",
			kind: "html", content: "<html><body>truncated",
		},
		{
			name:   "svg language block precedes raw html",
			answer: "<html>raw</html>\n```svg\n<svg><circle r=\"4\"/></svg>\n```",
			kind:   "svg", content: "<svg><circle r=\"4\"/></svg>",
		},
		{
			name: "xml svg case insensitive", answer: "```XML\n<SVG>drawing</SVG>\n```",
			kind: "svg", content: "<SVG>drawing</SVG>",
		},
		{
			name: "unclosed svg fence", answer: "```svg\n<svg>truncated",
			kind: "svg", content: "<svg>truncated",
		},
		{
			name:   "raw html first start to last close",
			answer: "Explanation\n<!DOCTYPE HTML><HTML>one</HTML><html>two</html>\nSuffix",
			kind:   "html", content: "<!DOCTYPE HTML><HTML>one</HTML><html>two</html>",
		},
		{
			name:   "raw html earliest start beats later doctype",
			answer: "Intro <html>first<!doctype html>later</html> outro",
			kind:   "html", content: "<html>first<!doctype html>later</html>",
		},
		{
			name: "raw html without close", answer: "Intro <html><body>unfinished",
			kind: "html", content: "<html><body>unfinished",
		},
		{
			name:   "raw svg first start to last close",
			answer: "Intro <SVG>one</SVG><svg>two</svg> outro",
			kind:   "svg", content: "<SVG>one</SVG><svg>two</svg>",
		},
		{
			name: "raw svg without close is text", answer: "Intro <svg>unfinished",
			kind: "text", content: "Intro <svg>unfinished",
		},
		{
			name: "text remains intact", answer: "  Refusal or description\nincluding whitespace.\n",
			kind: "text", content: "  Refusal or description\nincluding whitespace.\n",
		},
		{
			name: "unsupported fenced language remains full text", answer: "```python\nprint('text')\n```",
			kind: "text", content: "```python\nprint('text')\n```",
		},
		{
			name: "xml without svg remains full text", answer: "```xml\n<root/>\n```",
			kind: "text", content: "```xml\n<root/>\n```",
		},
		{
			name: "unicode prefix preserves html offsets", answer: "\u212a Intro <HTML>\u212a content</HTML> outro",
			kind: "html", content: "<HTML>\u212a content</HTML>",
		},
		{
			name: "unicode prefix preserves svg offsets", answer: "\u212a Intro <SVG>\u212a drawing</SVG> outro",
			kind: "svg", content: "<SVG>\u212a drawing</SVG>",
		},
		{
			name: "CRLF and indented fence", answer: "  ```HTML\r\n<p>crlf</p>\r\n  ```\r\n",
			kind: "html", content: "<p>crlf</p>",
		},
		{
			name:   "longer fence permits inner triple backticks",
			answer: "````html\n<p>example</p>\n```\n````",
			kind:   "html", content: "<p>example</p>\n```",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, content, err := extractDegradationArtifact(tt.answer)
			require.NoError(t, err)
			require.Equal(t, tt.kind, kind)
			require.Equal(t, tt.content, content)
		})
	}
}

func TestExtractDegradationArtifact_InjectsCSSAndScripts(t *testing.T) {
	answer := "```html\n<html><head><style>existing</style></HEAD><body><script>already()</script></BODY></html>\n```\n" +
		"```css\nexisting\n```\n```css\nbody { color: red; }\n```\n```CSS\np { display: grid; }\n```\n" +
		"```js\nalready()\n```\n```javascript\nconsole.log('classic');\n```\n" +
		"```mjs\nimport * as THREE from 'three';\nrender();\n```\n```js\n// indented module\n  import {thing} from 'module';\n```"
	kind, content, err := extractDegradationArtifact(answer)
	require.NoError(t, err)
	require.Equal(t, "html", kind)
	require.Equal(t, "<html><head><style>existing</style><style>\nbody { color: red; }\n</style>\n<style>\np { display: grid; }\n</style>\n</HEAD><body><script>already()</script><script>\nconsole.log('classic');\n</script>\n<script type=\"module\">\nimport * as THREE from 'three';\nrender();\n</script>\n<script type=\"module\">\n// indented module\n  import {thing} from 'module';\n</script>\n</BODY></html>", content)
}

func TestExtractDegradationArtifact_InjectsIntoFragmentAndUnclosedScript(t *testing.T) {
	answer := "```html\n<div>\u212a scene</div>\n```\n```css\ndiv{margin:0}\n```\n```js\nrun()"
	kind, content, err := extractDegradationArtifact(answer)
	require.NoError(t, err)
	require.Equal(t, "html", kind)
	require.Equal(t, "<style>\ndiv{margin:0}\n</style>\n<div>\u212a scene</div><script>\nrun()\n</script>\n", content)
}

func TestExtractDegradationArtifact_ByteLimit(t *testing.T) {
	for _, prefix := range []string{"", "```html\n", "```svg\n"} {
		answer := prefix + strings.Repeat("x", degradationMaxArtifactBytes)
		_, content, err := extractDegradationArtifact(answer)
		require.NoError(t, err)
		require.Len(t, content, degradationMaxArtifactBytes)
		kind, content, err := extractDegradationArtifact(answer + "x")
		require.ErrorContains(t, err, "512 KiB")
		require.Empty(t, kind)
		require.Empty(t, content)
	}
	// Two-byte code points ensure the bound is bytes, not rune count.
	_, content, err := extractDegradationArtifact(strings.Repeat("\u00e9", degradationMaxArtifactBytes/2))
	require.NoError(t, err)
	require.Len(t, content, degradationMaxArtifactBytes)
	_, _, err = extractDegradationArtifact(strings.Repeat("\u00e9", degradationMaxArtifactBytes/2+1))
	require.Error(t, err)
	_, _, err = extractDegradationArtifact("```html\n" + strings.Repeat("x", degradationMaxArtifactBytes) + "\n```\n```css\na{}\n```")
	require.ErrorContains(t, err, "512 KiB", "injection overhead counts toward the final artifact size")
	_, content, err = extractDegradationArtifact(strings.Repeat("p", degradationMaxArtifactBytes+1) + "<html>small</html>")
	require.NoError(t, err, "the limit applies to the extracted artifact, not discarded prose")
	require.Equal(t, "<html>small</html>", content)
}
