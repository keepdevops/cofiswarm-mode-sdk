package mode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestEngineSupported(t *testing.T) {
	if !engineSupported(Agent{Engine: "llama"}) {
		t.Error("llama should be supported")
	}
	if !engineSupported(Agent{Engine: "mlx"}) {
		t.Error("mlx should be supported")
	}
	if !engineSupported(Agent{Engine: "vllm"}) {
		t.Error("vllm should be supported")
	}
	if !engineSupported(Agent{}) {
		t.Error("default (empty) engine resolves to llama and should be supported")
	}
	if engineSupported(Agent{Engine: "ollama"}) {
		t.Error("ollama is not wired into the modes — should be unsupported")
	}
}

// An mlx agent must be driven through cofiswarm-backend-mlx (which posts to the
// mlx_lm.server /v1/chat/completions SSE), not the in-package llama path.
func TestStreamAgentMLXRoutesThroughBackend(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		for _, tok := range []string{"SH", "IP"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", tok)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	// backend-mlx targets 127.0.0.1:<port>/v1; httptest listens on 127.0.0.1, so
	// passing its port lines the backend up with this fake mlx_lm.server.
	_, portStr, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	a := Agent{Name: "mlx-scout", Engine: "mlx", Port: port, SystemPrompt: "You are Scout."}

	var deltas []string
	full, err := streamAgent("127.0.0.1", a, "rate it", 64, func(d string) error {
		deltas = append(deltas, d)
		return nil
	})
	if err != nil {
		t.Fatalf("streamAgent(mlx): %v", err)
	}
	if full != "SHIP" || strings.Join(deltas, "") != "SHIP" {
		t.Errorf("full=%q deltas=%v, want SHIP", full, deltas)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path=%q, want /v1/chat/completions (mlx backend wire)", gotPath)
	}
}

// MLX is host-local (Metal); a non-local infer_host must fail loudly, not
// silently target 127.0.0.1.
func TestStreamMLXRejectsNonLocalHost(t *testing.T) {
	a := Agent{Name: "mlx-scout", Engine: "mlx", Port: 8083}
	if _, err := streamAgent("host.docker.internal", a, "p", 16, nil); err == nil {
		t.Fatal("expected a loud error for a non-local mlx infer_host")
	}
}

// A vllm agent must be driven through cofiswarm-backend-vllm: it sends the
// required model id + the API-key auth header, and honours the infer_host
// (unlike mlx, vLLM is a shared server reachable off-box).
func TestStreamAgentVLLMRoutesThroughBackend(t *testing.T) {
	var gotPath, gotModel, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "text/event-stream")
		for _, tok := range []string{"SH", "IP"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", tok)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	a := Agent{Name: "programmer", Engine: "vllm", Port: port, Model: "Qwen2.5-7B", SystemPrompt: "code."}

	full, err := streamAgent(host, a, "build it", 64, nil)
	if err != nil {
		t.Fatalf("streamAgent(vllm): %v", err)
	}
	if full != "SHIP" {
		t.Errorf("full=%q, want SHIP", full)
	}
	if gotModel != "Qwen2.5-7B" {
		t.Errorf("model=%q, want Qwen2.5-7B (vLLM requires it)", gotModel)
	}
	if gotAuth != "Bearer EMPTY" {
		t.Errorf("auth=%q, want Bearer EMPTY", gotAuth)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path=%q, want /v1/chat/completions", gotPath)
	}
}

// vLLM requires a model id; an agent without one must fail loudly.
func TestStreamVLLMRequiresModel(t *testing.T) {
	a := Agent{Name: "programmer", Engine: "vllm", Port: 12434} // no Model
	if _, err := streamAgent("127.0.0.1", a, "p", 16, nil); err == nil {
		t.Fatal("expected a loud error when a vllm agent has no model")
	}
}
