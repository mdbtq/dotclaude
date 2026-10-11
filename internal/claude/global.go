package claude

import (
	"encoding/json"
	"os"
	"time"
)

// globalConfig is the subset of ~/.claude.json dotclaude reads. The file also
// holds the sign-in session and account details; those are never decoded.
type globalConfig struct {
	MCPServers  map[string]json.RawMessage `json:"mcpServers"`
	Projects    map[string]projectState    `json:"projects"`
	SkillUsage  map[string]rawUsage        `json:"skillUsage"`
	PluginUsage map[string]rawUsage        `json:"pluginUsage"`
}

type projectState struct {
	MCPServers             map[string]json.RawMessage `json:"mcpServers"`
	EnabledMcpjsonServers  []string                   `json:"enabledMcpjsonServers"`
	DisabledMcpjsonServers []string                   `json:"disabledMcpjsonServers"`
}

type rawUsage struct {
	UsageCount int   `json:"usageCount"`
	LastUsedAt int64 `json:"lastUsedAt"`
}

// Usage is how often Claude Code recorded a skill or plugin being used.
type Usage struct {
	Count    int       `json:"count"`
	LastUsed time.Time `json:"lastUsed"`
}

func (u rawUsage) usage() *Usage {
	return &Usage{Count: u.UsageCount, LastUsed: time.UnixMilli(u.LastUsedAt).UTC()}
}

func readGlobalConfig(path string) (globalConfig, error) {
	var g globalConfig
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return g, nil
	}
	if err != nil {
		return g, err
	}
	return g, json.Unmarshal(data, &g)
}
