package cli

import (
	stdcontext "context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/fsutil"
	"github.com/techspeque/metis/internal/launch"
	"github.com/techspeque/metis/internal/slice"
)

// withLock runs fn under the repository lock. metis auto holds no lock
// while an agent runs (the agent's own metis commands need it), so each of
// its own state changes takes the lock like any other metis process.
func (c *context) withLock(fn func() error) error {
	release, err := fsutil.AcquireLock(filepath.Join(c.repoRoot, ".metis", ".lock"))
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

// parkSlice parks a slice with a human wait, as 'metis wait <id> --reason
// ... --human' would, and commits the ledger.
func parkSlice(ctx *context, id, reason string) error {
	return ctx.withLock(func() error {
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}
		s := l.FindByID(id)
		if s == nil || s.IsDone() {
			return nil
		}
		if err := l.WaitFor(id, reason, time.Time{}, time.Now(), true); err != nil {
			return err
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}
		ctx.commitStateSoft(id, "wait: "+reason, ctx.ledgerPath())
		return nil
	})
}

// humanAgent is the first configured agent whose surface is human.
func humanAgent(cfg *config.Config) string {
	slugs := make([]string, 0, len(cfg.Agents))
	for slug, a := range cfg.Agents {
		if a.Surface == config.SurfaceHuman {
			slugs = append(slugs, slug)
		}
	}
	sort.Strings(slugs)
	if len(slugs) == 0 {
		return ""
	}
	return slugs[0]
}

// escalationPath is where a slice's escalation is written.
func (c *context) escalationPath(id string) string {
	return filepath.Join(c.autoDir(id), "escalation.md")
}

// writeEscalation renders the escalation for a stop on a slice: the rule,
// the slice state, the findings across reviews, the last session's final
// output, and the ways out as commands.
func writeEscalation(ctx *context, st *autoStop, s *slice.Slice, store *findings.Store, branch string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# metis auto stopped: %s\n\n", st.Slice)
	fmt.Fprintf(&b, "- **Rule:** `%s`\n- **Why:** %s\n- **When:** %s\n", st.Rule, st.Detail, time.Now().UTC().Format(time.RFC3339))
	if branch != "" {
		fmt.Fprintf(&b, "- **Branch:** `%s` (left in place)\n", branch)
	}
	if s != nil {
		fmt.Fprintf(&b, "\n## Slice\n\n- %s — %s\n- type %s, risk %s, coder %s, reviewer %s\n- coded %t, reviewed %t, review_cycles %d (cap %d)\n",
			s.ID, s.Title, s.Type, s.Risk, s.Coder, s.Reviewer, s.Coded, s.Reviewed, s.ReviewCycles, maxReviewsFor(ctx.cfg, s))
	}

	var all []findings.Finding
	if store != nil {
		all = store.ForSlice(st.Slice)
	}
	var open []findings.Finding
	if len(all) > 0 {
		b.WriteString("\n## Findings by review\n\n")
		sort.SliceStable(all, func(i, j int) bool { return all[i].Cycle < all[j].Cycle })
		last := -1
		for i := range all {
			f := &all[i]
			if f.Cycle != last {
				if f.Cycle == 0 {
					b.WriteString("\nReview unknown:\n\n")
				} else {
					fmt.Fprintf(&b, "\nReview %d:\n\n", f.Cycle)
				}
				last = f.Cycle
			}
			fmt.Fprintf(&b, "- **%s** [%s/%s, %s] %s\n", f.ID, f.Severity, f.Category, f.Status, f.Finding)
			if f.Status == "open" {
				open = append(open, *f)
			}
		}
		var history []string
		for i := range open {
			var related []string
			for j := range all {
				o := &all[j]
				if o.ID != open[i].ID && o.Cycle < open[i].Cycle && repeats(&open[i], o) {
					related = append(related, fmt.Sprintf("%s (review %d, %s)", o.ID, o.Cycle, o.Status))
				}
			}
			if len(related) > 0 {
				history = append(history, fmt.Sprintf("- %s repeats %s", open[i].ID, strings.Join(related, ", ")))
			}
		}
		if len(history) > 0 {
			b.WriteString("\nHistory across reviews:\n\n" + strings.Join(history, "\n") + "\n")
		}
	}

	if logs := launch.Sessions(ctx.autoDir(st.Slice)); len(logs) > 0 {
		lastLog := logs[len(logs)-1]
		fmt.Fprintf(&b, "\n## Last session\n\n`%s`, final lines:\n\n```\n%s\n```\n", ctx.rel(lastLog), tail(lastLog, 30))
	}

	b.WriteString("\n## Ways out\n\n")
	id := st.Slice
	if st.Park {
		fmt.Fprintf(&b, "Resume unattended once it is settled:\n\n```sh\nmetis wait %s --clear\nmetis auto\n```\n", id)
	} else {
		b.WriteString("Settle it, then run `metis auto` again.\n")
	}
	if len(open) > 0 {
		b.WriteString("\nResolve findings by decision:\n\n```sh\n")
		for i := range open {
			fmt.Fprintf(&b, "metis findings resolve %s --note \"decided: <why>\"\n", open[i].ID)
		}
		b.WriteString("```\n")
	}
	if st.Rule == ruleMaxReviewCycles && s != nil {
		fmt.Fprintf(&b, "\nAllow more reviews for this slice:\n\n```sh\nmetis edit %s --max-review-cycles %d\n```\n", id, s.ReviewCycles+1)
	}
	if s != nil && (s.Coded || s.ReviewCycles > 0) {
		human := humanAgent(ctx.cfg)
		b.WriteString("\nReview it yourself (the human review override):\n\n```sh\n")
		if human == "" {
			human = "human"
			b.WriteString("metis config set agents.human.surface human   # no human agent yet\nmetis config set agents.human.model human\nmetis config set agents.human.label Human\n")
		}
		fmt.Fprintf(&b, "metis edit %s --reviewer %s\n", id, human)
		if !s.Coded {
			fmt.Fprintf(&b, "metis verify --post --slice %s\nmetis commit --flip coded --slice %s\n", id, id)
		}
		fmt.Fprintf(&b, "metis commit --flip reviewed --agent %s --slice %s\nmetis archive\n```\n", human, id)
	}
	if branch != "" {
		fmt.Fprintf(&b, "\nThe slice's work is on `%s`. `metis auto` on that branch resumes it, and merges it once reviewed; to drop it, check out the work branch and delete the branch.\n", branch)
	}

	path := ctx.escalationPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

// tail returns the last n lines of a file.
func tail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// notify runs auto.notify, if set, with {slice}, {rule}, {detail} and
// {escalation} substituted. A notifier failing only warns.
func notify(ctx *context, st *autoStop, escalation string) {
	argv := ctx.cfg.Auto.Notify
	if len(argv) == 0 {
		return
	}
	r := strings.NewReplacer("{slice}", st.Slice, "{rule}", st.Rule, "{detail}", st.Detail, "{escalation}", escalation)
	args := make([]string, len(argv))
	for i, a := range argv {
		args[i] = r.Replace(a)
	}
	c, cancel := stdcontext.WithTimeout(stdcontext.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, args[0], args[1:]...)
	cmd.Dir = ctx.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto.notify failed: %v %s\n", err, strings.TrimSpace(string(out)))
	}
}
