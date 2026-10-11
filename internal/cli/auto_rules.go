package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/brief"
	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/slice"
)

// autoStop is a stop: the slice, the rule that fired, what the person must
// decide, and the exit code. Park says whether the slice is parked with a
// human wait (the stops inside a slice's loop) or left dispatchable.
type autoStop struct {
	Rule   string
	Slice  string
	Detail string
	Exit   int
	Park   bool
}

// Exit codes of metis auto.
const (
	exitAutoOK          = 0 // stopped where it was asked to, or nothing to do
	exitAutoNeedsPerson = 3 // stopped, needs a person
	exitAutoInterrupted = 130
)

func ruleStop(rule string, s *slice.Slice, park bool, format string, args ...any) *autoStop {
	return &autoStop{Rule: rule, Slice: s.ID, Detail: fmt.Sprintf(format, args...), Exit: exitAutoNeedsPerson, Park: park}
}

// maxReviewsFor is the slice's review-cycle cap: its own, else the
// project's.
func maxReviewsFor(cfg *config.Config, s *slice.Slice) int {
	if s.MaxReviewCycles > 0 {
		return s.MaxReviewCycles
	}
	return cfg.Auto.Stop.MaxReviews()
}

// briefContent is the slice's brief at HEAD, else on disk, else "".
func briefContent(ctx *context, id string) string {
	rel := filepath.Join(ctx.cfg.Paths.Briefs, id+".md")
	if data, err := git.FileAtHead(ctx.repoRoot, rel); err == nil {
		return string(data)
	}
	if data, err := os.ReadFile(filepath.Join(ctx.repoRoot, rel)); err == nil {
		return string(data)
	}
	return ""
}

// staticStop checks the rules that read only the ledger, the brief and the
// run log: the ones 'metis next' can report without side effects. mode is
// the slice's effective mode.
func staticStop(ctx *context, s *slice.Slice, mode string, events []autoEvent) *autoStop {
	stop := &ctx.cfg.Auto.Stop
	if mode == config.ModeManual {
		return &autoStop{Rule: ruleManual, Slice: s.ID, Detail: fmt.Sprintf("%s runs manually: drive its sessions with metis handoff / metis launch", s.ID), Exit: exitAutoOK}
	}
	if s.Type == slice.TypeGate && config.On(stop.Gate) {
		return ruleStop(ruleGate, s, false, "%s is a gate: the gate review is a person's (metis handoff %s prints its prompt)", s.ID, s.ID)
	}
	if config.On(stop.Planning) && (s.Plan == "" || s.Type == slice.TypeRecon) {
		why := "it has no plan section"
		if s.Type == slice.TypeRecon {
			why = "it is a planning slice (recon)"
		}
		return ruleStop(rulePlanning, s, false, "%s is not run unattended: %s", s.ID, why)
	}
	if config.On(stop.NeedsHuman) {
		if open := brief.UnansweredDecisions(briefContent(ctx, s.ID)); len(open) > 0 {
			return ruleStop(ruleNeedsHuman, s, true, "the brief has %d unanswered human decision(s): %s", len(open), strings.Join(open, "; "))
		}
	}
	if max := maxReviewsFor(ctx.cfg, s); s.ReviewCycles >= max {
		return ruleStop(ruleMaxReviewCycles, s, true, "%d review(s) blocked %s; the next would be review %d (cap %d)", s.ReviewCycles, s.ID, s.ReviewCycles+1, max)
	}
	sessions, wall := sliceBudgetUse(events, s.ID)
	if n := stop.BudgetSessions(); n > 0 && sessions >= n {
		return ruleStop(ruleBudget, s, true, "%s used %d session(s) since its last stop (budget %d)", s.ID, sessions, n)
	}
	if b := stop.BudgetWallClock(); wall >= b {
		return ruleStop(ruleBudget, s, true, "%s used %s of sessions since its last stop (budget %s)", s.ID, wall.Round(time.Minute), b)
	}
	return nil
}

// dirtyOutOfScope lists dirty files that neither metis owns nor the
// slice's brief declares. Without a committed brief every dirty file
// outside metis state counts.
func dirtyOutOfScope(ctx *context, s *slice.Slice) ([]string, error) {
	files, err := git.DirtyFiles(ctx.repoRoot)
	if err != nil {
		return nil, err
	}
	owned := brief.ParseOwnedPaths(briefContent(ctx, s.ID))
	var out []string
	for _, f := range files {
		if ctx.cfg.IsManagedPath(f) || (len(owned) > 0 && brief.InScope(f, owned)) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

var findingIDRe = regexp.MustCompile(`\bf-\d+\b`)

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "not": true, "but": true, "with": true,
	"that": true, "this": true, "are": true, "was": true, "has": true, "have": true,
	"its": true, "into": true, "from": true, "still": true, "should": true, "must": true,
	"add": true, "use": true, "raise": true, "re-raise": true, "when": true, "does": true,
}

func words(f *findings.Finding) map[string]bool {
	set := map[string]bool{}
	text := strings.ToLower(f.Finding)
	if f.File != "" {
		text = strings.ReplaceAll(text, strings.ToLower(f.File), " ")
	}
	text = findingIDRe.ReplaceAllString(text, " ")
	for _, w := range strings.FieldsFunc(text, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-'
	}) {
		if len(w) >= 3 && !stopWords[w] {
			set[w] = true
		}
	}
	return set
}

// similarity is the Jaccard index of two findings' significant words.
func similarity(a, b *findings.Finding) float64 {
	wa, wb := words(a), words(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	inter := 0
	for w := range wa {
		if wb[w] {
			inter++
		}
	}
	return float64(inter) / float64(len(wa)+len(wb)-inter)
}

// repeatThreshold is the similarity over which a new finding repeats an
// earlier one about the same file.
const repeatThreshold = 0.5

// repeats reports whether newer re-raises older: it cites older's id, or
// it is about the same file (when both name one) and reads alike.
func repeats(newer, older *findings.Finding) bool {
	for _, id := range findingIDRe.FindAllString(newer.Finding, -1) {
		if id == older.ID {
			return true
		}
	}
	if newer.File != "" && older.File != "" && newer.File != older.File {
		return false
	}
	return similarity(newer, older) >= repeatThreshold
}

// findRepeat returns a new blocking finding that re-raises one from an
// earlier review — raised before the coder's rework claimed its fix.
func findRepeat(all []findings.Finding, opened map[string]bool) (newer, older *findings.Finding) {
	for i := range all {
		n := &all[i]
		if !opened[n.ID] || !n.Blocking() {
			continue
		}
		for j := range all {
			o := &all[j]
			if o.ID == n.ID || opened[o.ID] || !o.Blocking() || o.Cycle == 0 || o.Cycle >= n.Cycle {
				continue
			}
			if repeats(n, o) {
				return n, o
			}
		}
	}
	return nil, nil
}

// blockingIn counts the blocking findings a review raised.
func blockingIn(all []findings.Finding, cycle int) int {
	n := 0
	for i := range all {
		if all[i].Cycle == cycle && all[i].Blocking() {
			n++
		}
	}
	return n
}
