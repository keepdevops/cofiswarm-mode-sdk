package mode

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

func ExecuteMode(cfg Config, modeName, prompt string, modeConfig map[string]interface{}) Envelope {
	agents, err := LoadAgents(cfg.SwarmConfigPath)
	if err != nil {
		return stubWithMeta(modeName, prompt, map[string]interface{}{
			"infer_error": err.Error(), "stub": true,
		})
	}
	maxTok := cfg.DefaultMaxTokens
	if modeConfig != nil {
		if v, ok := modeConfig["max_tokens"].(float64); ok && int(v) > 0 {
			maxTok = int(v)
		}
	}
	if maxTok <= 0 {
		maxTok = 256
	}

	name := normalizeModeName(modeName)
	switch name {
	case "flat":
		return runFlat(cfg, agents, prompt, maxTok, modeConfig)
	case "pipeline":
		return runPipeline(cfg, agents, prompt, maxTok, modeConfig)
	case "cascade":
		return runCascade(cfg, agents, prompt, maxTok, modeConfig)
	case "router":
		return runRouter(cfg, agents, prompt, maxTok, modeConfig)
	default:
		return StubEnvelope(modeName, prompt)
	}
}

func normalizeModeName(m string) string {
	m = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(m)), "mode-")
	return m
}

func stubWithMeta(modeName, prompt string, meta map[string]interface{}) Envelope {
	e := StubEnvelope(modeName, prompt)
	for k, v := range meta {
		e.Meta[k] = v
	}
	return e
}

func agentsByNames(agents []Agent, modeConfig map[string]interface{}) []Agent {
	if modeConfig == nil {
		return agents
	}
	raw, ok := modeConfig["agents"].([]interface{})
	if !ok || len(raw) == 0 {
		return agents
	}
	byName := map[string]Agent{}
	for _, a := range agents {
		byName[a.Name] = a
	}
	out := []Agent{}
	for _, n := range raw {
		if s, ok := n.(string); ok {
			if a, ok := byName[s]; ok {
				out = append(out, a)
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	return agents
}

func runFlat(cfg Config, agents []Agent, prompt string, maxTok int, modeConfig map[string]interface{}) Envelope {
	agents = agentsByNames(agents, modeConfig)
	rt := parseRAGTarget(modeConfig)
	outputs := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	participants := []string{}
	for _, a := range agents {
		if !engineSupported(a) {
			continue
		}
		participants = append(participants, a.Name)
		wg.Add(1)
		go func(ag Agent) {
			defer wg.Done()
			out, err := callAgent(cfg, ag, rt.inject(ag.Name, prompt), maxTok)
			if err != nil {
				out = fmt.Sprintf("[unavailable: %v]", err)
			}
			mu.Lock()
			outputs[ag.Name] = out
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	meta := map[string]interface{}{"infer": true, "participants": participants}
	if len(outputs) == 0 {
		return stubWithMeta("flat", prompt, map[string]interface{}{
			"stub": true, "infer_error": "no agents reachable",
		})
	}
	return Envelope{Mode: "flat", Agents: outputs, Final: nil, Meta: meta}
}

func runPipeline(cfg Config, agents []Agent, prompt string, maxTok int, modeConfig map[string]interface{}) Envelope {
	order := pipelineOrder(agents, modeConfig)
	rt := parseRAGTarget(modeConfig)
	outputs := map[string]string{}
	participants := []string{}
	prevAgent, prevOut := "", ""
	for _, a := range order {
		if !engineSupported(a) {
			continue
		}
		staged := prompt
		if prevAgent != "" {
			staged = fmt.Sprintf("Original task:\n%s\n\nPrevious agent (%s):\n%s\n\nContinue the workflow.",
				prompt, prevAgent, prevOut)
		}
		out, err := callAgent(cfg, a, rt.inject(a.Name, staged), maxTok)
		if err != nil {
			out = fmt.Sprintf("[unavailable: %v]", err)
		}
		outputs[a.Name] = out
		participants = append(participants, a.Name)
		prevAgent, prevOut = a.Name, out
	}
	var final *string
	if prevOut != "" {
		final = &prevOut
	}
	meta := map[string]interface{}{"infer": true, "participants": participants}
	if len(outputs) == 0 {
		return stubWithMeta("pipeline", prompt, map[string]interface{}{
			"stub": true, "infer_error": "no agents reachable",
		})
	}
	return Envelope{Mode: "pipeline", Agents: outputs, Final: final, Meta: meta}
}

func pipelineOrder(agents []Agent, modeConfig map[string]interface{}) []Agent {
	if modeConfig != nil {
		if raw, ok := modeConfig["agents"].([]interface{}); ok {
			byName := map[string]Agent{}
			for _, a := range agents {
				byName[a.Name] = a
			}
			out := []Agent{}
			for _, n := range raw {
				if s, ok := n.(string); ok {
					if a, ok := byName[s]; ok {
						out = append(out, a)
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	out := []Agent{}
	for _, a := range agents {
		if a.Name != "synthesis" {
			out = append(out, a)
		}
	}
	return out
}

func runCascade(cfg Config, agents []Agent, prompt string, maxTok int, modeConfig map[string]interface{}) Envelope {
	synthName := "synthesis"
	if modeConfig != nil {
		if s, ok := modeConfig["synthesizer"].(string); ok && s != "" {
			synthName = s
		}
	}
	rt := parseRAGTarget(modeConfig)
	flat := runFlat(cfg, agents, prompt, maxTok, modeConfig)
	excluded := []string{}
	for name, out := range flat.Agents {
		if strings.HasPrefix(out, "[unavailable:") {
			excluded = append(excluded, name)
		}
	}
	var synth *Agent
	for i := range agents {
		if agents[i].Name == synthName {
			synth = &agents[i]
			break
		}
	}
	meta := flat.Meta
	if meta == nil {
		meta = map[string]interface{}{}
	}
	meta["infer"] = true
	if len(excluded) > 0 {
		meta["excluded"] = excluded
	}
	if synth != nil {
		var b strings.Builder
		b.WriteString("Synthesize these agent outputs into one deliverable.\n\nTask:\n")
		b.WriteString(prompt)
		b.WriteString("\n\nAgent outputs:\n")
		for k, v := range flat.Agents {
			if k == synthName {
				continue
			}
			fmt.Fprintf(&b, "\n--- %s ---\n%s\n", k, v)
		}
		out, err := callAgent(cfg, *synth, rt.inject(synthName, b.String()), maxTok*2)
		if err != nil {
			out = fmt.Sprintf("[unavailable: %v]", err)
		}
		flat.Agents[synthName] = out
		final := out
		flat.Final = &final
	}
	flat.Mode = "cascade"
	flat.Meta = meta
	return flat
}

func runRouter(cfg Config, agents []Agent, prompt string, maxTok int, modeConfig map[string]interface{}) Envelope {
	classifier := "foreman"
	maxSelect := 2
	if modeConfig != nil {
		if s, ok := modeConfig["classifier"].(string); ok && s != "" {
			classifier = s
		}
		if v, ok := modeConfig["max_select"].(float64); ok && int(v) > 0 {
			maxSelect = int(v)
		}
	}
	byName := map[string]Agent{}
	for _, a := range agents {
		byName[a.Name] = a
	}
	fm, ok := byName[classifier]
	if !ok {
		return stubWithMeta("router", prompt, map[string]interface{}{
			"stub": true, "infer_error": "classifier not in roster",
		})
	}
	rt := parseRAGTarget(modeConfig)
	routePrompt := buildClassifierPrompt(agents, prompt, maxSelect)
	selection, err := callAgent(cfg, fm, routePrompt, maxTok)
	if err != nil {
		return stubWithMeta("router", prompt, map[string]interface{}{
			"stub": true, "infer_error": err.Error(),
		})
	}
	selected := parseSelected(selection, maxSelect)
	outputs := map[string]string{classifier: selection}
	for _, name := range selected {
		if name == classifier {
			continue
		}
		a, ok := byName[name]
		if !ok {
			continue
		}
		out, err := callAgent(cfg, a, rt.inject(name, prompt), maxTok)
		if err != nil {
			out = fmt.Sprintf("[unavailable: %v]", err)
		}
		outputs[name] = out
	}
	meta := map[string]interface{}{
		"infer": true, "classifier": classifier, "selected": selected,
	}
	return Envelope{Mode: "router", Agents: outputs, Final: nil, Meta: meta}
}

func buildClassifierPrompt(agents []Agent, task string, maxSelect int) string {
	var b strings.Builder
	b.WriteString(task)
	b.WriteString("\n\nEnd with EXACTLY one line:\nSELECTED: <comma-separated role names, max ")
	fmt.Fprintf(&b, "%d>", maxSelect)
	return b.String()
}

var selectedRE = regexp.MustCompile(`(?im)SELECTED:\s*([^\n]+)`)

func parseSelected(text string, maxSelect int) []string {
	m := selectedRE.FindStringSubmatch(text)
	if len(m) < 2 {
		return nil
	}
	parts := strings.Split(m[1], ",")
	out := []string{}
	for _, p := range parts {
		n := strings.TrimSpace(p)
		if n == "" {
			continue
		}
		out = append(out, n)
		if len(out) >= maxSelect {
			break
		}
	}
	return out
}
