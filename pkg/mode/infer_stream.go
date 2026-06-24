package mode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func LlamaChatStream(host string, port int, systemPrompt, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 256
	}
	messages := []map[string]string{}
	if systemPrompt != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userPrompt})
	body, _ := json.Marshal(map[string]any{
		"messages": messages, "max_tokens": maxTokens, "stream": true,
		"stream_options": map[string]any{"include_usage": true},
		"cache_prompt":   true,
	})
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", host, port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("stream %d: %s", resp.StatusCode, string(raw))
	}
	var full strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		for _, c := range chunk.Choices {
			d := c.Delta.Content
			if d == "" {
				continue
			}
			full.WriteString(d)
			if onDelta != nil {
				if err := onDelta(d); err != nil {
					return full.String(), err
				}
			}
		}
	}
	return full.String(), sc.Err()
}
