package fix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mdbtq/dotclaude/internal/claude"
)

// skillOverride switches a skill off in the user's settings.local.json,
// where Claude Code itself keeps skill toggles that apply to every project,
// or switches it back on by removing every "off" entry for it.
func skillOverride(ws workspace, c *claude.Config, p claude.Paths, r Request) (*Plan, error) {
	found, active := false, false
	for _, comp := range c.Components {
		if comp.Kind != claude.KindAgent && comp.Name == r.Target {
			found, active = true, active || comp.Status == claude.StatusActive
		}
	}
	if !found {
		return nil, fmt.Errorf("no skill or command named %q: %w", r.Target, ErrNotApplicable)
	}

	switch r.Value {
	case "off":
		if !active {
			return nil, fmt.Errorf("%s is already off: %w", r.Target, ErrNotApplicable)
		}
		ch, err := editSettings(ws, filepath.Join(p.ConfigDir, "settings.local.json"), "skillOverrides", func(o *object) error {
			return o.set(r.Target, "off")
		})
		if err != nil {
			return nil, err
		}
		return &Plan{Title: fmt.Sprintf("Switch off %s for every project", r.Target), Changes: []Change{ch}}, nil
	case "on":
		plan := &Plan{Title: fmt.Sprintf("Switch %s back on", r.Target)}
		for _, l := range c.SettingsLayers {
			if !l.Exists || l.Ignored != "" || l.Scope == claude.ScopeManaged {
				continue
			}
			obj, _ := l.Values["skillOverrides"].(map[string]any)
			if obj[r.Target] != "off" {
				continue
			}
			ch, err := editSettings(ws, l.Path, "skillOverrides", func(o *object) error {
				o.remove(r.Target)
				return nil
			})
			if err != nil {
				return nil, err
			}
			plan.Changes = append(plan.Changes, ch)
		}
		if len(plan.Changes) == 0 {
			return nil, fmt.Errorf("%s is not switched off in any settings file dotclaude may change: %w", r.Target, ErrNotApplicable)
		}
		return plan, nil
	}
	return nil, fmt.Errorf("value must be on or off, not %q", r.Value)
}

// pluginEnabled sets enabledPlugins in the settings file that currently
// decides the plugin, or in the user settings when none does.
func pluginEnabled(ws workspace, c *claude.Config, p claude.Paths, r Request) (*Plan, error) {
	var plugin *claude.Plugin
	for i := range c.Plugins {
		if c.Plugins[i].ID == r.Target {
			plugin = &c.Plugins[i]
		}
	}
	if plugin == nil {
		return nil, fmt.Errorf("no plugin %q: %w", r.Target, ErrNotApplicable)
	}
	enable := r.Value == "true"
	if !enable && r.Value != "false" {
		return nil, fmt.Errorf("value must be true or false, not %q", r.Value)
	}
	if enable && !plugin.Installed {
		return nil, fmt.Errorf("%s is not installed; install it with /plugin first: %w", r.Target, ErrNotApplicable)
	}
	if plugin.EnabledBy == claude.ScopeManaged {
		return nil, fmt.Errorf("%s is decided by managed settings: %w", r.Target, ErrNotApplicable)
	}
	file := plugin.EnabledIn
	if file == "" {
		file = filepath.Join(p.ConfigDir, "settings.json")
	}
	ch, err := editSettings(ws, file, "enabledPlugins", func(o *object) error {
		return o.set(r.Target, enable)
	})
	if err != nil {
		return nil, err
	}
	verb := "Disable"
	if enable {
		verb = "Enable"
	}
	return &Plan{Title: fmt.Sprintf("%s %s", verb, r.Target), Changes: []Change{ch}}, nil
}

// editSettings changes one object-valued key of a settings file, keeping the
// rest of the file as it is. An object left empty is removed.
func editSettings(ws workspace, path, key string, edit func(*object) error) (Change, error) {
	ch, err := ws.read(path)
	if err != nil {
		return ch, err
	}
	root, err := parseObject([]byte(ch.before))
	if err != nil {
		return ch, fmt.Errorf("%s: %w", path, err)
	}
	child, err := root.child(key)
	if err != nil {
		return ch, fmt.Errorf("%s: %s: %w", path, key, err)
	}
	if err := edit(child); err != nil {
		return ch, err
	}
	if len(child.keys) == 0 {
		root.remove(key)
	} else if err := root.setObject(key, child); err != nil {
		return ch, err
	}
	ch.after = string(root.encode("")) + "\n"
	if ch.after == ch.before {
		return ch, fmt.Errorf("%s already has this value: %w", path, ErrNotApplicable)
	}
	return ch, nil
}

// memoryIndexAdd appends a line for a memory to its store's MEMORY.md, in
// the "- [Title](file.md) — description" form the index uses.
func memoryIndexAdd(ws workspace, c *claude.Config, r Request) (*Plan, error) {
	for _, s := range c.Memory {
		for _, m := range s.Entries {
			if m.Path != r.Target {
				continue
			}
			if s.Index == nil {
				return nil, fmt.Errorf("%s has no MEMORY.md: %w", s.Dir, ErrNotApplicable)
			}
			for _, l := range s.Index.Links {
				if filepath.Clean(l.File) == m.File {
					return nil, fmt.Errorf("%s is already indexed: %w", m.File, ErrNotApplicable)
				}
			}
			ch, err := ws.read(s.Index.Path)
			if err != nil {
				return nil, err
			}
			line := fmt.Sprintf("- [%s](%s)", title(m), m.File)
			if desc := strings.Join(strings.Fields(m.Description), " "); desc != "" {
				line += " — " + desc
			}
			ch.after = ch.before
			if ch.after != "" && !strings.HasSuffix(ch.after, "\n") {
				ch.after += "\n"
			}
			ch.after += line + "\n"
			return &Plan{Title: fmt.Sprintf("List %s in MEMORY.md", m.File), Changes: []Change{ch}}, nil
		}
	}
	return nil, fmt.Errorf("no memory file %s: %w", r.Target, ErrNotApplicable)
}

// title turns a memory's slug into a readable title: "ask-before-tests"
// becomes "Ask before tests".
func title(m claude.MemoryEntry) string {
	t := strings.ReplaceAll(m.Key(), "-", " ")
	t = strings.ReplaceAll(t, "_", " ")
	if t == "" {
		return m.File
	}
	r := []rune(t)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// memoryIndexRemove drops a MEMORY.md line whose link points nowhere.
func memoryIndexRemove(ws workspace, c *claude.Config, r Request) (*Plan, error) {
	for _, s := range c.Memory {
		if s.Index == nil || s.Index.Path != r.Target {
			continue
		}
		for _, l := range s.Index.Links {
			if l.Line != r.Line || l.Exists {
				continue
			}
			ch, err := ws.read(s.Index.Path)
			if err != nil {
				return nil, err
			}
			lines := strings.SplitAfter(ch.before, "\n")
			if r.Line > len(lines) || !strings.Contains(lines[r.Line-1], "("+l.File+")") {
				return nil, fmt.Errorf("line %d no longer links to %s: %w", r.Line, l.File, ErrNotApplicable)
			}
			others := 0
			for _, other := range s.Index.Links {
				if other.Line == r.Line {
					others++
				}
			}
			if others > 1 {
				return nil, fmt.Errorf("line %d also links to other memories; edit it by hand: %w", r.Line, ErrNotApplicable)
			}
			ch.after = strings.Join(append(lines[:r.Line-1:r.Line-1], lines[r.Line:]...), "")
			return &Plan{Title: fmt.Sprintf("Remove the link to %s from MEMORY.md", l.File), Changes: []Change{ch}}, nil
		}
	}
	return nil, fmt.Errorf("no dangling link on line %d of %s: %w", r.Line, r.Target, ErrNotApplicable)
}

// removeSymlink deletes a skill entry that is a symlink to nothing.
func removeSymlink(c *claude.Config, r Request) (*Plan, error) {
	for _, comp := range c.Components {
		if comp.Path != r.Target || !strings.HasPrefix(comp.Error, "dangling symlink") {
			continue
		}
		target, err := os.Readlink(r.Target)
		if err != nil {
			return nil, fmt.Errorf("%s is not a symlink: %w", r.Target, ErrNotApplicable)
		}
		if _, err := os.Stat(r.Target); err == nil {
			return nil, fmt.Errorf("%s resolves again: %w", r.Target, ErrNotApplicable)
		}
		ch := Change{
			Path: r.Target, Op: "remove", linkTarget: target,
			Hash: hash(target, true),
			Diff: fmt.Sprintf("remove symlink %s\n    pointing to %s, which does not exist\n", r.Target, target),
		}
		return &Plan{Title: "Remove dangling symlink " + filepath.Base(r.Target), Changes: []Change{ch}}, nil
	}
	return nil, fmt.Errorf("%s is not a dangling skill symlink: %w", r.Target, ErrNotApplicable)
}

// Hashes returns the path → hash map a client sends back to Apply.
func (p *Plan) Hashes() map[string]string {
	out := map[string]string{}
	for _, ch := range p.Changes {
		out[ch.Path] = ch.Hash
	}
	return out
}
