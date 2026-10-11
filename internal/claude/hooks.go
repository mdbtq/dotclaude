package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Hook is one handler registered for a hook event.
type Hook struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
	Type    string `json:"type"`
	// Detail is what the handler runs: the command, prompt or URL.
	Detail  string `json:"detail"`
	Timeout int    `json:"timeout,omitempty"`
	Source  Source `json:"source"`
	// Path is the file the hook is defined in.
	Path string `json:"path"`
	// PluginRoot is the value ${CLAUDE_PLUGIN_ROOT} expands to, for plugin hooks.
	PluginRoot string `json:"pluginRoot,omitempty"`
	Status     Status `json:"status"`
}

type hookGroup struct {
	Matcher string `json:"matcher"`
	Hooks   []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Prompt  string `json:"prompt"`
		URL     string `json:"url"`
		Timeout int    `json:"timeout"`
	} `json:"hooks"`
}

// parseHooks reads a hooks object: event name → list of matcher groups.
func parseHooks(raw any, src Source, path string) ([]Hook, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var events map[string][]hookGroup
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("unexpected hooks structure: %w", err)
	}
	var out []Hook
	for event, groups := range events {
		for _, g := range groups {
			for _, h := range g.Hooks {
				detail := h.Command
				if detail == "" {
					detail = h.Prompt
				}
				if detail == "" {
					detail = h.URL
				}
				out = append(out, Hook{
					Event: event, Matcher: g.Matcher, Type: h.Type, Detail: detail,
					Timeout: h.Timeout, Source: src, Path: path, Status: StatusActive,
				})
			}
		}
	}
	sortHooks(out)
	return out, nil
}

// readHooksFile reads a plugin's hooks.json, which wraps the hooks object in
// a top-level "hooks" key.
func readHooksFile(path string, src Source) ([]Hook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Hooks any `json:"hooks"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return parseHooks(file.Hooks, src, path)
}

func sortHooks(h []Hook) {
	sort.SliceStable(h, func(i, j int) bool {
		if h[i].Event != h[j].Event {
			return h[i].Event < h[j].Event
		}
		return h[i].Matcher < h[j].Matcher
	})
}
