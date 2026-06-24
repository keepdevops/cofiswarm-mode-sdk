package mode

// ragTarget carries the per-request RAG context that cofiswarm-dispatch forwards via
// mode_config["rag"]: a rendered context block and the set of agent names it should be
// injected for (the roster's use_rag agents, plus any request-level rag_agents). The mode
// prepends the block to those agents' user prompts so per-agent RAG targeting actually
// reaches inference — the downstream half of dispatch's mergeRosterRAG.
type ragTarget struct {
	block  string
	agents map[string]bool
}

// parseRAGTarget extracts the RAG block + targeted agents from mode_config. Returns a zero
// value (no-op) when dispatch forwarded nothing, so non-RAG runs are unaffected.
func parseRAGTarget(modeConfig map[string]interface{}) ragTarget {
	rt := ragTarget{}
	if modeConfig == nil {
		return rt
	}
	raw, ok := modeConfig["rag"].(map[string]interface{})
	if !ok {
		return rt
	}
	if b, ok := raw["block"].(string); ok {
		rt.block = b
	}
	if as, ok := raw["agents"].([]interface{}); ok {
		rt.agents = make(map[string]bool, len(as))
		for _, n := range as {
			if s, ok := n.(string); ok {
				rt.agents[s] = true
			}
		}
	}
	return rt
}

// inject prepends the RAG context block to userPrompt when name is a targeted agent. It is a
// no-op when no block was forwarded or the agent isn't targeted.
func (rt ragTarget) inject(name, userPrompt string) string {
	if rt.block == "" || !rt.agents[name] {
		return userPrompt
	}
	return rt.block + userPrompt
}
