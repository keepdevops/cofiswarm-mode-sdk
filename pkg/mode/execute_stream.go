package mode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	sseSession   = "session"
	sseToken     = "token"
	sseAgentDone = "agent_done"
	sseStage     = "stage"
	sseSelected        = "selected"
	sseSynthesisStart  = "synthesis_start"
	sseMetrics         = "metrics"
	sseDone      = "done"
	sseError     = "error"
)

type sseWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return &sseWriter{w: w, f: f}, nil
}

func (sw *sseWriter) emit(event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	sw.f.Flush()
	return nil
}

func StreamFlat(cfg Config, w http.ResponseWriter, prompt string, modeConfig map[string]interface{}, sessionID string) error {
	sw, err := newSSEWriter(w)
	if err != nil {
		return err
	}
	agents, err := LoadAgents(cfg.SwarmConfigPath)
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"error": err.Error()})
		return err
	}
	agents = agentsByNames(agents, modeConfig)
	rt := parseRAGTarget(modeConfig)
	if sessionID == "" {
		sessionID = "sess-stream"
	}
	_ = sw.emit(sseSession, map[string]string{"session_id": sessionID})
	maxTok := streamMaxTok(cfg, modeConfig)
	host := inferHost(cfg)
	streamed := false
	for _, a := range agents {
		if !engineSupported(a) || a.Port <= 0 {
			continue
		}
		if !LlamaHealthy(host, a.Port) {
			continue
		}
		agent := a.Name
		_, err := streamAgent(host, a, rt.inject(agent, prompt), maxTok, func(delta string) error {
			return sw.emit(sseToken, map[string]string{"agent": agent, "delta": delta})
		})
		if err != nil {
			_ = sw.emit(sseError, map[string]string{"agent": agent, "error": err.Error()})
			continue
		}
		_ = sw.emit(sseAgentDone, map[string]string{"agent": agent})
		streamed = true
		break
	}
	if !streamed {
		_ = sw.emit(sseError, map[string]string{"error": "no healthy agent for stream"})
		return fmt.Errorf("no stream")
	}
	_ = sw.emit(sseMetrics, map[string]any{"stream": true, "calls": 1})
	_, _ = fmt.Fprintf(sw.w, "event: %s\ndata: [DONE]\n\n", sseDone)
	sw.f.Flush()
	return nil
}

func streamMaxTok(cfg Config, modeConfig map[string]interface{}) int {
	maxTok := cfg.DefaultMaxTokens
	if modeConfig != nil {
		if v, ok := modeConfig["max_tokens"].(float64); ok && int(v) > 0 {
			maxTok = int(v)
		}
	}
	if maxTok <= 0 {
		maxTok = 256
	}
	return maxTok
}

func StreamPipeline(cfg Config, w http.ResponseWriter, prompt string, modeConfig map[string]interface{}, sessionID string) error {
	sw, err := newSSEWriter(w)
	if err != nil {
		return err
	}
	agents, err := LoadAgents(cfg.SwarmConfigPath)
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"error": err.Error()})
		return err
	}
	order := pipelineOrder(agents, modeConfig)
	rt := parseRAGTarget(modeConfig)
	llama := []Agent{}
	for _, a := range order {
		if engineSupported(a) && a.Port > 0 {
			llama = append(llama, a)
		}
	}
	if sessionID == "" {
		sessionID = "sess-stream"
	}
	_ = sw.emit(sseSession, map[string]string{"session_id": sessionID})
	maxTok := streamMaxTok(cfg, modeConfig)
	host := inferHost(cfg)
	total := len(llama)
	if total == 0 {
		_ = sw.emit(sseError, map[string]string{"error": "no agents in pipeline"})
		return fmt.Errorf("no pipeline agents")
	}
	prevAgent, prevOut := "", ""
	step := 0
	streamed := 0
	for _, a := range llama {
		if !LlamaHealthy(host, a.Port) {
			_ = sw.emit(sseError, map[string]string{"agent": a.Name, "error": "not healthy"})
			continue
		}
		step++
		_ = sw.emit(sseStage, map[string]any{"step": step, "total": total, "agent": a.Name})
		staged := prompt
		if prevAgent != "" {
			staged = fmt.Sprintf("Original task:\n%s\n\nPrevious agent (%s):\n%s\n\nContinue the workflow.",
				prompt, prevAgent, prevOut)
		}
		var assembled strings.Builder
		_, err := streamAgent(host, a, rt.inject(a.Name, staged), maxTok, func(delta string) error {
			assembled.WriteString(delta)
			return sw.emit(sseToken, map[string]string{"agent": a.Name, "delta": delta})
		})
		if err != nil {
			_ = sw.emit(sseError, map[string]string{"agent": a.Name, "error": err.Error()})
			continue
		}
		prevOut = assembled.String()
		prevAgent = a.Name
		_ = sw.emit(sseAgentDone, map[string]string{"agent": a.Name})
		streamed++
	}
	if streamed == 0 {
		return fmt.Errorf("pipeline stream failed")
	}
	_ = sw.emit(sseMetrics, map[string]any{"stream": true, "calls": streamed})
	_, _ = fmt.Fprintf(sw.w, "event: %s\ndata: [DONE]\n\n", sseDone)
	sw.f.Flush()
	return nil
}

func routerConfig(modeConfig map[string]interface{}) (classifier string, maxSelect int) {
	classifier, maxSelect = "foreman", 2
	if modeConfig != nil {
		if s, ok := modeConfig["classifier"].(string); ok && s != "" {
			classifier = s
		}
		if v, ok := modeConfig["max_select"].(float64); ok && int(v) > 0 {
			maxSelect = int(v)
		}
	}
	return classifier, maxSelect
}

func StreamRouter(cfg Config, w http.ResponseWriter, prompt string, modeConfig map[string]interface{}, sessionID string) error {
	sw, err := newSSEWriter(w)
	if err != nil {
		return err
	}
	agents, err := LoadAgents(cfg.SwarmConfigPath)
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"error": err.Error()})
		return err
	}
	byName := map[string]Agent{}
	for _, a := range agents {
		byName[a.Name] = a
	}
	classifier, maxSelect := routerConfig(modeConfig)
	rt := parseRAGTarget(modeConfig)
	fm, ok := byName[classifier]
	if !ok || !engineSupported(fm) {
		_ = sw.emit(sseError, map[string]string{"error": "classifier unavailable"})
		return fmt.Errorf("no classifier")
	}
	if sessionID == "" {
		sessionID = "sess-stream"
	}
	_ = sw.emit(sseSession, map[string]string{"session_id": sessionID})
	maxTok := streamMaxTok(cfg, modeConfig)
	host := inferHost(cfg)
	if !LlamaHealthy(host, fm.Port) {
		_ = sw.emit(sseError, map[string]string{"error": "classifier not healthy"})
		return fmt.Errorf("classifier down")
	}
	var selection strings.Builder
	_, err = streamAgent(host, fm, buildClassifierPrompt(agents, prompt, maxSelect), maxTok, func(delta string) error {
		selection.WriteString(delta)
		return sw.emit(sseToken, map[string]string{"agent": classifier, "delta": delta})
	})
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"agent": classifier, "error": err.Error()})
		return err
	}
	_ = sw.emit(sseAgentDone, map[string]string{"agent": classifier})
	selected := parseSelected(selection.String(), maxSelect)
	_ = sw.emit(sseSelected, map[string]any{"agents": selected, "classifier": classifier})
	streamed := 1
	for _, name := range selected {
		if name == classifier {
			continue
		}
		a, ok := byName[name]
		if !ok || !engineSupported(a) || !LlamaHealthy(host, a.Port) {
			_ = sw.emit(sseError, map[string]string{"agent": name, "error": "unavailable"})
			continue
		}
		_, err := streamAgent(host, a, rt.inject(name, prompt), maxTok, func(delta string) error {
			return sw.emit(sseToken, map[string]string{"agent": name, "delta": delta})
		})
		if err != nil {
			_ = sw.emit(sseError, map[string]string{"agent": name, "error": err.Error()})
			continue
		}
		_ = sw.emit(sseAgentDone, map[string]string{"agent": name})
		streamed++
	}
	_ = sw.emit(sseMetrics, map[string]any{"stream": true, "calls": streamed, "selected": len(selected)})
	_, _ = fmt.Fprintf(sw.w, "event: %s\ndata: [DONE]\n\n", sseDone)
	sw.f.Flush()
	return nil
}

func cascadeSynthName(modeConfig map[string]interface{}) string {
	if modeConfig != nil {
		if s, ok := modeConfig["synthesizer"].(string); ok && s != "" {
			return s
		}
	}
	return "synthesis"
}

func StreamCascade(cfg Config, w http.ResponseWriter, prompt string, modeConfig map[string]interface{}, sessionID string) error {
	sw, err := newSSEWriter(w)
	if err != nil {
		return err
	}
	allAgents, err := LoadAgents(cfg.SwarmConfigPath)
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"error": err.Error()})
		return err
	}
	workers := agentsByNames(allAgents, modeConfig)
	rt := parseRAGTarget(modeConfig)
	synthName := cascadeSynthName(modeConfig)
	if sessionID == "" {
		sessionID = "sess-stream"
	}
	_ = sw.emit(sseSession, map[string]string{"session_id": sessionID})
	maxTok := streamMaxTok(cfg, modeConfig)
	host := inferHost(cfg)
	outputs := map[string]string{}
	streamed := 0
	for _, a := range workers {
		if a.Name == synthName || !engineSupported(a) || a.Port <= 0 {
			continue
		}
		if !LlamaHealthy(host, a.Port) {
			_ = sw.emit(sseError, map[string]string{"agent": a.Name, "error": "not healthy"})
			continue
		}
		var assembled strings.Builder
		_, err := streamAgent(host, a, rt.inject(a.Name, prompt), maxTok, func(delta string) error {
			assembled.WriteString(delta)
			return sw.emit(sseToken, map[string]string{"agent": a.Name, "delta": delta})
		})
		if err != nil {
			_ = sw.emit(sseError, map[string]string{"agent": a.Name, "error": err.Error()})
			continue
		}
		outputs[a.Name] = assembled.String()
		_ = sw.emit(sseAgentDone, map[string]string{"agent": a.Name})
		streamed++
	}
	if streamed == 0 {
		return fmt.Errorf("cascade: no agent output")
	}
	var synth *Agent
	for i := range allAgents {
		if allAgents[i].Name == synthName {
			synth = &allAgents[i]
			break
		}
	}
	if synth == nil || !engineSupported(*synth) || !LlamaHealthy(host, synth.Port) {
		_ = sw.emit(sseError, map[string]string{"error": "synthesizer unavailable"})
		return fmt.Errorf("no synthesizer")
	}
	_ = sw.emit(sseSynthesisStart, map[string]string{"agent": synthName})
	var b strings.Builder
	b.WriteString("Synthesize these agent outputs into one deliverable.\n\nTask:\n")
	b.WriteString(prompt)
	b.WriteString("\n\nAgent outputs:\n")
	for k, v := range outputs {
		fmt.Fprintf(&b, "\n--- %s ---\n%s\n", k, v)
	}
	_, err = streamAgent(host, *synth, rt.inject(synthName, b.String()), maxTok*2, func(delta string) error {
		return sw.emit(sseToken, map[string]string{"agent": synthName, "delta": delta})
	})
	if err != nil {
		_ = sw.emit(sseError, map[string]string{"agent": synthName, "error": err.Error()})
		return err
	}
	_ = sw.emit(sseAgentDone, map[string]string{"agent": synthName})
	_ = sw.emit(sseMetrics, map[string]any{"stream": true, "calls": streamed + 1, "synthesis": synthName})
	_, _ = fmt.Fprintf(sw.w, "event: %s\ndata: [DONE]\n\n", sseDone)
	sw.f.Flush()
	return nil
}
