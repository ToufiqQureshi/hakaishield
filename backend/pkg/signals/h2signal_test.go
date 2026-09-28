package signals

import "testing"

// The scored check must actually consult the H2 tool DB: if the check
// stops firing, this test fails. Mutation-verified: replacing the check
// body with `return false` makes this test fail.
func TestH2ToolMatchSignalFires(t *testing.T) {
	// Seed a verified-capture fingerprint under this test's control.
	const fp = "9:9;1:1|5||"
	AddKnownToolHTTP2(fp, "mutation-test-tool")

	e := Evaluate(RequestFacts{
		JA4:    "t13d1516h2_8daaf6152771_e5627efa2ab1",
		UA:     "Mozilla/5.0 Chrome/120.0",
		Header: realBrowserHeaders(),
		HTTP2:  fp,
	})

	found := false
	for _, s := range e.Signals {
		if s == "h2_tool_match" {
			found = true
		}
	}
	if !found {
		t.Fatalf("h2_tool_match did not fire for a known tool fingerprint; signals = %v", e.Signals)
	}
	// Weight is 0: evidence only, never score.
	if e.Score != 0 {
		t.Fatalf("h2_tool_match changed Score to %d; it must stay evidence-only at weight 0", e.Score)
	}

	// Bit agrees with the name list.
	bit, ok := FeatureBit("h2_tool_match")
	if !ok {
		t.Fatal("h2_tool_match missing from the feature list")
	}
	if e.Fired&bit == 0 {
		t.Fatal("h2_tool_match fired by name but not by bit; model evidence would disagree with the trail")
	}
}
