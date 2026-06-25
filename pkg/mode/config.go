package mode

import (
	"fmt"
	"log"
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
	SwarmConfigPath  string `yaml:"swarm_config_path"`
	InferHost        string `yaml:"infer_host"`
	DefaultMaxTokens int    `yaml:"default_max_tokens"`
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
	applyEnvOverrides(&cfg)
	if cfg.DispatchURL == "" || cfg.SlotManagerURL == "" {
		return Config{}, fmt.Errorf("dispatch_url and slot_manager_url required in %s", path)
	}
	return cfg, nil
}

// applyEnvOverrides lets a deployment point a mode at its neighbours (and the inference host)
// without editing the mounted YAML — option B: env-configurable hosts so the same binary is
// deployment-agnostic (127.0.0.1 on the host, host.docker.internal in a container, a service
// DNS name on a cluster). An unset env leaves the YAML value untouched; every applied override
// is logged so a surprising endpoint is never silent. COFISWARM_AGENT_HOST is shared with
// dispatch, which uses the same knob for its per-agent llama/MLX caller.
func applyEnvOverrides(cfg *Config) {
	for _, o := range []struct {
		env string
		dst *string
	}{
		{"COFISWARM_AGENT_HOST", &cfg.InferHost},
		{"COFISWARM_SWARM_CONFIG", &cfg.SwarmConfigPath},
		{"COFISWARM_DISPATCH_URL", &cfg.DispatchURL},
		{"COFISWARM_SLOT_MANAGER_URL", &cfg.SlotManagerURL},
		{"COFISWARM_KVPOOL_URL", &cfg.KVPoolURL},
		{"COFISWARM_AGENT_REGISTRY_URL", &cfg.AgentRegistryURL},
	} {
		if v := os.Getenv(o.env); v != "" {
			*o.dst = v
			log.Printf("mode config: %s overrides %s", o.env, v)
		}
	}
}
