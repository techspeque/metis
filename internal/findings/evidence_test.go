package findings

import (
	"path/filepath"
	"strings"
	"testing"
)

// A resolution is a claim unless it names the fix. The ledger used to
// accept any note, and bulk closures ("fixed per brief", forty at a time)
// came back as re-raised findings.
func TestResolveNeedsCommitAndEvidence(t *testing.T) {
	s := &Store{}
	id := s.Add("feat-0001", "P2", "security", "issuer query reflected into the startup error")

	for name, args := range map[string][2]string{
		"no commit":   {"", "TestIssuerRefused"},
		"no evidence": {"4e16005", ""},
		"blank":       {"  ", " "},
	} {
		if err := s.Resolve(id, args[0], args[1], "fixed per brief"); err == nil {
			t.Errorf("%s: resolve accepted without evidence", name)
		}
	}
	if s.FindByID(id).Status != "open" {
		t.Fatal("a refused resolution changed the status")
	}

	if err := s.Resolve(id, "4e160050f3a1b2c3", "TestIssuerRefused", "query stripped before the error is built"); err != nil {
		t.Fatal(err)
	}
	f := s.FindByID(id)
	if f.Status != "resolved" || f.Commit != "4e160050f3a1b2c3" || f.Evidence != "TestIssuerRefused" {
		t.Errorf("resolved finding = %+v", f)
	}
	if !strings.Contains(f.ResolvedBy, "4e160050f3a1") || !strings.Contains(f.ResolvedBy, "TestIssuerRefused") {
		t.Errorf("ResolvedBy = %q, want the commit and the evidence readable in one line", f.ResolvedBy)
	}
	if err := s.Resolve(id, "4e16005", "TestIssuerRefused", ""); err == nil {
		t.Error("a resolved finding was resolved again")
	}
}

// A duplicate closes by saying which finding carries it, never with a
// fix hash of its own.
func TestDisposeNamesTheCarrier(t *testing.T) {
	s := &Store{}
	first := s.Add("feat-0001", "P2", "data", "trailing bytes accepted on /v1/things")
	second := s.Add("feat-0002", "P2", "data", "trailing bytes accepted on /v1/things (again)")

	if err := s.Dispose(second, "duplicate", second); err == nil {
		t.Error("a finding was made a duplicate of itself")
	}
	if err := s.Dispose(second, "duplicate", "f-999"); err == nil {
		t.Error("a duplicate of a finding that does not exist was accepted")
	}
	if err := s.Dispose(second, "closed", first); err == nil {
		t.Error("an unknown disposition was accepted")
	}
	if err := s.Dispose(second, "duplicate", first); err != nil {
		t.Fatal(err)
	}
	f := s.FindByID(second)
	if f.Status != "resolved" || f.Disposition != "duplicate" || f.Of != first || f.Commit != "" {
		t.Errorf("disposed finding = %+v", f)
	}
	if s.FindByID(first).Status != "open" {
		t.Error("disposing the duplicate touched the original")
	}
}

// A waiver is a decision, not a fix: severity and text stay as raised,
// and the count reports it on its own line so a pass never reads as a fix.
func TestWaiveKeepsTheFindingAndCountsApart(t *testing.T) {
	s := &Store{}
	id := s.Add("feat-0001", "P1", "performance", "bind rate below the stated floor")
	fixed := s.Add("feat-0001", "P2", "tests", "no regression test")
	if err := s.Resolve(fixed, "abc123def456", "TestRegression", ""); err != nil {
		t.Fatal(err)
	}

	if err := s.Waive(id, &Waiver{Reason: "floor withdrawn with the bench"}); err == nil {
		t.Error("a waiver without an actor was accepted")
	}
	if err := s.Waive(id, &Waiver{By: "human"}); err == nil {
		t.Error("a waiver without a reason was accepted")
	}
	w := Waiver{By: "human", Reason: "floor withdrawn with the bench", Residual: "no measured floor until the performance phase", Owner: "planning", Revisit: "performance phase"}
	if err := s.Waive(id, &w); err != nil {
		t.Fatal(err)
	}
	f := s.FindByID(id)
	if f.Status != "waived" || f.Severity != "P1" || f.Waiver == nil || f.Waiver.Date == "" {
		t.Errorf("waived finding = %+v", f)
	}
	if f.Waiver.Revisit != "performance phase" || f.Waiver.Residual == "" {
		t.Errorf("waiver terms lost: %+v", f.Waiver)
	}
	if err := s.Waive(id, &w); err == nil {
		t.Error("a waived finding was waived again")
	}
	if err := s.Resolve(id, "abc123def456", "TestX", ""); err == nil {
		t.Error("a waived finding was resolved over the waiver")
	}

	st := s.GetStats()
	if st.Waived != 1 || st.ByStatus["waived"] != 1 || st.ByStatus["resolved"] != 1 {
		t.Errorf("stats = %+v, want one waived apart from one resolved", st)
	}
	if !ValidStatus("waived") {
		t.Error("waived is not in the status vocabulary")
	}
	if !f.Closed() || !s.FindByID(fixed).Closed() || (&Finding{Status: "open"}).Closed() {
		t.Error("Closed() does not match the lifecycle")
	}
}

// A correction keeps what was raised and who changed it. Excluding a row
// takes it out of the severity and category counts and nowhere else.
func TestReclassifyKeepsProvenanceAndLabelsTheDenominator(t *testing.T) {
	s := &Store{}
	agg := s.Add("feat-0001", "P3", "", "cycle 3 blocked on f-12..f-15")
	real := s.Add("feat-0001", "P3", "", "BLOCKING: bearer logged on refusal")
	s.Add("feat-0002", "P2", "tests", "flaky test")

	if err := s.Reclassify(real, &Correction{Severity: "P1", Category: "security"}); err == nil {
		t.Error("a correction without provenance was accepted")
	}
	if err := s.Reclassify(real, &Correction{By: "human", Reason: "pre-severity row", Severity: "P9"}); err == nil {
		t.Error("an unknown severity was accepted")
	}
	if err := s.Reclassify(real, &Correction{By: "human", Reason: "nothing", Severity: "P3"}); err == nil {
		t.Error("a correction that changes nothing was accepted")
	}

	if err := s.Reclassify(real, &Correction{By: "human", Reason: "recorded before severities meant anything", Severity: "P1", Category: "security"}); err != nil {
		t.Fatal(err)
	}
	f := s.FindByID(real)
	if f.Severity != "P1" || f.Category != "security" {
		t.Errorf("corrected finding = %+v", f)
	}
	if f.Original == nil || f.Original.Severity != "P3" || f.Original.Category != "" {
		t.Errorf("original classification lost: %+v", f.Original)
	}
	if len(f.Corrections) != 1 || f.Corrections[0].By != "human" || f.Corrections[0].Date == "" {
		t.Errorf("corrections = %+v", f.Corrections)
	}
	// A second correction keeps the first original.
	if err := s.Reclassify(real, &Correction{By: "human", Reason: "P2 on reflection", Severity: "P2"}); err != nil {
		t.Fatal(err)
	}
	if f.Original.Severity != "P3" || len(f.Corrections) != 2 {
		t.Errorf("second correction rewrote history: original %+v, corrections %d", f.Original, len(f.Corrections))
	}

	if err := s.Reclassify(agg, &Correction{By: "human", Reason: "aggregator row", Excluded: true}); err != nil {
		t.Fatal(err)
	}
	st := s.GetStats()
	if st.Total != 3 || st.Counted != 2 || st.Excluded != 1 {
		t.Errorf("stats = total %d counted %d excluded %d", st.Total, st.Counted, st.Excluded)
	}
	if st.BySeverity["P3"] != 0 || st.BySeverity["P2"] != 2 {
		t.Errorf("BySeverity = %v, want the excluded row gone and the corrected one moved", st.BySeverity)
	}
	if st.ByStatus["open"] != 3 {
		t.Errorf("ByStatus = %v, want every row still in the ledger", st.ByStatus)
	}
}

// A follow-on says which earlier finding it continues, so a chain of
// partial fixes reads as one problem. The link is stated, never inferred.
func TestFollowOnChain(t *testing.T) {
	s := &Store{}
	a := s.Add("feat-0001", "P2", "behavior", "cursor not advanced on replay")
	b := s.Add("feat-0001", "P2", "behavior", "cursor advanced, but not persisted")
	c := s.Add("feat-0001", "P2", "behavior", "cursor persisted, but after the ack")

	if err := s.Follow(a, a); err == nil {
		t.Error("a finding followed itself")
	}
	if err := s.Follow(b, "f-404"); err == nil {
		t.Error("a follow-on of a missing finding was accepted")
	}
	if err := s.Follow(b, a); err != nil {
		t.Fatal(err)
	}
	if err := s.Follow(c, b); err != nil {
		t.Fatal(err)
	}
	if got := s.Chain(c); len(got) != 2 || got[0] != b || got[1] != a {
		t.Errorf("Chain(%s) = %v, want [%s %s]", c, got, b, a)
	}
	if got := s.Chain(a); len(got) != 0 {
		t.Errorf("Chain(%s) = %v, want none", a, got)
	}
	if st := s.GetStats(); st.Recurrences != 2 {
		t.Errorf("Recurrences = %d, want 2", st.Recurrences)
	}

	// The link survives a save.
	path := filepath.Join(t.TempDir(), "findings.yaml")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FindByID(c).FollowOn != b {
		t.Error("follow_on did not round-trip")
	}
}

func TestHasLineRef(t *testing.T) {
	for text, want := range map[string]bool{
		"platform/oidc/oidc.go:352-366 renders the issuer verbatim": true,
		"see handlers.go:28":               true,
		"no secret reaches a log or error": false,
		"the file config.yaml is loaded":   false,
	} {
		if HasLineRef(text) != want {
			t.Errorf("HasLineRef(%q) != %v", text, want)
		}
	}
}
