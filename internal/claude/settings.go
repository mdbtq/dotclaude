package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// SettingsLayer is one settings file in the precedence stack.
type SettingsLayer struct {
	Scope  Scope  `json:"scope"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
	// Ignored explains why a file that exists is not part of the stack.
	Ignored string         `json:"ignored,omitempty"`
	Values  map[string]any `json:"-"`
}

func readSettings(path string, scope Scope) SettingsLayer {
	l := SettingsLayer{Scope: scope, Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			l.Error = err.Error()
		}
		return l
	}
	l.Exists = true
	if err := json.Unmarshal(data, &l.Values); err != nil {
		l.Error = fmt.Sprintf("invalid JSON: %v", err)
	}
	return l
}

// SettingEntry is one leaf of the merged settings, with where it came from.
type SettingEntry struct {
	Key string `json:"key"`
	// Value is the effective value. Values of sensitive keys are redacted.
	Value any `json:"value"`
	// Scope is the layer whose value is in effect. For lists, which merge
	// across layers, it is the highest layer that contributed.
	Scope Scope `json:"scope"`
	// Overridden lists lower layers that also set the key; for lists these
	// layers contributed entries instead of being overridden.
	Overridden []Scope `json:"overridden,omitempty"`
	Merged     bool    `json:"merged,omitempty"`
}

// mergeSettings combines layers given lowest precedence first. Objects merge
// key by key, lists concatenate without duplicates, and any other value is
// taken from the highest layer that sets it. Claude Code has a few keys with
// their own rules (fallbackModel, modelPicker, availableModels,
// modelSettings); those are shown with the generic rule.
func mergeSettings(layers []SettingsLayer) (map[string]any, []SettingEntry) {
	merged := map[string]any{}
	type origin struct {
		scopes []Scope
		list   bool
	}
	origins := map[string]*origin{}

	var walk func(prefix string, src map[string]any, dst map[string]any, scope Scope)
	walk = func(prefix string, src map[string]any, dst map[string]any, scope Scope) {
		for k, v := range src {
			key := prefix + k
			if obj, ok := v.(map[string]any); ok && len(obj) > 0 && !opaqueKey(key) {
				child, ok := dst[k].(map[string]any)
				if !ok {
					child = map[string]any{}
					dst[k] = child
				}
				walk(key+".", obj, child, scope)
				continue
			}
			o := origins[key]
			if o == nil {
				o = &origin{}
				origins[key] = o
			}
			o.scopes = append(o.scopes, scope)
			if list, ok := v.([]any); ok {
				o.list = true
				dst[k] = appendUnique(asList(dst[k]), list)
				continue
			}
			dst[k] = v
		}
	}
	for _, l := range layers {
		if l.Values != nil {
			walk("", l.Values, merged, l.Scope)
		}
	}

	entries := make([]SettingEntry, 0, len(origins))
	for key, o := range origins {
		last := len(o.scopes) - 1
		e := SettingEntry{Key: key, Value: redact(key, lookup(merged, key)), Scope: o.scopes[last], Merged: o.list && last > 0}
		if last > 0 {
			e.Overridden = o.scopes[:last]
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return merged, entries
}

// opaqueKey marks objects that are shown as one value rather than flattened:
// hooks have their own view, and per-name maps read better whole.
func opaqueKey(key string) bool {
	return key == "hooks" || key == "mcpServers"
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func appendUnique(dst, src []any) []any {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[fmt.Sprint(v)] = true
	}
	for _, v := range src {
		if !seen[fmt.Sprint(v)] {
			seen[fmt.Sprint(v)] = true
			dst = append(dst, v)
		}
	}
	return dst
}

func lookup(m map[string]any, key string) any {
	var cur any = m
	for _, part := range strings.Split(key, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[part]
	}
	return cur
}

var sensitiveKey = regexp.MustCompile(`(?i)(token|secret|passw|api[_-]?key|credential|auth)`)

// Redacted is the placeholder shown instead of a sensitive value.
const Redacted = "[redacted]"

// redact hides values that are likely to be secrets: everything under env,
// and any key whose name suggests a credential.
func redact(key string, v any) any {
	last := key[strings.LastIndex(key, ".")+1:]
	if strings.HasPrefix(key, "env.") || sensitiveKey.MatchString(last) {
		return Redacted
	}
	return v
}

// stringMap reads a settings object of string values, e.g. skillOverrides.
func stringMap(merged map[string]any, key string) map[string]string {
	out := map[string]string{}
	obj, _ := merged[key].(map[string]any)
	for k, v := range obj {
		out[k] = fmt.Sprint(v)
	}
	return out
}
