package mode

import (
	"context"
	"fmt"
	"strings"

	bmlx "github.com/keepdevops/cofiswarm-backend-mlx/pkg/mlx"
	"github.com/keepdevops/cofiswarm-backend-sdk/pkg/backend"
)

// engineSupported reports whether the modes can drive this agent's engine.
// llama uses the in-package OpenAI/llama path (unchanged); mlx is routed through
// cofiswarm-backend-mlx via the shared InferenceBackend contract. Other engines
// (e.g. vllm) are not yet wired into the modes.
func engineSupported(a Agent) bool {
	switch a.InferBackend() {
	case "llama", "mlx":
		return true
	}
	return false
}

// streamAgent runs a streaming generation for any supported engine and returns
// the full text. The llama path is byte-identical to before; mlx goes through
// the backend package.
func streamAgent(host string, a Agent, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	if a.InferBackend() == "mlx" {
		return streamMLX(host, a, userPrompt, maxTokens, onDelta)
	}
	return LlamaChatStream(host, a.Port, a.SystemPrompt, userPrompt, maxTokens, onDelta)
}

// chatAgent is the non-streaming counterpart (mlx accumulates its own stream).
func chatAgent(host string, a Agent, userPrompt string, maxTokens int) (string, error) {
	if a.InferBackend() == "mlx" {
		return streamMLX(host, a, userPrompt, maxTokens, nil)
	}
	return LlamaChat(host, a.Port, a.SystemPrompt, userPrompt, maxTokens)
}

// streamMLX drives mlx_lm.server through cofiswarm-backend-mlx. MLX is host-local
// (Apple Silicon Metal), so a non-local infer_host is a misconfiguration we fail
// on loudly rather than silently targeting 127.0.0.1 (the backend's fixed host).
func streamMLX(host string, a Agent, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	if host != "" && host != "127.0.0.1" && host != "localhost" {
		return "", fmt.Errorf("agent %q: mlx backend is host-local (Metal); infer_host %q unsupported", a.Name, host)
	}
	b := bmlx.New(a.Port, a.Name, a.SystemPrompt, maxTokens, 0)
	defer b.Close()
	var full strings.Builder
	err := b.GenerateStream(context.Background(),
		backend.GenerateRequest{Prompt: userPrompt, MaxTokens: maxTokens},
		func(c backend.TokenChunk) error {
			if c.Text == "" {
				return nil
			}
			full.WriteString(c.Text)
			if onDelta != nil {
				return onDelta(c.Text)
			}
			return nil
		})
	return full.String(), err
}
