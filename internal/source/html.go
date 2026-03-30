package source

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/himanshulodha/ai-news-slack/internal/httpx"
)

const maxSlackMessageLen = 2800

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}

	var builder strings.Builder
	var walkText func(*html.Node)
	walkText = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walkText(child)
		}
	}
	walkText(node)

	return builder.String()
}

func toSlackMarkdown(node *html.Node, pageURL string) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case html.TextNode:
		return node.Data
	case html.ElementNode:
		if node.Data == "a" {
			label := normalizeWhitespace(textContent(node))
			href := getAttr(node, "href")
			if label == "" || href == "" {
				return label
			}

			resolved, err := httpx.ResolveURL(pageURL, href)
			if err != nil {
				return label
			}

			cleanLabel := strings.ReplaceAll(strings.ReplaceAll(label, "<", ""), ">", "")
			return "<" + resolved + "|" + cleanLabel + ">"
		}

		var builder strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			builder.WriteString(toSlackMarkdown(child, pageURL))
		}

		content := normalizeWhitespace(builder.String())
		switch node.Data {
		case "strong", "b":
			if content == "" {
				return ""
			}
			return "*" + content + "*"
		case "em", "i":
			if content == "" {
				return ""
			}
			return "_" + content + "_"
		case "code":
			if content == "" {
				return ""
			}
			return "`" + content + "`"
		default:
			return builder.String()
		}
	default:
		return ""
	}
}

func collectContentNodes(root *html.Node) []*html.Node {
	tags := map[string]struct{}{
		"h1": {}, "h2": {}, "h3": {}, "h4": {}, "h5": {}, "h6": {},
		"p": {}, "ul": {}, "ol": {},
	}

	var nodes []*html.Node
	walk(root, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if _, ok := tags[node.Data]; !ok {
			return
		}
		if normalizeWhitespace(textContent(node)) == "" {
			return
		}
		nodes = append(nodes, node)
	})

	return nodes
}

func findContentIndex(nodes []*html.Node, target string) int {
	for index, node := range nodes {
		if strings.EqualFold(normalizeWhitespace(textContent(node)), target) {
			return index
		}
	}
	return -1
}

func findFirstTag(root *html.Node, tag string) *html.Node {
	var found *html.Node
	walk(root, func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == tag {
			found = node
		}
	})

	return found
}

func findAncestorTag(node *html.Node, tag string) *html.Node {
	for current := node; current != nil; current = current.Parent {
		if current.Type == html.ElementNode && current.Data == tag {
			return current
		}
	}
	return nil
}

func getAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func walk(node *html.Node, visit func(*html.Node)) {
	var traverse func(*html.Node)
	traverse = func(current *html.Node) {
		visit(current)
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}
	}
	traverse(node)
}

func splitLongMessage(text string) []string {
	if len(text) <= maxSlackMessageLen {
		return []string{text}
	}

	var parts []string
	current := ""
	for _, line := range strings.Split(text, "\n") {
		candidate := line
		if current != "" {
			candidate = current + "\n" + line
		}

		if len(candidate) <= maxSlackMessageLen {
			current = candidate
			continue
		}

		if current != "" {
			parts = append(parts, current)
		}
		current = line
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func buildIntroMessages(nodes []*html.Node, recapStart int, pageURL string) []string {
	if recapStart <= 1 {
		return nil
	}

	var lines []string
	for _, node := range nodes[1:recapStart] {
		switch node.Data {
		case "p":
			line := normalizeWhitespace(toSlackMarkdown(node, pageURL))
			if line != "" {
				lines = append(lines, line)
			}
		case "ul", "ol":
			lines = append(lines, bulletLines(node, pageURL)...)
		}
	}

	return lines
}

func buildRecapMessages(nodes []*html.Node, pageURL string) []string {
	var blocks []string
	var heading string
	var lines []string

	flush := func() {
		payload := make([]string, 0, len(lines)+1)
		if heading != "" {
			payload = append(payload, "*"+heading+"*")
		}
		payload = append(payload, lines...)
		if len(payload) == 0 {
			return
		}

		blocks = append(blocks, splitLongMessage(strings.Join(payload, "\n"))...)
		heading = ""
		lines = nil
	}

	for index, node := range nodes {
		text := normalizeWhitespace(textContent(node))
		if text == "" {
			continue
		}

		switch node.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			flush()
			heading = text
		case "ul", "ol":
			lines = append(lines, bulletLines(node, pageURL)...)
		case "p":
			nextTag := ""
			if index+1 < len(nodes) {
				nextTag = nodes[index+1].Data
			}
			if (nextTag == "ul" || nextTag == "ol") && len(lines) == 0 {
				flush()
				heading = text
				continue
			}

			line := normalizeWhitespace(toSlackMarkdown(node, pageURL))
			if line != "" {
				lines = append(lines, line)
			}
		}
	}

	flush()
	return blocks
}

func bulletLines(listNode *html.Node, pageURL string) []string {
	var lines []string
	for child := listNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.Data != "li" {
			continue
		}

		line := normalizeWhitespace(toSlackMarkdown(child, pageURL))
		if line != "" {
			lines = append(lines, "• "+line)
		}
	}

	return lines
}

func buildItemID(rawURL, publishedAt string) string {
	if publishedAt == "" {
		publishedAt = "unknown"
	}
	return rawURL + "::" + publishedAt
}

func looksLikeAINews(rawURL, title string) bool {
	haystack := strings.ToLower(rawURL + " " + title)
	return strings.Contains(haystack, "ainews") || strings.Contains(haystack, "ai news")
}
