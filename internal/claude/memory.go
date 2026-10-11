package claude

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MemoryStore is a memory directory: a MEMORY.md index plus one file per
// memory.
type MemoryStore struct {
	// Label names the store: the project path for auto memory, otherwise the
	// directory.
	Label string `json:"label"`
	Dir   string `json:"dir"`
	// Kind is auto (per-project auto memory), configured (autoMemoryDirectory)
	// or extra (a directory passed to dotclaude).
	Kind string `json:"kind"`
	// Project is the project path the store belongs to, when known.
	Project string        `json:"project,omitempty"`
	Index   *MemoryIndex  `json:"index,omitempty"`
	Entries []MemoryEntry `json:"entries"`
}

// MemoryIndex is the MEMORY.md file.
type MemoryIndex struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Bytes int    `json:"bytes"`
	// Links are the files the index points to, in order.
	Links []IndexLink `json:"links"`
}

// IndexLink is one "[Title](file.md)" entry in MEMORY.md.
type IndexLink struct {
	Title  string `json:"title"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Exists bool   `json:"exists"`
}

// MemoryEntry is one memory file.
type MemoryEntry struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Type             string   `json:"type"`
	File             string   `json:"file"`
	Path             string   `json:"path"`
	Links            []string `json:"links"`
	Bytes            int      `json:"bytes"`
	FrontmatterError string   `json:"frontmatterError,omitempty"`
}

var (
	wikiLink  = regexp.MustCompile(`\[\[([^\]\n|]+)(?:\|[^\]\n]*)?\]\]`)
	indexLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+\.md)\)`)
)

// readMemoryStore reads a memory directory. It returns nil when the directory
// does not exist.
func readMemoryStore(dir, label, kind string) *MemoryStore {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	s := &MemoryStore{Label: label, Dir: dir, Kind: kind, Entries: []MemoryEntry{}}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if name == "MEMORY.md" {
			s.Index = parseIndex(path, data, dir)
			continue
		}
		doc := ParseDocument(data)
		m := MemoryEntry{
			Name: doc.String("name"), Description: doc.String("description"),
			File: name, Path: path, Bytes: len(data), FrontmatterError: doc.FrontmatterError,
			Links: []string{},
		}
		m.Type = doc.String("type")
		if meta, ok := doc.Frontmatter["metadata"].(map[string]any); ok && m.Type == "" {
			m.Type, _ = meta["type"].(string)
		}
		seen := map[string]bool{}
		for _, l := range wikiLink.FindAllStringSubmatch(doc.Body, -1) {
			target := strings.TrimSpace(l[1])
			if !seen[target] {
				seen[target] = true
				m.Links = append(m.Links, target)
			}
		}
		s.Entries = append(s.Entries, m)
	}
	sort.Slice(s.Entries, func(i, j int) bool { return s.Entries[i].File < s.Entries[j].File })
	return s
}

func parseIndex(path string, data []byte, dir string) *MemoryIndex {
	idx := &MemoryIndex{Path: path, Bytes: len(data), Links: []IndexLink{}}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	idx.Lines = len(lines)
	for i, line := range lines {
		for _, m := range indexLink.FindAllStringSubmatch(line, -1) {
			file := m[2]
			if strings.Contains(file, "://") {
				continue
			}
			idx.Links = append(idx.Links, IndexLink{
				Title: m[1], File: file, Line: i + 1,
				Exists: isFile(filepath.Join(dir, file)),
			})
		}
	}
	return idx
}

// Key is the name other memories link to: the front matter name, or the file
// name without extension.
func (m MemoryEntry) Key() string {
	if m.Name != "" {
		return m.Name
	}
	return strings.TrimSuffix(m.File, ".md")
}
