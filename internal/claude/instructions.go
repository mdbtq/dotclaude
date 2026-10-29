package claude

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Instruction is an instruction file Claude Code loads into context: a
// CLAUDE.md, CLAUDE.local.md, AGENTS.md or rule file, or a file one of those
// imports.
type Instruction struct {
	Scope Scope `json:"scope"`
	// Kind is claude-md, local, agents-md, rule or import.
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Lines  int    `json:"lines"`
	Bytes  int    `json:"bytes"`
	Error  string `json:"error,omitempty"`
	Exists bool   `json:"exists"`
	// Conditional is set for rules with a paths field: they load only when
	// Claude works with a matching file.
	Conditional bool `json:"conditional,omitempty"`
	// Import is the @-reference as written, for imported files.
	Import string `json:"import,omitempty"`
	// Line is where the import appears in its parent file.
	Line int `json:"line,omitempty"`
	// External marks a project import that resolves outside the project,
	// which Claude Code only loads after the user approves it.
	External bool `json:"external,omitempty"`
	// AppliesUnder is set in the user-level view for files in a directory
	// above known projects: they load only in projects below that directory.
	AppliesUnder string `json:"appliesUnder,omitempty"`
	// Projects is how many known projects AppliesUnder covers.
	Projects int           `json:"projects,omitempty"`
	Imports  []Instruction `json:"imports,omitempty"`
}

// maxImportDepth is how many import hops Claude Code follows.
const maxImportDepth = 4

// readInstruction reads an instruction file and, recursively, its imports.
// projectRoot is used to flag external imports; it is empty for user scope.
func readInstruction(path string, scope Scope, kind, projectRoot string, depth int, seen map[string]bool) Instruction {
	in := Instruction{Scope: scope, Kind: kind, Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			in.Error = err.Error()
		}
		return in
	}
	in.Exists = true
	in.Bytes = len(data)
	in.Lines = bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		in.Lines++
	}
	if kind == "rule" {
		doc := ParseDocument(data)
		_, in.Conditional = doc.Frontmatter["paths"]
	}
	if depth >= maxImportDepth || seen[path] {
		return in
	}
	seen[path] = true
	defer delete(seen, path)
	for _, ref := range FindImports(string(data)) {
		target := resolveImport(path, ref.Path)
		child := readInstruction(target, scope, "import", projectRoot, depth+1, seen)
		child.Import, child.Line = "@"+ref.Path, ref.Line
		child.External = projectRoot != "" && !within(target, projectRoot)
		in.Imports = append(in.Imports, child)
	}
	return in
}

// ImportRef is an @path reference found in an instruction file.
type ImportRef struct {
	Path string
	Line int
}

var (
	importToken = regexp.MustCompile(`(?:^|[\s(])@((?:\\ |[^\s` + "`" + `])+)`)
	codeSpan    = regexp.MustCompile("`[^`]*`")
)

// FindImports returns the @path references in text, skipping fenced code
// blocks and code spans as Claude Code does. A backslash before a space keeps
// the space in the path. Only tokens that look like a path (containing "/" or
// "." or starting with "~") are returned: prose like "@someone" is far more
// common than an extensionless import, and reporting it would be noise.
// Trailing sentence punctuation is dropped for the same reason.
func FindImports(text string) []ImportRef {
	var out []ImportRef
	fence := ""
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			switch {
			case fence == "":
				fence = marker
			case fence == marker:
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		line = codeSpan.ReplaceAllString(line, "")
		for _, m := range importToken.FindAllStringSubmatch(line, -1) {
			p := strings.TrimRight(m[1], ".,;:!?)")
			p = strings.ReplaceAll(p, `\ `, " ")
			if p == "" || !(strings.ContainsAny(p, "/.") || strings.HasPrefix(p, "~")) {
				continue
			}
			out = append(out, ImportRef{Path: p, Line: i + 1})
		}
	}
	return out
}

func resolveImport(from, ref string) string {
	if strings.HasPrefix(ref, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ref[2:])
		}
	}
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref)
	}
	return filepath.Join(filepath.Dir(from), ref)
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// projectInstructions lists the instruction files that load at launch in
// dir, in load order: for every directory from the filesystem root down to
// dir, its CLAUDE.md, .claude/CLAUDE.md and CLAUDE.local.md, followed by the
// rules in each .claude/rules. AGENTS.md files are read instead when none of
// the CLAUDE.md files exist anywhere in that chain. Paths in skip (the user's
// own files when the home directory is part of the chain) are left out.
func projectInstructions(dir, projectRoot string, skip map[string]bool) []Instruction {
	var chain []string
	for d := dir; ; d = filepath.Dir(d) {
		chain = append([]string{d}, chain...)
		if filepath.Dir(d) == d {
			break
		}
	}

	var out []Instruction
	hasClaudeMd := false
	add := func(path string, scope Scope, kind string) {
		if skip[path] || !isFile(path) {
			return
		}
		out = append(out, readInstruction(path, scope, kind, projectRoot, 0, map[string]bool{}))
	}
	for _, d := range chain {
		for _, name := range []string{"CLAUDE.md", filepath.Join(".claude", "CLAUDE.md"), "CLAUDE.local.md"} {
			path := filepath.Join(d, name)
			if skip[path] || !isFile(path) {
				continue
			}
			hasClaudeMd = true
			scope, kind := ScopeProject, "claude-md"
			if name == "CLAUDE.local.md" {
				scope, kind = ScopeLocal, "local"
			}
			add(path, scope, kind)
		}
	}
	if !hasClaudeMd {
		for _, d := range chain {
			add(filepath.Join(d, "AGENTS.md"), ScopeProject, "agents-md")
			add(filepath.Join(d, ".claude", "AGENTS.md"), ScopeProject, "agents-md")
		}
	}
	for _, d := range chain {
		walkMarkdown(filepath.Join(d, ".claude", "rules"), func(path, _ string) {
			add(path, ScopeProject, "rule")
		})
	}
	return out
}

// ancestorInstructions lists, for the user-level view, the instruction files
// in directories above the known projects' repository roots. Each loads only
// in projects below its directory, so it applies by project path rather than
// everywhere. Paths in skip (the user's own files) are left out.
func ancestorInstructions(projects []string, skip map[string]bool) []Instruction {
	under := map[string]int{}
	for _, proj := range projects {
		if !isDir(proj) {
			continue
		}
		root, _ := repoRoot(proj)
		for d := filepath.Dir(root); ; d = filepath.Dir(d) {
			under[d]++
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	dirs := make([]string, 0, len(under))
	for d := range under {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var out []Instruction
	for _, d := range dirs {
		add := func(path string, scope Scope, kind string) {
			if skip[path] || !isFile(path) {
				return
			}
			in := readInstruction(path, scope, kind, "", 0, map[string]bool{})
			in.AppliesUnder, in.Projects = d, under[d]
			out = append(out, in)
		}
		add(filepath.Join(d, "CLAUDE.md"), ScopeProject, "claude-md")
		add(filepath.Join(d, ".claude", "CLAUDE.md"), ScopeProject, "claude-md")
		add(filepath.Join(d, "CLAUDE.local.md"), ScopeLocal, "local")
		walkMarkdown(filepath.Join(d, ".claude", "rules"), func(path, _ string) {
			add(path, ScopeProject, "rule")
		})
	}
	return out
}
