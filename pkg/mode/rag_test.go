package mode

import (
	"encoding/json"
	"testing"
)

// TestParseRAGTargetWireContract pins the cross-repo seam: cofiswarm-dispatch builds
// mode_config["rag"] = {block: string, agents: []string} and sends it as JSON. After the
// round-trip agents arrives as []interface{}, which parseRAGTarget must still read.
func TestParseRAGTargetWireContract(t *testing.T) {
	dispatchShape := map[string]any{
		"max_tokens": 256,
		"rag": map[string]any{
			"block":  "CONTEXT:\nx\n\n",
			"agents": []string{"programmer", "security"}, // dispatch sends []string
		},
	}
	raw, err := json.Marshal(dispatchShape)
	if err != nil {
		t.Fatal(err)
	}
	var overWire map[string]interface{}
	if err := json.Unmarshal(raw, &overWire); err != nil {
		t.Fatal(err)
	}
	rt := parseRAGTarget(overWire)
	if got := rt.inject("programmer", "do it"); got != "CONTEXT:\nx\n\ndo it" {
		t.Errorf("wire round-trip targeted inject = %q", got)
	}
	if got := rt.inject("architect", "do it"); got != "do it" {
		t.Errorf("wire round-trip untargeted leaked: %q", got)
	}
}

func TestParseRAGTargetAndInject(t *testing.T) {
	t.Run("nil/absent config is a no-op", func(t *testing.T) {
		rt := parseRAGTarget(nil)
		if got := rt.inject("programmer", "q"); got != "q" {
			t.Errorf("nil config injected: %q", got)
		}
		rt = parseRAGTarget(map[string]interface{}{"max_tokens": 256.0})
		if got := rt.inject("programmer", "q"); got != "q" {
			t.Errorf("no rag key injected: %q", got)
		}
	})

	mc := map[string]interface{}{
		"rag": map[string]interface{}{
			"block":  "CONTEXT:\nfoo\n\n",
			"agents": []interface{}{"programmer", "security"},
		},
	}
	rt := parseRAGTarget(mc)

	t.Run("targeted agent gets the block prepended", func(t *testing.T) {
		if got := rt.inject("programmer", "do it"); got != "CONTEXT:\nfoo\n\ndo it" {
			t.Errorf("targeted inject = %q", got)
		}
	})
	t.Run("untargeted agent is untouched", func(t *testing.T) {
		if got := rt.inject("architect", "do it"); got != "do it" {
			t.Errorf("untargeted agent injected: %q", got)
		}
	})
	t.Run("empty block is a no-op even if targeted", func(t *testing.T) {
		rt := parseRAGTarget(map[string]interface{}{
			"rag": map[string]interface{}{"agents": []interface{}{"programmer"}},
		})
		if got := rt.inject("programmer", "q"); got != "q" {
			t.Errorf("empty block injected: %q", got)
		}
	})
}
