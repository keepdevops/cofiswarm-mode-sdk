package mode

import (
	"encoding/json"
	"os"
)

type Agent struct {
	Name         string `json:"name"`
	Port         int    `json:"port"`
	Engine       string `json:"engine"`
	Backend      string `json:"backend"`
	SystemPrompt string `json:"system_prompt"`
	MaxTokens    int    `json:"max_tokens"`
}

func LoadAgents(path string) ([]Agent, error) {
	if path == "" {
		if v := os.Getenv("COFISWARM_SWARM_CONFIG"); v != "" {
			path = v
		}
	}
	if path == "" {
		path = "/etc/cofiswarm/config/swarm-config.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Agents []Agent `json:"agents"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return doc.Agents, nil
}

func (a Agent) InferBackend() string {
	if a.Backend != "" {
		return a.Backend
	}
	if a.Engine != "" {
		return a.Engine
	}
	return "llama"
}

func (a Agent) MaxTok(defaultTok int) int {
	if a.MaxTokens > 0 {
		return a.MaxTokens
	}
	return defaultTok
}
