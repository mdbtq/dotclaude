package claude

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document is a markdown file with optional YAML front matter.
type Document struct {
	Frontmatter map[string]any
	Body        string
	// FrontmatterError is set when the front matter is present but is not
	// valid YAML. Frontmatter then holds a best-effort "key: value" reading,
	// which is roughly what a lenient loader would still pick up.
	FrontmatterError string
}

// ReadDocument reads and parses a markdown file.
func ReadDocument(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	return ParseDocument(data), nil
}

// ParseDocument splits front matter from the body. Front matter must start on
// the first line with "---" and end with a line that is exactly "---".
func ParseDocument(data []byte) Document {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Document{Body: text}
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	var raw, body string
	switch {
	case end >= 0:
		raw, body = rest[:end], rest[end+len("\n---\n"):]
	case strings.HasSuffix(rest, "\n---"):
		raw = strings.TrimSuffix(rest, "\n---")
	default:
		return Document{Body: text}
	}

	doc := Document{Body: body}
	fm := map[string]any{}
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
		doc.FrontmatterError = fmt.Sprintf("invalid YAML front matter: %v", err)
		fm = lenientFrontmatter(raw)
	}
	doc.Frontmatter = fm
	return doc
}

// lenientFrontmatter reads "key: value" lines, for front matter that strict
// YAML rejects (typically an unquoted description containing ": "). Indented
// lines under a key without a value become a nested map, one level deep.
func lenientFrontmatter(raw string) map[string]any {
	fm := map[string]any{}
	var parent map[string]any
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		indented := line[0] == ' ' || line[0] == '\t'
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`)
		switch {
		case indented && parent != nil:
			parent[key] = value
		case indented:
			continue
		case value == "":
			parent = map[string]any{}
			fm[key] = parent
		default:
			parent = nil
			fm[key] = value
		}
	}
	return fm
}

// String returns a front matter value as a string, or "" when absent.
func (d Document) String(key string) string {
	switch v := d.Frontmatter[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// Bool returns a front matter value as a boolean.
func (d Document) Bool(key string) bool {
	switch v := d.Frontmatter[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}
