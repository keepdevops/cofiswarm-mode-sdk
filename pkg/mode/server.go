package mode

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Server struct {
	name string
	cfg  Config
}

func NewServer(name, configPath string) (*Server, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	if cfg.Mode == "" {
		cfg.Mode = name
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8020"
	}
	return &Server{name: name, cfg: cfg}, nil
}

func (s *Server) Addr() string { return s.cfg.Listen }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/info", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": s.cfg.Mode, "dispatch_url": s.cfg.DispatchURL,
			"slot_manager_url": s.cfg.SlotManagerURL,
		})
	})
	mux.HandleFunc("/v1/execute", s.handleExecute)
	mux.HandleFunc("/v1/execute/stream", s.handleExecuteStream)
	return mux
}

func (s *Server) handleExecuteStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req executeRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "empty prompt"})
		return
	}
	sid := r.Header.Get("X-Session-Id")
	name := normalizeModeName(s.cfg.Mode)
	var err error
	switch name {
	case "flat":
		err = StreamFlat(s.cfg, w, req.Prompt, req.ModeConfig, sid)
	case "pipeline":
		err = StreamPipeline(s.cfg, w, req.Prompt, req.ModeConfig, sid)
	case "router":
		err = StreamRouter(s.cfg, w, req.Prompt, req.ModeConfig, sid)
	case "cascade":
		err = StreamCascade(s.cfg, w, req.Prompt, req.ModeConfig, sid)
	default:
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "stream not implemented for mode " + name})
		return
	}
	if err != nil {
		if w.Header().Get("Content-Type") != "text/event-stream" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
	}
}

type executeRequest struct {
	Prompt     string                 `json:"prompt"`
	ModeConfig map[string]interface{} `json:"mode_config"`
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req executeRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "empty prompt"})
		return
	}
	client := &http.Client{}
	if resp, err := client.Get(s.cfg.SlotManagerURL + "/healthz"); err == nil {
		resp.Body.Close()
	}
	env := ExecuteMode(s.cfg, s.cfg.Mode, req.Prompt, req.ModeConfig)
	if env.Meta == nil {
		env.Meta = map[string]interface{}{}
	}
	env.Meta["dispatch_url"] = s.cfg.DispatchURL
	_ = json.NewEncoder(w).Encode(env)
}
