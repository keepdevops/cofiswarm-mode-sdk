package mode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func inferHost(cfg Config) string {
	if cfg.InferHost != "" {
		return cfg.InferHost
	}
	return "127.0.0.1"
}

// LlamaHealthy probes the OpenAI-compatible /v1/models endpoint, which every backend the swarm
// fronts (llama.cpp server, the MLX python server, vLLM, …) exposes — unlike llama.cpp's native
// /props, which the host servers here do not serve. A reachable 200 means the backend can take a
// /v1/chat/completions call.
func LlamaHealthy(host string, port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/v1/models", host, port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func LlamaChat(host string, port int, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 256
	}
	messages := []map[string]string{}
	if systemPrompt != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userPrompt})
	body, _ := json.Marshal(map[string]any{
		"messages": messages, "max_tokens": maxTokens,
		"cache_prompt": true,
		"stop":         []string{"<|im_start|>", "<|eot_id|>"},
	})
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", host, port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("chat %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return out.Choices[0].Message.Content, nil
}

func callAgent(cfg Config, a Agent, userPrompt string, maxTokens int) (string, error) {
	if a.InferBackend() != "llama" || a.Port <= 0 {
		return "", fmt.Errorf("agent %q not llama-http", a.Name)
	}
	host := inferHost(cfg)
	if !LlamaHealthy(host, a.Port) {
		return "", fmt.Errorf("agent %q port %d not healthy", a.Name, a.Port)
	}
	return LlamaChat(host, a.Port, a.SystemPrompt, userPrompt, a.MaxTok(maxTokens))
}
