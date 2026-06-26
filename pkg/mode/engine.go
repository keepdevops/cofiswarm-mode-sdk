package mode

import (
	"context"
	"fmt"
	"strings"

	bmlx "github.com/keepdevops/cofiswarm-backend-mlx/pkg/mlx"
	"github.com/keepdevops/cofiswarm-backend-sdk/pkg/backend"
	bvllm "github.com/keepdevops/cofiswarm-backend-vllm/pkg/vllm"
)

// engineSupported reports whether the modes can drive this agent's engine.
// llama uses the in-package OpenAI/llama path (unchanged); mlx and vllm are
// routed through their cofiswarm-backend-* packages via the shared
// InferenceBackend contract.
func engineSupported(a Agent) bool {
	switch a.InferBackend() {
	case "llama", "mlx", "vllm":
		return true
	}
	return false
}

// streamAgent runs a streaming generation for any supported engine and returns
// the full text. The llama path is byte-identical to before; mlx/vllm go through
// the backend packages.
func streamAgent(host string, a Agent, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	switch a.InferBackend() {
	case "mlx":
		return streamMLX(host, a, userPrompt, maxTokens, onDelta)
	case "vllm":
		return streamVLLM(host, a, userPrompt, maxTokens, onDelta)
	default:
		return LlamaChatStream(host, a.Port, a.SystemPrompt, userPrompt, maxTokens, onDelta)
	}
}

// chatAgent is the non-streaming counterpart (mlx/vllm accumulate their stream).
func chatAgent(host string, a Agent, userPrompt string, maxTokens int) (string, error) {
	switch a.InferBackend() {
	case "mlx":
		return streamMLX(host, a, userPrompt, maxTokens, nil)
	case "vllm":
		return streamVLLM(host, a, userPrompt, maxTokens, nil)
	default:
		return LlamaChat(host, a.Port, a.SystemPrompt, userPrompt, maxTokens)
	}
}

// streamMLX drives mlx_lm.server through cofiswarm-backend-mlx. MLX is host-local
// (Apple Silicon Metal), so a non-local infer_host is a misconfiguration we fail
// on loudly rather than silently targeting 127.0.0.1 (the backend's fixed host).
func streamMLX(host string, a Agent, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	if host != "" && host != "127.0.0.1" && host != "localhost" {
		return "", fmt.Errorf("agent %q: mlx backend is host-local (Metal); infer_host %q unsupported", a.Name, host)
	}
	return driveBackend(bmlx.New(a.Port, a.Name, a.SystemPrompt, maxTokens, 0), userPrompt, maxTokens, onDelta)
}

// streamVLLM drives a vLLM OpenAI-compatible server through cofiswarm-backend-vllm.
// vLLM requires a model id; an agent without one fails loudly.
func streamVLLM(host string, a Agent, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
	if a.Model == "" {
		return "", fmt.Errorf("agent %q: vllm requires a model (set agent.model in the roster)", a.Name)
	}
	return driveBackend(bvllm.NewBackend(host, a.Port, a.Model, a.Name, a.SystemPrompt, maxTokens, 0), userPrompt, maxTokens, onDelta)
}

// driveBackend runs a streaming generation against any InferenceBackend, relaying
// content deltas to onDelta and returning the accumulated text.
func driveBackend(b backend.InferenceBackend, userPrompt string, maxTokens int, onDelta func(string) error) (string, error) {
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
