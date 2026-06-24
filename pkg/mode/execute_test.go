package mode

import "testing"

func TestParseSelected(t *testing.T) {
	got := parseSelected("foo\nSELECTED: architect, debugger\n", 2)
	if len(got) != 2 || got[0] != "architect" || got[1] != "debugger" {
		t.Fatalf("got %v", got)
	}
}
