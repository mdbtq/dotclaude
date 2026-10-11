package claude

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
)

// MCPServer is a configured MCP server. Only the transport and what it
// connects to are kept: arguments, environment variables and headers often
// carry credentials, so they are never read into this struct.
type MCPServer struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	// Target is the command for stdio servers, or the URL without its query
	// string for remote servers.
	Target     string `json:"target"`
	Source     Source `json:"source"`
	Path       string `json:"path"`
	Status     Status `json:"status"`
	StatusNote string `json:"statusNote,omitempty"`
}

type rawServer struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	URL     string `json:"url"`
}

func parseServers(raw map[string]json.RawMessage, src Source, path string) []MCPServer {
	var out []MCPServer
	for name, data := range raw {
		var r rawServer
		_ = json.Unmarshal(data, &r)
		s := MCPServer{Name: name, Transport: r.Type, Source: src, Path: path, Status: StatusActive}
		switch {
		case r.Command != "":
			s.Target = r.Command
			if s.Transport == "" {
				s.Transport = "stdio"
			}
		case r.URL != "":
			s.Target = stripQuery(r.URL)
			if s.Transport == "" {
				s.Transport = "http"
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// readMCPFile reads a .mcp.json-style file. Plugins sometimes put the servers
// at the top level instead of under "mcpServers"; both are accepted.
func readMCPFile(path string, src Source) []MCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &file) == nil && file.MCPServers != nil {
		return parseServers(file.MCPServers, src, path)
	}
	var flat map[string]json.RawMessage
	if json.Unmarshal(data, &flat) != nil {
		return nil
	}
	return parseServers(flat, src, path)
}

func stripQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}
