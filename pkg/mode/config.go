package mode

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Mode             string `yaml:"mode"`
	Listen           string `yaml:"listen"`
	DispatchURL      string `yaml:"dispatch_url"`
	SlotManagerURL   string `yaml:"slot_manager_url"`
	KVPoolURL        string `yaml:"kvpool_url"`
	AgentRegistryURL string `yaml:"agent_registry_url"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.DispatchURL == "" || cfg.SlotManagerURL == "" {
		return Config{}, fmt.Errorf("dispatch_url and slot_manager_url required in %s", path)
	}
	return cfg, nil
}
