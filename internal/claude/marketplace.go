package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Marketplace is a plugin marketplace Claude Code knows, or one an installed
// or enabled plugin names without Claude Code knowing it.
type Marketplace struct {
	Name string `json:"name"`
	// Source is how Claude Code fetches the marketplace: github, git, url,
	// directory or file.
	Source string `json:"source,omitempty"`
	// Location is the repository, URL (without credentials or query) or path
	// the marketplace comes from.
	Location        string    `json:"location,omitempty"`
	InstallLocation string    `json:"installLocation,omitempty"`
	LastUpdated     time.Time `json:"lastUpdated,omitzero"`
	AutoUpdate      bool      `json:"autoUpdate,omitempty"`
	Description     string    `json:"description,omitempty"`
	Owner           string    `json:"owner,omitempty"`
	// Listed is the number of plugins the marketplace offers.
	Listed int    `json:"listed"`
	Known  bool   `json:"known"`
	Error  string `json:"error,omitempty"`
}

type knownMarketplace struct {
	Source struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
		URL    string `json:"url"`
		Path   string `json:"path"`
	} `json:"source"`
	InstallLocation string    `json:"installLocation"`
	LastUpdated     time.Time `json:"lastUpdated"`
	AutoUpdate      bool      `json:"autoUpdate"`
}

type marketplaceManifest struct {
	Description string          `json:"description"`
	Owner       json.RawMessage `json:"owner"`
	Metadata    struct {
		Description string `json:"description"`
	} `json:"metadata"`
	Plugins []struct {
		Name   string          `json:"name"`
		Source json.RawMessage `json:"source"`
	} `json:"plugins"`
}

func readKnownMarketplaces(configDir string) map[string]knownMarketplace {
	var known map[string]knownMarketplace
	if data, err := os.ReadFile(filepath.Join(configDir, "plugins", "known_marketplaces.json")); err == nil {
		_ = json.Unmarshal(data, &known)
	}
	return known
}

func readMarketplaceManifest(installLocation string) (marketplaceManifest, error) {
	var m marketplaceManifest
	data, err := os.ReadFile(filepath.Join(installLocation, ".claude-plugin", "marketplace.json"))
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

// loadMarketplaces lists the marketplaces in known_marketplaces.json, plus
// those that plugins name but that file does not.
func loadMarketplaces(configDir string, plugins []Plugin) []Marketplace {
	known := readKnownMarketplaces(configDir)
	var list []Marketplace
	for name, k := range known {
		m := Marketplace{
			Name: name, Known: true, Source: k.Source.Source,
			InstallLocation: k.InstallLocation, LastUpdated: k.LastUpdated, AutoUpdate: k.AutoUpdate,
		}
		switch {
		case k.Source.Repo != "":
			m.Location = k.Source.Repo
		case k.Source.URL != "":
			m.Location = stripQuery(k.Source.URL)
		default:
			m.Location = k.Source.Path
		}
		if manifest, err := readMarketplaceManifest(k.InstallLocation); err != nil {
			m.Error = "cannot read marketplace.json: " + err.Error()
		} else {
			m.Description = manifest.Description
			if m.Description == "" {
				m.Description = manifest.Metadata.Description
			}
			m.Owner = authorName(manifest.Owner)
			m.Listed = len(manifest.Plugins)
		}
		list = append(list, m)
	}
	seen := map[string]bool{}
	for _, p := range plugins {
		if _, ok := known[p.Marketplace]; ok || seen[p.Marketplace] {
			continue
		}
		seen[p.Marketplace] = true
		list = append(list, Marketplace{Name: p.Marketplace, Error: "not in known_marketplaces.json"})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
