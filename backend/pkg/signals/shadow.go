package signals

// ShadowSignals reports detection candidates that are recorded as evidence
// but never change the score or the decision. Each one needs its
// false-positive rate reviewed on real pilot traffic before it may score
// (CLAUDE.md §14). It runs once per request: the session check updates
// Redis counters, so calling it twice would double-count.
func ShadowSignals(f RequestFacts) []string {
	found := clientHintSignals(f)
	if h2FamilyMismatch(f.UA, f.HTTP2) {
		found = append(found, "h2_ua_family_mismatch")
	}
	if !f.VerifiedGoodBot && isHostingIP(f.IP) {
		found = append(found, "datacenter_ip")
	}
	return append(found, sessionSignals(f)...)
}
