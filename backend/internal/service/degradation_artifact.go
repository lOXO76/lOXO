package service

import (
	"fmt"
	"regexp"
	"strings"
)

const degradationMaxArtifactBytes = 512 * 1024

var degradationModuleImport = regexp.MustCompile(`(?m)^[\t ]*import[\t ]`)

type degradationCodeBlock struct {
	language string
	content  string
}

func extractDegradationArtifact(answer string) (kind, content string, err error) {
	blocks := parseDegradationCodeBlocks(answer)
	htmlIndex := -1
	for i, block := range blocks {
		lower := strings.ToLower(block.content)
		isHTML := block.language == "html" || block.language == "htm" || block.language == "xhtml" ||
			strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype html")
		if isHTML && (htmlIndex < 0 || len(block.content) > len(blocks[htmlIndex].content)) {
			htmlIndex = i
		}
	}
	if htmlIndex >= 0 {
		content = blocks[htmlIndex].content
		originalHTML := content
		var styles, scripts strings.Builder
		for i, block := range blocks {
			if i == htmlIndex || block.content == "" || strings.Contains(originalHTML, block.content) {
				continue
			}
			switch block.language {
			case "css":
				_, _ = styles.WriteString("<style>\n" + block.content + "\n</style>\n")
			case "js", "javascript", "mjs":
				if degradationModuleImport.MatchString(block.content) {
					_, _ = scripts.WriteString("<script type=\"module\">\n" + block.content + "\n</script>\n")
				} else {
					_, _ = scripts.WriteString("<script>\n" + block.content + "\n</script>\n")
				}
			}
		}
		content = insertDegradationBefore(content, "</head>", styles.String(), true)
		content = insertDegradationBefore(content, "</body>", scripts.String(), false)
		return checkedDegradationArtifact(DegradationArtifactHTML, content)
	}
	for _, block := range blocks {
		if block.language == "svg" || block.language == "xml" && strings.Contains(strings.ToLower(block.content), "<svg") {
			return checkedDegradationArtifact(DegradationArtifactSVG, block.content)
		}
	}
	lower := degradationASCIILower(answer)
	start := strings.Index(lower, "<!doctype html")
	if htmlStart := strings.Index(lower, "<html"); htmlStart >= 0 && (start < 0 || htmlStart < start) {
		start = htmlStart
	}
	if start >= 0 {
		end := len(answer)
		if closing := strings.LastIndex(lower, "</html>"); closing >= start {
			end = closing + len("</html>")
		}
		return checkedDegradationArtifact(DegradationArtifactHTML, answer[start:end])
	}
	if start := strings.Index(lower, "<svg"); start >= 0 {
		if closing := strings.LastIndex(lower, "</svg>"); closing >= start {
			return checkedDegradationArtifact(DegradationArtifactSVG, answer[start:closing+len("</svg>")])
		}
	}
	return checkedDegradationArtifact(DegradationArtifactText, answer)
}

func checkedDegradationArtifact(kind, content string) (string, string, error) {
	if len(content) > degradationMaxArtifactBytes {
		return "", "", fmt.Errorf("degradation visual artifact exceeds 512 KiB (%d bytes)", len(content))
	}
	return kind, content, nil
}

func insertDegradationBefore(document, closing, addition string, prepend bool) string {
	if addition == "" {
		return document
	}
	if index := strings.Index(degradationASCIILower(document), closing); index >= 0 {
		return document[:index] + addition + document[index:]
	}
	if prepend {
		return addition + document
	}
	return document + addition
}

// Preserve byte offsets while matching ASCII markup in otherwise Unicode text.
func degradationASCIILower(value string) string {
	lower := []byte(value)
	for i, char := range lower {
		if char >= 'A' && char <= 'Z' {
			lower[i] = char + ('a' - 'A')
		}
	}
	return string(lower)
}

func parseDegradationCodeBlocks(answer string) []degradationCodeBlock {
	var blocks []degradationCodeBlock
	var language string
	contentStart, fenceLength := -1, 0
	for offset := 0; offset < len(answer); {
		lineEnd := strings.IndexByte(answer[offset:], '\n')
		next := len(answer)
		if lineEnd < 0 {
			lineEnd = len(answer)
		} else {
			lineEnd += offset
			next = lineEnd + 1
		}
		line := strings.TrimSuffix(answer[offset:lineEnd], "\r")
		count, info := degradationFence(line)
		if contentStart < 0 && count >= 3 && !strings.Contains(info, "`") {
			language = ""
			if fields := strings.Fields(info); len(fields) > 0 {
				language = strings.ToLower(fields[0])
			}
			contentStart, fenceLength = next, count
		} else if contentStart >= 0 && count >= fenceLength && info == "" {
			content := strings.TrimSuffix(strings.TrimSuffix(answer[contentStart:offset], "\n"), "\r")
			blocks = append(blocks, degradationCodeBlock{language: language, content: content})
			contentStart = -1
		}
		offset = next
	}
	if contentStart >= 0 {
		blocks = append(blocks, degradationCodeBlock{language: language, content: answer[contentStart:]})
	}
	return blocks
}

func degradationFence(line string) (int, string) {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 {
		return 0, ""
	}
	line = line[indent:]
	count := len(line) - len(strings.TrimLeft(line, "`"))
	if count < 3 {
		return 0, ""
	}
	return count, strings.TrimSpace(line[count:])
}
