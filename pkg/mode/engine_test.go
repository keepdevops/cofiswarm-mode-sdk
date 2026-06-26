package mode

import (
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
	if !engineSupported(Agent{}) {
		t.Error("default (empty) engine resolves to llama and should be supported")
	}
	if engineSupported(Agent{Engine: "vllm"}) {
		t.Error("vllm is not wired into the modes yet — should be unsupported")
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
