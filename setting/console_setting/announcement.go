package console_setting

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var announcementAllowedTags = map[string]bool{
	"a": true, "b": true, "blockquote": true, "br": true, "code": true,
	"del": true, "div": true, "em": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "hr": true,
	"i": true, "li": true, "ol": true, "p": true, "pre": true,
	"s": true, "span": true, "strong": true, "table": true,
	"tbody": true, "td": true, "th": true, "thead": true, "tr": true,
	"u": true, "ul": true,
}

var announcementDiscardedTags = map[string]bool{
	"embed": true, "iframe": true, "object": true, "script": true,
	"style": true, "template": true,
}

func NormalizeAnnouncementsJSON(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	announcements, err := parseJSONArray(raw, "系统公告")
	if err != nil {
		return "", err
	}
	for index, announcement := range announcements {
		if value, exists := announcement["title"]; exists {
			title, ok := value.(string)
			if !ok {
				return "", fmt.Errorf("第%d个公告的标题格式不正确", index+1)
			}
			if title = sanitizeAnnouncementTitle(title); title == "" {
				delete(announcement, "title")
			} else {
				announcement["title"] = title
			}
		}
		for _, field := range []string{"content", "extra"} {
			value, exists := announcement[field]
			if !exists {
				continue
			}
			text, ok := value.(string)
			if !ok {
				return "", fmt.Errorf("第%d个公告的%s格式不正确", index+1, announcementFieldName(field))
			}
			announcement[field] = sanitizeAnnouncementHTML(text)
		}
		if value, exists := announcement["published"]; exists {
			if _, ok := value.(bool); !ok {
				return "", fmt.Errorf("第%d个公告的发布状态格式不正确", index+1)
			}
		} else {
			announcement["published"] = true
		}
		if value, exists := announcement["type"]; !exists || strings.TrimSpace(fmt.Sprint(value)) == "" {
			announcement["type"] = "default"
		}
		delete(announcement, "content_format")
	}
	payload, err := common.Marshal(announcements)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func sanitizeAnnouncementTitle(value string) string {
	nodes := parseAnnouncementFragment(value)
	var builder strings.Builder
	for _, node := range nodes {
		appendAnnouncementText(&builder, node)
	}
	return strings.TrimSpace(builder.String())
}

func sanitizeAnnouncementHTML(value string) string {
	nodes := parseAnnouncementFragment(value)
	var buffer bytes.Buffer
	for _, node := range nodes {
		for _, clean := range sanitizeAnnouncementNode(node) {
			_ = html.Render(&buffer, clean)
		}
	}
	return strings.TrimSpace(buffer.String())
}

func parseAnnouncementFragment(value string) []*html.Node {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(value), context)
	if err != nil {
		return []*html.Node{{Type: html.TextNode, Data: value}}
	}
	return nodes
}

func sanitizeAnnouncementNode(node *html.Node) []*html.Node {
	switch node.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: node.Data}}
	case html.ElementNode:
		tag := strings.ToLower(node.Data)
		if announcementDiscardedTags[tag] {
			return nil
		}
		children := sanitizeAnnouncementChildren(node)
		if !announcementAllowedTags[tag] {
			return children
		}
		clean := &html.Node{Type: html.ElementNode, DataAtom: node.DataAtom, Data: tag}
		clean.Attr = sanitizeAnnouncementAttributes(tag, node.Attr)
		for _, child := range children {
			clean.AppendChild(child)
		}
		return []*html.Node{clean}
	default:
		return nil
	}
}

func sanitizeAnnouncementChildren(node *html.Node) []*html.Node {
	children := make([]*html.Node, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, sanitizeAnnouncementNode(child)...)
	}
	return children
}

func sanitizeAnnouncementAttributes(tag string, attributes []html.Attribute) []html.Attribute {
	clean := make([]html.Attribute, 0, len(attributes)+1)
	for _, attribute := range attributes {
		key := strings.ToLower(attribute.Key)
		switch {
		case tag == "a" && key == "href" && isSafeAnnouncementLink(attribute.Val):
			clean = append(clean, html.Attribute{Key: "href", Val: strings.TrimSpace(attribute.Val)})
		case tag == "a" && key == "target" && attribute.Val == "_blank":
			clean = append(clean, html.Attribute{Key: "target", Val: "_blank"})
		case tag == "a" && key == "title":
			clean = append(clean, html.Attribute{Key: "title", Val: attribute.Val})
		case (tag == "td" || tag == "th") && (key == "colspan" || key == "rowspan"):
			clean = append(clean, html.Attribute{Key: key, Val: attribute.Val})
		}
	}
	if tag == "a" && hasAnnouncementAttribute(clean, "href") {
		clean = append(clean, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
	}
	return clean
}

func isSafeAnnouncementLink(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "mailto") && parsed.Host != "" ||
		parsed.Scheme == "mailto" && parsed.Opaque != ""
}

func hasAnnouncementAttribute(attributes []html.Attribute, key string) bool {
	for _, attribute := range attributes {
		if attribute.Key == key {
			return true
		}
	}
	return false
}

func appendAnnouncementText(builder *strings.Builder, node *html.Node) {
	if node.Type == html.ElementNode && announcementDiscardedTags[strings.ToLower(node.Data)] {
		return
	}
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendAnnouncementText(builder, child)
	}
}

func announcementFieldName(field string) string {
	if field == "extra" {
		return "说明"
	}
	return "内容"
}
