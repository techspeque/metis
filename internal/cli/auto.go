package cli

import (
	stdcontext "context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/launch"
	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/progress"
	"github.com/techspeque/metis/internal/runner"
	"github.com/techspeque/metis/internal/runs"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	autoCmd.Flags().String("until", "", "Stop at: gate, phase, empty, or slice:<id> (default auto.until, else gate)")
	autoCmd.Flags().Int("max-slices", 0, "Stop after this many slices reach reviewed (0: no limit)")
	autoCmd.Flags().Bool("dry-run", false, "Show the next session (slice, role, agent, argv) without running it")
	autoCmd.Flags().String("mode", "", "This run only: manual, step or auto — at the --slice or --phase level when one is named, else the project level")
	autoCmd.Flags().Bool("step", false, "This run only: --mode step at the project level")
	autoCmd.Flags().String("slice", "", "Limit the run to this slice")
	autoCmd.Flags().String("phase", "", "Limit the run to this phase")
	autoCmd.Flags().Bool("status", false, "Show the last run: slices done, sessions, cycles per slice, the stop")
	rootCmd.AddCommand(autoCmd)
}

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Drive coder and reviewer sessions until a slice needs a person",
	Long: `Takes every dispatchable slice through coding and the review loop, starting
for each session the agent and model the slice names (metis launch), until
a stop rule fires. The ledger is the only state: interrupting it leaves a
repository a person can continue by hand, and metis auto resumes from it.

How far a run goes is the mode (metis mode): manual stops before the slice,
step takes one slice to reviewed and archived, auto continues. The
narrowest setting wins: slice, phase, risk, project, then step.

Stops are results. A stop inside a slice's loop parks the slice
('metis wait --human'), writes .metis/runs/<slice>/auto/escalation.md with
the ways out as commands, runs auto.notify, and exits 3. step_done, manual
and a reached --until exit 0.

A session stopped by its agent's usage or rate limit is not a failure: the
slice waits ('metis wait --until', no --human) and the run sleeps until
the limit resets, then runs the session again. Waiting longer than
auto.rate_limit.max_wait stops the run (rate_limited, exit 3) with the
wait in place; metis auto after it carries on. A launcher exit code in the
agent's transient_exit_codes is waited out the same way.

A session of an agent with remote: true is wrapped in a sync with origin
(metis launch); a branch origin rejects, or one that cannot fast-forward
afterwards, stops the run (sync, exit 3) with nothing rewritten. A slice
that ran one is pushed when it lands: the work branch, and origin's
auto/<slice-id> deleted.

Each session is one line of .metis/runs/auto.jsonl; --status summarises the
last run. With auto.branch: slice (the default) a slice's work lands on
auto/<slice-id>, fast-forward merged into the work branch once reviewed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		if s, _ := cmd.Flags().GetBool("status"); s {
			return printAutoStatus(cmd, ctx)
		}
		opts, err := autoOptionsFrom(cmd, ctx)
		if err != nil {
			return err
		}
		// Agents run metis themselves: they need the repository lock.
		releaseRepoLock()

		sigCtx, stopSignals := signal.NotifyContext(stdcontext.Background(), os.Interrupt, syscall.SIGTERM)
		defer stopSignals()

		r := newAutoRun(ctx, opts, cmd.OutOrStdout())
		st := r.loop(sigCtx)
		return r.finish(cmd, st)
	},
}

type autoOptions struct {
	until     string
	maxSlices int
	dryRun    bool
	slice     string
	phase     string
	ov        modeOverrides
	modeLabel string
}

func autoOptionsFrom(cmd *cobra.Command, ctx *context) (*autoOptions, error) {
	o := &autoOptions{ov: modeOverrides{slice: map[string]string{}, phase: map[string]string{}}}
	o.until, _ = cmd.Flags().GetString("until")
	if o.until == "" {
		o.until = ctx.cfg.Auto.UntilTarget()
	}
	if err := config.ValidUntil(o.until); err != nil {
		return nil, fmt.Errorf("--until: %w", err)
	}
	o.maxSlices, _ = cmd.Flags().GetInt("max-slices")
	o.dryRun, _ = cmd.Flags().GetBool("dry-run")
	o.slice, _ = cmd.Flags().GetString("slice")
	o.phase, _ = cmd.Flags().GetString("phase")
	mode, _ := cmd.Flags().GetString("mode")
	if step, _ := cmd.Flags().GetBool("step"); step {
		if mode != "" && mode != config.ModeStep {
			return nil, fmt.Errorf("--step and --mode %s are two choices", mode)
		}
		mode = config.ModeStep
		if o.slice != "" || o.phase != "" {
			return nil, fmt.Errorf("--step sets the project level; with --slice or --phase use --mode step")
		}
	}
	if mode != "" && !config.ValidMode(mode) {
		return nil, fmt.Errorf("--mode %q is not a mode (valid: manual, step, auto)", mode)
	}
	switch {
	case mode == "":
	case o.slice != "":
		o.ov.slice[o.slice] = mode
	case o.phase != "":
		o.ov.phase[o.phase] = mode
	default:
		o.ov.project = mode
	}
	o.modeLabel = mode
	return o, nil
}

// autoRun is one metis auto run.
type autoRun struct {
	ctx      *context
	opts     *autoOptions
	out      io.Writer
	modes    *modeResolver
	runID    string
	done     []string
	phase    string            // the first slice's phase (--until phase)
	inFlight string            // the slice the run is working on
	sliceMod map[string]string // each slice's mode when the run took it
	failures map[string]int    // failed sessions in a row, per slice and role
	idle     map[string]int    // sessions without progress in a row
	prs      map[string]bool   // slices whose pull request is pending
	branch   string            // the auto branch checked out when dispatch ran
}

func newAutoRun(ctx *context, opts *autoOptions, out io.Writer) *autoRun {
	return &autoRun{
		ctx:      ctx,
		opts:     opts,
		out:      out,
		modes:    newModeResolver(ctx, opts.ov),
		runID:    time.Now().UTC().Format("20060102T150405Z"),
		sliceMod: map[string]string{},
		failures: map[string]int{},
		idle:     map[string]int{},
		prs:      map[string]bool{},
	}
}

func (r *autoRun) say(format string, args ...any) {
	_, _ = fmt.Fprintf(r.out, "metis auto: "+format+"\n", args...)
}

func (r *autoRun) loop(sig stdcontext.Context) *autoStop {
	if !r.opts.dryRun {
		r.ctx.appendAutoEvent(&autoEvent{Run: r.runID, Event: "start", Mode: r.opts.modeLabel, Until: r.opts.until})
	}
	if st := r.resumeBranch(); st != nil {
		return st
	}
	for {
		if sig.Err() != nil {
			return &autoStop{Rule: ruleInterrupted, Slice: r.inFlight, Detail: "interrupted", Exit: exitAutoInterrupted}
		}
		if st := r.iterate(sig); st != nil {
			return st
		}
	}
}

// keep narrows dispatch to the run's slice or phase, away from slices
// whose pull request is pending, and to the slice in flight while its
// auto branch is checked out.
func (r *autoRun) keep(s *slice.Slice) bool {
	if r.prs[s.ID] {
		return false
	}
	if r.opts.slice != "" && s.ID != r.opts.slice {
		return false
	}
	if r.opts.phase != "" && progress.PhaseOf(s) != r.opts.phase {
		return false
	}
	if r.branch != "" && r.branch != git.AutoBranch(s.ID) {
		return false
	}
	return true
}

// iterate runs one session, or returns the stop that ends the run.
func (r *autoRun) iterate(sig stdcontext.Context) *autoStop {
	ctx := r.ctx
	l, err := ctx.loadLedger()
	if err != nil {
		return r.errStop("", err)
	}
	r.branch = currentAutoBranch(ctx)
	next := l.NextWhere(r.keep)
	if next == nil {
		return r.nothingToDo(l)
	}
	s := next.Slice

	// A new slice: the run-level bounds apply before it starts.
	if s.ID != r.inFlight {
		if st := r.beforeNewSlice(s); st != nil {
			return st
		}
	}
	mode := r.sliceMode(s)
	events := ctx.readAutoEvents()
	if st := staticStop(ctx, s, mode, events); st != nil {
		return st
	}

	if !r.opts.dryRun {
		if config.On(ctx.cfg.Auto.Stop.DirtyTree) {
			dirty, err := dirtyOutOfScope(ctx, s)
			if err != nil {
				return r.errStop(s.ID, err)
			}
			if len(dirty) > 0 {
				return ruleStop(ruleDirtyTree, s, false, "uncommitted files outside %s's scope: %s", s.ID, strings.Join(dirty, ", "))
			}
		}
		if st := r.enterBranch(s); st != nil {
			return st
		}
		if config.On(ctx.cfg.Auto.Stop.VerifyEnv) && ctx.cfg.Commands.EnvCheck != "" {
			store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))
			if code, err := runner.EnvCheck(ctx.cfg, ctx.repoRoot, s.ID, store); err != nil || code != 0 {
				return ruleStop(ruleVerifyEnv, s, true, "the environment check failed before the session (exit %d): see %s", code, filepath.Join(ctx.cfg.Paths.Runs, s.ID, "env-check.log"))
			}
		}
		// The branch may hold more of the slice than the work branch did.
		if l, err = ctx.loadLedger(); err != nil {
			return r.errStop(s.ID, err)
		}
		if s = l.FindByID(next.Slice.ID); s == nil || s.IsDone() {
			return r.complete(next.Slice.ID)
		}
		// Parked on its branch by an earlier stop: still the person's.
		if s.IsWaiting(time.Now()) {
			return r.waitingStop([]slice.Slice{*s})
		}
	}

	plan, err := planSession(ctx, l, s.ID, "", "", r.branchFor(s.ID))
	if err != nil {
		if le, ok := err.(*launchError); ok {
			return ruleStop(le.rule, s, false, "%s", le.msg)
		}
		return r.errStop(s.ID, err)
	}
	if r.opts.dryRun {
		r.printPlan(plan, mode)
		return &autoStop{Rule: "dry_run", Slice: s.ID, Detail: "dry run: nothing was launched", Exit: exitAutoOK}
	}

	r.inFlight = s.ID
	return r.runSession(sig, s, plan)
}

// beforeNewSlice applies the run-level bounds before a slice the run has
// not worked on yet: --max-slices, --until, and the mode boundary.
func (r *autoRun) beforeNewSlice(s *slice.Slice) *autoStop {
	mode := r.modes.of(s).Mode
	if len(r.done) > 0 {
		if r.opts.maxSlices > 0 && len(r.done) >= r.opts.maxSlices {
			return &autoStop{Rule: ruleMaxSlices, Slice: s.ID, Detail: fmt.Sprintf("%d slice(s) done (--max-slices)", len(r.done)), Exit: exitAutoOK}
		}
		if r.opts.until == "phase" && progress.PhaseOf(s) != r.phase {
			return &autoStop{Rule: ruleUntil, Slice: s.ID, Detail: fmt.Sprintf("phase %s is done; %s starts %s", r.phase, s.ID, progress.PhaseOf(s)), Exit: exitAutoOK}
		}
		if mode == config.ModeStep {
			return &autoStop{Rule: ruleStepDone, Slice: s.ID, Detail: fmt.Sprintf("the next slice, %s, runs step by step: run metis auto again", s.ID), Exit: exitAutoOK}
		}
	} else {
		r.phase = progress.PhaseOf(s)
	}
	if r.opts.until == "gate" && s.Type == slice.TypeGate && !config.On(r.ctx.cfg.Auto.Stop.Gate) {
		return &autoStop{Rule: ruleUntil, Slice: s.ID, Detail: fmt.Sprintf("reached the gate %s (--until gate)", s.ID), Exit: exitAutoOK}
	}
	r.sliceMod[s.ID] = mode
	return nil
}

func (r *autoRun) sliceMode(s *slice.Slice) string {
	if m, ok := r.sliceMod[s.ID]; ok {
		return m
	}
	m := r.modes.of(s).Mode
	r.sliceMod[s.ID] = m
	return m
}

// nothingToDo is the stop when dispatch finds no slice for this run.
func (r *autoRun) nothingToDo(l *ledger.Ledger) *autoStop {
	if r.opts.slice != "" {
		s := l.FindByID(r.opts.slice)
		switch {
		case s == nil && r.isArchived(r.opts.slice), s != nil && s.IsDone():
			return &autoStop{Rule: ruleUntil, Slice: r.opts.slice, Detail: r.opts.slice + " is done", Exit: exitAutoOK}
		case s == nil:
			return &autoStop{Rule: ruleBacklogEmpty, Slice: r.opts.slice, Detail: fmt.Sprintf("slice %s is not in the ledger", r.opts.slice), Exit: exitAutoNeedsPerson}
		case s.IsWaiting(time.Now()):
			return r.waitingStop([]slice.Slice{*s})
		default:
			return &autoStop{Rule: ruleAllWaiting, Slice: s.ID, Detail: fmt.Sprintf("%s is blocked by %s", s.ID, strings.Join(s.BlockedBy, ", ")), Exit: exitAutoNeedsPerson}
		}
	}
	if b := currentAutoBranch(r.ctx); b != "" {
		id := strings.TrimPrefix(b, git.AutoBranchPrefix)
		if s := l.FindByID(id); s != nil && s.IsWaiting(time.Now()) {
			return r.waitingStop([]slice.Slice{*s})
		}
	}
	if waiting := l.WaitingSlices(); len(waiting) > 0 {
		return r.waitingStop(waiting)
	}
	if len(l.PendingSlices()) > 0 {
		return &autoStop{Rule: ruleAllWaiting, Detail: "every pending slice is blocked or outside this run", Exit: exitAutoOK}
	}
	return &autoStop{Rule: ruleBacklogEmpty, Detail: "the backlog is empty", Exit: exitAutoOK}
}

func (r *autoRun) waitingStop(waiting []slice.Slice) *autoStop {
	var parts []string
	exit := exitAutoOK
	for i := range waiting {
		w := waiting[i].Waiting
		label := "waits"
		if w.Human {
			label = "needs you"
			exit = exitAutoNeedsPerson
		}
		part := fmt.Sprintf("%s %s: %s", waiting[i].ID, label, w.Reason)
		if w.Until != "" {
			part += " (until " + w.Until + ")"
		}
		parts = append(parts, part)
	}
	st := &autoStop{Rule: ruleAllWaiting, Detail: strings.Join(parts, "; "), Exit: exit}
	if len(waiting) == 1 {
		st.Slice = waiting[0].ID
	}
	return st
}

func (r *autoRun) isArchived(id string) bool {
	a, err := r.ctx.loadArchive()
	if err != nil {
		return false
	}
	for i := range a.Slices {
		if a.Slices[i].ID == id {
			return true
		}
	}
	return false
}

// snapshot is what progress is measured against.
type snapshot struct {
	coded, reviewed, present, waiting bool
	cycles                            int
	commits                           int
	findings                          map[string]string // id -> status
}

func (r *autoRun) snap(id string) snapshot {
	sn := snapshot{findings: map[string]string{}}
	if l, err := r.ctx.loadLedger(); err == nil {
		if s := l.FindByID(id); s != nil {
			sn.present, sn.coded, sn.reviewed, sn.cycles = true, s.Coded, s.Reviewed, s.ReviewCycles
			sn.waiting = s.IsWaiting(time.Now())
		}
	}
	if commits, err := git.SliceCommits(r.ctx.repoRoot, id); err == nil {
		sn.commits = len(commits)
	}
	if store, err := r.loadFindings(); err == nil {
		all := store.ForSlice(id)
		for i := range all {
			sn.findings[all[i].ID] = all[i].Status
		}
	}
	return sn
}

func (r *autoRun) loadFindings() (*findings.Store, error) {
	return findings.Load(filepath.Join(r.ctx.repoRoot, r.ctx.cfg.Paths.Findings))
}

// runSession launches one session and applies the rules that read its
// result.
func (r *autoRun) runSession(sig stdcontext.Context, s *slice.Slice, plan *sessionPlan) *autoStop {
	ctx := r.ctx
	id := s.ID
	before := r.snap(id)
	start := time.Now()
	r.say("session %d on %s: %s %s (%s), log %s", plan.Session, id, strings.ToLower(plan.Handoff.Role), plan.Handoff.AgentSlug, plan.Handoff.Template, ctx.rel(plan.LogPath))
	res, err := plan.run(sig, ctx, nil)
	var syncErr *syncError
	switch {
	case errors.As(err, &syncErr):
		// The remote's state is unknown: the ledger here says nothing
		// about the session. Not parked: a park would be one more commit
		// origin does not have.
		if sig.Err() == nil {
			return ruleStop(ruleSync, s, false, "%s", syncErr.msg)
		}
	case err != nil:
		return ruleStop(ruleSessionFailed, s, true, "the session could not start: %v", err)
	}
	if sig.Err() != nil {
		return &autoStop{Rule: ruleInterrupted, Slice: id, Detail: "interrupted during session " + strconv.Itoa(plan.Session), Exit: exitAutoInterrupted}
	}

	after := r.snap(id)
	archived := !after.present && r.isArchived(id)
	var opened, resolved []string
	for fid, status := range after.findings {
		was, ok := before.findings[fid]
		if !ok {
			opened = append(opened, fid)
		} else if !closedStatus(was) && closedStatus(status) {
			resolved = append(resolved, fid)
		}
	}
	sort.Strings(opened)
	sort.Strings(resolved)

	outcome := "none"
	switch {
	case archived || after.reviewed:
		outcome = "reviewed"
	case after.cycles > before.cycles:
		outcome = "blocked"
	case after.coded && !before.coded:
		outcome = "coded"
	case after.waiting && !before.waiting:
		outcome = "waiting"
	case after.commits > before.commits || len(opened) > 0 || len(resolved) > 0:
		outcome = "commits"
	}
	progressed := outcome != "none"
	exitCode := res.ExitCode
	// A failed session is classified before it is counted: one the agent's
	// usage or rate limit stopped is waited out, not charged as a failure.
	// One whose launcher exited with a transient code is waited out the
	// same way. Either that made progress is progress: the next session
	// meets the limit, or the launcher's trouble, and waits.
	limited, transient, reset := false, false, time.Time{}
	if res.Failed() && !progressed {
		code := res.ExitCode
		if res.TimedOut {
			code = -1 // the exit code is the kill's, not the agent's
		}
		limited, reset = launch.Limited(&plan.Agent, code, plan.LogPath, time.Now())
		transient = !limited && plan.Agent.Transient(code)
	}
	ctx.appendAutoEvent(&autoEvent{
		Run: r.runID, Event: "session", Slice: id, Session: plan.Session,
		Role: plan.Handoff.Role, Agent: plan.Handoff.AgentSlug, Model: plan.Agent.Model, Template: plan.Handoff.Template,
		DurationSeconds: res.Duration.Round(time.Second).Seconds(), ExitCode: &exitCode, TimedOut: res.TimedOut,
		Outcome: outcome, FindingsOpened: opened, FindingsResolved: resolved, Log: ctx.rel(plan.LogPath),
		RateLimited: limited, Transient: transient,
	})
	r.say("session %d ended: exit %d, %s, outcome %s", plan.Session, res.ExitCode, res.Duration.Round(time.Second), outcome)

	stop := &ctx.cfg.Auto.Stop
	l, _ := ctx.loadLedger()
	var cur *slice.Slice
	if l != nil {
		cur = l.FindByID(id)
	}
	if cur == nil {
		cur = s
	}

	// An agent asked for a person: the slice is parked already.
	if cur.Waiting != nil && cur.Waiting.Human && cur.IsWaiting(time.Now()) && config.On(stop.NeedsHuman) {
		return ruleStop(ruleNeedsHuman, cur, false, "the %s asked for a decision: %s", strings.ToLower(plan.Handoff.Role), cur.Waiting.Reason)
	}

	if len(opened) > 0 {
		store, err := r.loadFindings()
		if err == nil {
			all := store.ForSlice(id)
			openedSet := map[string]bool{}
			for _, fid := range opened {
				openedSet[fid] = true
			}
			if config.On(stop.P1) {
				for i := range all {
					if openedSet[all[i].ID] && all[i].Severity == "P1" {
						return ruleStop(ruleP1, cur, true, "P1 finding %s: %s", all[i].ID, all[i].Finding)
					}
				}
			}
			if config.On(stop.RepeatFinding) {
				if n, o := findRepeat(all, openedSet); n != nil {
					return ruleStop(ruleRepeatFinding, cur, true, "%s (review %d) re-raises %s (review %d) after the rework that claimed its fix", n.ID, n.Cycle, o.ID, o.Cycle)
				}
			}
			if config.On(stop.GrowingFindings) && after.cycles > before.cycles && after.cycles >= 2 {
				if now, prev := blockingIn(all, after.cycles), blockingIn(all, after.cycles-1); now > prev {
					return ruleStop(ruleGrowingFindings, cur, true, "review %d opened %d blocking finding(s), more than review %d's %d", after.cycles, now, after.cycles-1, prev)
				}
			}
		}
	}

	if config.On(stop.VerifyEnv) {
		if name := envFailureSince(ctx, id, start); name != "" {
			return ruleStop(ruleVerifyEnv, cur, true, "the environment check failed during the session (metis verify exit 2): see %s", filepath.Join(ctx.cfg.Paths.Runs, id, name+".log"))
		}
	}

	if config.On(stop.Scope) && plan.Handoff.Role == string(slice.RoleCoder) && after.coded && !before.coded {
		if commits, err := git.SliceCommits(ctx.repoRoot, id); err == nil {
			if report := auditSlice(ctx, id, commits); !report.OK {
				return ruleStop(ruleScope, cur, true, "metis log %s --validate fails after the coder session: %s", id, auditFailureReason(&report))
			}
		}
	}

	key := id + "/" + plan.Handoff.Role
	switch {
	case outcome == "reviewed":
		delete(r.failures, key)
		delete(r.idle, key)
		return r.complete(id)
	case limited:
		return r.waitOutLimit(sig, cur, plan, reset, rateLimitReason, "is rate limited")
	case transient:
		return r.waitOutLimit(sig, cur, plan, time.Time{}, transientReason, fmt.Sprintf("exited %d, one of its transient_exit_codes", res.ExitCode))
	case res.Failed() && !progressed:
		r.failures[key]++
		if r.failures[key] > stop.Retries() {
			why := fmt.Sprintf("exit %d", res.ExitCode)
			if res.TimedOut {
				why = "timed out after " + plan.Timeout.String()
			}
			return ruleStop(ruleSessionFailed, cur, true, "the %s session failed %d time(s) in a row (%s)", strings.ToLower(plan.Handoff.Role), r.failures[key], why)
		}
		r.say("retrying (%d of %d)", r.failures[key], stop.Retries())
	case !progressed:
		r.idle[key]++
		if r.idle[key] > stop.Retries() {
			return ruleStop(ruleNoProgress, cur, true, "%d %s session(s) in a row made no progress (no flip, block, wait or commit)", r.idle[key], strings.ToLower(plan.Handoff.Role))
		}
		r.say("no progress; retrying (%d of %d)", r.idle[key], stop.Retries())
	default:
		delete(r.failures, key)
		delete(r.idle, key)
	}
	return nil
}

// rateLimitReason is the wait metis auto sets on a slice whose agent is
// rate limited.
const rateLimitReason = "auto: " + ruleRateLimited

// transientReason is the wait metis auto sets on a slice whose agent's
// launcher exited with a transient code.
const transientReason = "auto: transient"

// autoSleep waits until a time or until ctx is done, whichever is first,
// and reports ctx's error. It checks the wall clock in short steps: a
// timer alone runs late on a machine that slept. Tests replace it.
var autoSleep = func(ctx stdcontext.Context, until time.Time) error {
	for {
		d := time.Until(until)
		if d <= 0 {
			return ctx.Err()
		}
		t := time.NewTimer(min(d, 30*time.Second))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// waitOutLimit handles a session the agent's rate limit stopped: the slice
// waits in the ledger until the limit's reset time plus a minute (or, when
// the log names none, a backoff that doubles), the run sleeps until then,
// and the same session runs again. Waiting past auto.rate_limit.max_wait
// stops the run instead, with the wait left in place. A session whose
// launcher exited with a transient code is waited out here too, under its
// own reason: cause says which, after the agent's name.
func (r *autoRun) waitOutLimit(sig stdcontext.Context, s *slice.Slice, plan *sessionPlan, reset time.Time, reason, cause string) *autoStop {
	ctx := r.ctx
	rl := &ctx.cfg.Auto.RateLimit
	role := strings.ToLower(plan.Handoff.Role)
	inRow, waited := sliceWaitUse(ctx.readAutoEvents(), s.ID)
	now := time.Now()
	until, why := now.Add(rl.BackoffAfter(inRow)), "backoff"
	if !reset.IsZero() {
		until, why = reset.Add(time.Minute), "its log names the reset time"
	}
	// The ledger keeps whole seconds.
	until = until.Add(time.Second - 1).Truncate(time.Second)
	wait := until.Sub(now)
	stamp := until.UTC().Format(time.RFC3339)

	if err := parkSlice(ctx, s.ID, reason, until); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark %s waiting: %v\n", s.ID, err)
	}
	if limit := rl.MaxWaitTime(); waited+wait > limit {
		return ruleStop(ruleRateLimited, s, false, "the %s (%s) %s: waiting until %s would pass auto.rate_limit.max_wait (%s; %s waited); %s waits until then",
			role, plan.Handoff.AgentSlug, cause, stamp, limit, waited.Round(time.Second), s.ID)
	}
	ctx.appendAutoEvent(&autoEvent{Run: r.runID, Event: "wait", Slice: s.ID, Rule: ruleRateLimited, WaitUntil: stamp, DurationSeconds: wait.Round(time.Second).Seconds()})
	r.say("the %s (%s) %s: waiting %s, until %s (%s)", role, plan.Handoff.AgentSlug, cause, wait.Round(time.Second), stamp, why)
	if err := autoSleep(sig, until); err != nil {
		return &autoStop{Rule: ruleInterrupted, Slice: s.ID, Detail: "interrupted while waiting: the " + role + " " + cause + " (until " + stamp + ")", Exit: exitAutoInterrupted}
	}
	if err := unparkSlice(ctx, s.ID, reason); err != nil {
		return r.errStop(s.ID, err)
	}
	return nil
}

// envFailureSince names an env-check run of the slice that failed after t.
// Only the env check speaks for the environment. A verify-* log records the
// verify command's own exit code, and tools such as make exit 2 for any
// failed target, so that code says nothing about the environment.
func envFailureSince(ctx *context, id string, t time.Time) string {
	dir := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs, id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".exit")
		if !ok || (name != "env-check" && !strings.HasPrefix(name, "env-check.")) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().Before(t) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if code, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && code != 0 {
			return name
		}
	}
	return ""
}

// complete finishes a reviewed slice: archive it, land its branch, and
// apply the step and --until bounds.
func (r *autoRun) complete(id string) *autoStop {
	ctx := r.ctx
	if l, err := ctx.loadLedger(); err == nil {
		if s := l.FindByID(id); s != nil && s.IsDone() {
			if err := ctx.withLock(func() error { _, err := archiveDone(ctx); return err }); err != nil {
				return r.errStop(id, err)
			}
		}
	}
	cycles := 0
	if a, err := ctx.loadArchive(); err == nil {
		for i := range a.Slices {
			if a.Slices[i].ID == id {
				cycles = a.Slices[i].ReviewCycles
			}
		}
	}
	remote := r.usedRemote(id)
	if remote && r.branchMode() == config.BranchSlice {
		// The archive commit is not on origin, and git measures a branch
		// with an upstream against it: the merged branch would not delete.
		_ = git.UnsetUpstream(ctx.repoRoot, git.AutoBranch(id))
	}
	merge, st := r.landBranch(id)
	if st != nil {
		return st
	}
	ctx.appendAutoEvent(&autoEvent{Run: r.runID, Event: "done", Slice: id, ReviewCycles: &cycles, Merge: merge})
	r.done = append(r.done, id)
	r.inFlight = ""
	r.say("%s reviewed and archived (%d blocked review(s))%s", id, cycles, map[bool]string{true: "; " + merge, false: ""}[merge != ""])
	if remote {
		if st := r.pushLanded(id); st != nil {
			return st
		}
	}

	if r.sliceMod[id] == config.ModeStep {
		return &autoStop{Rule: ruleStepDone, Slice: id, Detail: id + " is reviewed and archived (step mode): look, then run metis auto again", Exit: exitAutoOK}
	}
	if r.opts.until == "slice:"+id {
		return &autoStop{Rule: ruleUntil, Slice: id, Detail: id + " is done (--until slice:" + id + ")", Exit: exitAutoOK}
	}
	return nil
}

func (r *autoRun) errStop(id string, err error) *autoStop {
	return &autoStop{Rule: "error", Slice: id, Detail: err.Error(), Exit: 1}
}

func (r *autoRun) printPlan(p *sessionPlan, mode string) {
	h := p.Handoff
	_, _ = fmt.Fprintf(r.out, "slice:    %s\nrole:     %s\nagent:    %s (model %s)\ntemplate: %s\nmode:     %s\nbranch:   %s\nlog:      %s\nargv:     %s\n",
		h.Slice, h.Role, h.AgentSlug, p.Agent.Model, h.Template, mode, r.branchFor(h.Slice), r.ctx.rel(p.LogPath), launch.Display(p.Argv, h.Prompt))
}

// finish reports a stop: the run log line, and for a stop on a slice that
// needs a person the park, the escalation and the notifier.
func (r *autoRun) finish(cmd *cobra.Command, st *autoStop) error {
	ctx := r.ctx
	escalation := ""
	if st.Rule == ruleAllWaiting && st.Slice != "" {
		// Still waiting on an earlier stop: point at its escalation
		// rather than replace it.
		if _, err := os.Stat(ctx.escalationPath(st.Slice)); err == nil {
			r.say("see %s", ctx.rel(ctx.escalationPath(st.Slice)))
		}
	} else if st.Exit == exitAutoNeedsPerson && st.Slice != "" {
		var s *slice.Slice
		if l, err := ctx.loadLedger(); err == nil {
			s = l.FindByID(st.Slice)
		}
		if st.Park && s != nil && !s.IsDone() && !s.IsWaiting(time.Now()) {
			if err := parkSlice(ctx, st.Slice, "auto: "+st.Rule, time.Time{}); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not park %s: %v\n", st.Slice, err)
			} else if l, err := ctx.loadLedger(); err == nil {
				s = l.FindByID(st.Slice)
			}
		}
		store, _ := r.loadFindings()
		path, err := writeEscalation(ctx, st, s, store, currentAutoBranch(ctx))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: escalation not written: %v\n", err)
		} else {
			escalation = path
		}
	}
	if st.Rule != "dry_run" {
		exit := st.Exit
		ctx.appendAutoEvent(&autoEvent{Run: r.runID, Event: "stop", Slice: st.Slice, Rule: st.Rule, Detail: st.Detail, Exit: &exit, Escalation: ctx.rel(escalation)})
	}
	if jsonOutput() {
		if err := printJSON(cmd, map[string]any{
			"run": r.runID, "rule": st.Rule, "slice": st.Slice, "detail": st.Detail, "exit": st.Exit,
			"slices_done": nonNil(r.done), "escalation": ctx.rel(escalation),
		}); err != nil {
			return err
		}
	} else {
		r.say("stopped (%s): %s", st.Rule, st.Detail)
		if escalation != "" {
			data, _ := os.ReadFile(escalation)
			_, _ = fmt.Fprintf(r.out, "\n%s\n(written to %s)\n", strings.TrimSpace(string(data)), ctx.rel(escalation))
		}
	}
	if escalation != "" {
		notify(ctx, st, escalation)
	}
	if st.Exit == 0 {
		return nil
	}
	if st.Rule == "error" {
		return fmt.Errorf("%s", st.Detail)
	}
	return exitWithCode(cmd, st.Exit)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// --- branching -----------------------------------------------------------

func (r *autoRun) branchMode() string {
	return r.ctx.cfg.Auto.BranchMode()
}

// branchFor is the branch a slice's sessions work on.
func (r *autoRun) branchFor(id string) string {
	if r.branchMode() == config.BranchOff {
		return r.ctx.cfg.Project.IntegrationBranch
	}
	return git.AutoBranch(id)
}

// resumeBranch handles a run that starts on an auto branch: a slice that
// is reviewed there is landed; otherwise the run resumes that slice. It
// also finds slices whose pull request is pending.
func (r *autoRun) resumeBranch() *autoStop {
	ctx := r.ctx
	if r.branchMode() == config.BranchPR {
		for _, b := range git.ListBranches(ctx.repoRoot, git.AutoBranchPrefix) {
			id := strings.TrimPrefix(b, git.AutoBranchPrefix)
			if doneAt(ctx, b, id) {
				r.prs[id] = true
			}
		}
	}
	b := currentAutoBranch(ctx)
	if b == "" || r.branchMode() == config.BranchOff {
		cur, err := git.CurrentBranch(ctx.repoRoot)
		if err == nil && cur != ctx.cfg.Project.IntegrationBranch && !r.opts.dryRun {
			where := fmt.Sprintf("the work branch %q or an auto/<slice-id> branch", ctx.cfg.Project.IntegrationBranch)
			if r.branchMode() == config.BranchOff {
				where = fmt.Sprintf("the work branch %q (auto.branch: off)", ctx.cfg.Project.IntegrationBranch)
			}
			return &autoStop{Rule: ruleBranch, Detail: fmt.Sprintf("on %q: metis auto starts on %s", cur, where), Exit: exitAutoNeedsPerson}
		}
		return nil
	}
	id := strings.TrimPrefix(b, git.AutoBranchPrefix)
	delete(r.prs, id)
	r.inFlight = id
	l, err := ctx.loadLedger()
	if err != nil {
		return r.errStop(id, err)
	}
	if s := l.FindByID(id); (s == nil && r.isArchived(id)) || (s != nil && s.IsDone()) {
		if r.opts.dryRun {
			r.say("%s is reviewed on %s: a run would land it first", id, b)
			return nil
		}
		r.say("%s is reviewed on %s: landing it", id, b)
		if st := r.complete(id); st != nil {
			return st
		}
	}
	return nil
}

// doneAt reports whether a slice is reviewed in the ledger, or archived,
// at a revision.
func doneAt(ctx *context, rev, id string) bool {
	if data, err := git.FileAt(ctx.repoRoot, rev, ctx.cfg.Paths.Ledger); err == nil {
		if l, err := ledger.Parse(data); err == nil {
			if s := l.FindByID(id); s != nil {
				return s.IsDone()
			}
		}
	}
	if data, err := git.FileAt(ctx.repoRoot, rev, ctx.cfg.Paths.Archive); err == nil {
		if a, err := ledger.ParseArchive(data); err == nil {
			for i := range a.Slices {
				if a.Slices[i].ID == id {
					return true
				}
			}
		}
	}
	return false
}

// enterBranch puts the run on the slice's auto branch: the existing one
// (a resumed slice) or a new one cut from the work branch.
func (r *autoRun) enterBranch(s *slice.Slice) *autoStop {
	if r.branchMode() == config.BranchOff {
		return nil
	}
	ctx := r.ctx
	want := git.AutoBranch(s.ID)
	cur, err := git.CurrentBranch(ctx.repoRoot)
	if err != nil {
		return r.errStop(s.ID, err)
	}
	if cur == want {
		return nil
	}
	if git.BranchExists(ctx.repoRoot, want) {
		err = git.Checkout(ctx.repoRoot, want)
	} else {
		err = git.CreateBranch(ctx.repoRoot, want, ctx.cfg.Project.IntegrationBranch)
	}
	if err != nil {
		return ruleStop(ruleBranch, s, false, "cannot switch to %s: %v", want, err)
	}
	r.say("on %s", want)
	return nil
}

// landBranch lands a reviewed slice's branch: rebased onto the work branch
// if it moved, verified, fast-forward merged and deleted (slice); or
// pushed with a pull request opened (pr). Off: nothing to do.
func (r *autoRun) landBranch(id string) (string, *autoStop) {
	ctx := r.ctx
	b := git.AutoBranch(id)
	if r.branchMode() == config.BranchOff || !git.BranchExists(ctx.repoRoot, b) {
		return "", nil
	}
	work := ctx.cfg.Project.IntegrationBranch
	stop := func(format string, args ...any) *autoStop {
		return &autoStop{Rule: ruleMerge, Slice: id, Detail: fmt.Sprintf(format, args...), Exit: exitAutoNeedsPerson}
	}
	if cur, _ := git.CurrentBranch(ctx.repoRoot); cur != b {
		if err := git.Checkout(ctx.repoRoot, b); err != nil {
			return "", stop("cannot check out %s: %v", b, err)
		}
	}
	if r.branchMode() == config.BranchPR {
		if err := git.Push(ctx.repoRoot, b); err != nil {
			return "", stop("pushing %s failed: %v", b, err)
		}
		argv := ctx.cfg.Auto.PRArgv()
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = ctx.repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", stop("%s failed: %v %s", strings.Join(argv, " "), err, strings.TrimSpace(string(out)))
		}
		if err := git.Checkout(ctx.repoRoot, work); err != nil {
			return "", stop("cannot return to %s: %v", work, err)
		}
		r.prs[id] = true
		return "pull request: " + strings.TrimSpace(string(out)), nil
	}

	how := "fast-forward merged into " + work
	if !git.IsAncestor(ctx.repoRoot, work, b) {
		if err := git.Rebase(ctx.repoRoot, work); err != nil {
			return "", stop("rebasing %s onto %s failed (the rebase was aborted, the branch is as it was): %v", b, work, err)
		}
		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))
		code, _, err := runner.Verify(ctx.cfg, ctx.repoRoot, id, runner.VerifyOptions{Label: "merge"}, store)
		if err != nil || code != 0 {
			return "", stop("after rebasing %s onto %s, verify exited %d: see %s", b, work, code, filepath.Join(ctx.cfg.Paths.Runs, id, "verify-merge.log"))
		}
		how = "rebased, verified and " + how
	}
	if err := git.Checkout(ctx.repoRoot, work); err != nil {
		return "", stop("cannot check out %s: %v", work, err)
	}
	if err := git.MergeFastForward(ctx.repoRoot, b); err != nil {
		_ = git.Checkout(ctx.repoRoot, b)
		return "", stop("fast-forwarding %s to %s failed: %v", work, b, err)
	}
	if err := git.DeleteBranch(ctx.repoRoot, b); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s merged but not deleted: %v\n", b, err)
	}
	return how, nil
}

// usedRemote reports whether any session of a slice ran a remote agent, by
// the run log: only then does landing the slice touch origin.
func (r *autoRun) usedRemote(id string) bool {
	events := r.ctx.readAutoEvents()
	for i := range events {
		e := &events[i]
		if e.Event == "session" && e.Slice == id && r.ctx.cfg.Agents[e.Agent].Remote {
			return true
		}
	}
	return false
}

// pushLanded publishes a landed slice whose sessions ran remotely: the
// work branch is pushed to origin and origin's copy of the slice's auto
// branch deleted. With auto.branch: pr the pull request carries the slice.
func (r *autoRun) pushLanded(id string) *autoStop {
	ctx := r.ctx
	if r.branchMode() == config.BranchPR {
		return nil
	}
	work := ctx.cfg.Project.IntegrationBranch
	if err := git.Push(ctx.repoRoot, work); err != nil {
		return &autoStop{Rule: ruleSync, Slice: id, Exit: exitAutoNeedsPerson,
			Detail: fmt.Sprintf("%s is reviewed and landed on %s here, but %s could not be pushed to origin: %v", id, work, work, err)}
	}
	if r.branchMode() == config.BranchSlice {
		if err := git.DeleteRemoteBranch(ctx.repoRoot, git.AutoBranch(id)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s not deleted on origin: %v\n", git.AutoBranch(id), err)
		}
	}
	return nil
}

// printAutoStatus shows the last run from the run log.
func printAutoStatus(cmd *cobra.Command, ctx *context) error {
	st := lastAutoRun(ctx.readAutoEvents())
	if jsonOutput() {
		if st == nil {
			return printJSON(cmd, map[string]any{"run": nil})
		}
		return printJSON(cmd, st)
	}
	if st == nil {
		fmt.Println("No metis auto run recorded.")
		return nil
	}
	fmt.Printf("Last run: %s (started %s)\n", st.Run, st.Started)
	fmt.Printf("  Slices done: %d %s\n", len(st.Done), strings.Join(st.Done, ", "))
	fmt.Printf("  Sessions:    %d\n", st.Sessions)
	for _, c := range st.Slices {
		fmt.Printf("    %-28s %d session(s), %d blocked review(s)\n", c.Slice, c.Sessions, c.ReviewCycles)
	}
	if st.Stop != nil {
		fmt.Printf("  Stop:        %s — %s\n", st.Stop.Rule, st.Stop.Detail)
		if st.Stop.Escalation != "" {
			fmt.Printf("  Escalation:  %s\n", st.Stop.Escalation)
		}
	} else {
		fmt.Println("  Stop:        none recorded (still running, or killed)")
	}
	return nil
}
