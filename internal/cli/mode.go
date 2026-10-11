package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/fsutil"
	"github.com/techspeque/metis/internal/progress"
	"github.com/techspeque/metis/internal/slice"
)

// Where an effective mode came from, narrowest first.
const (
	modeFromSlice   = "slice"
	modeFromPhase   = "phase"
	modeFromRisk    = "risk"
	modeFromProject = "project"
	modeFromDefault = "default"
)

// modeChoice is a slice's effective auto mode and the level it came from.
type modeChoice struct {
	Mode   string `json:"auto_mode"`
	Source string `json:"auto_mode_source"`
}

// modeOverrides are one run's overrides; nothing writes them anywhere.
// A scoped one (slice, phase) wins over the persistent setting at its
// level; project replaces only auto.mode.
type modeOverrides struct {
	slice   map[string]string
	phase   map[string]string
	project string
}

// modeResolver resolves effective modes, reading each plan file's header
// once.
type modeResolver struct {
	ctx   *context
	ov    modeOverrides
	plans map[string]string
}

func newModeResolver(ctx *context, ov modeOverrides) *modeResolver {
	return &modeResolver{ctx: ctx, ov: ov, plans: map[string]string{}}
}

// of returns a slice's effective mode: slice, then phase, then risk, then
// project, then the default (step — an upgrade never makes a project
// unattended).
func (r *modeResolver) of(s *slice.Slice) modeChoice {
	if m := r.ov.slice[s.ID]; m != "" {
		return modeChoice{m, modeFromSlice}
	}
	if s.AutoMode != "" {
		return modeChoice{s.AutoMode, modeFromSlice}
	}
	if m := r.phaseMode(progress.PhaseOf(s), s.Plan); m != "" {
		return modeChoice{m, modeFromPhase}
	}
	if m := r.ctx.cfg.Auto.ByRisk[string(s.Risk)]; m != "" {
		return modeChoice{m, modeFromRisk}
	}
	if m := r.projectMode(); m.Mode != "" {
		return m
	}
	return modeChoice{config.ModeStep, modeFromDefault}
}

// phaseMode is a phase's own mode: the run's override, auto.phases, then
// the plan file's "> Auto: <mode>" header line.
func (r *modeResolver) phaseMode(phase, plan string) string {
	if m := r.ov.phase[phase]; m != "" {
		return m
	}
	if m := r.ctx.cfg.Auto.Phases[phase]; m != "" {
		return m
	}
	if plan == "" {
		return ""
	}
	m, ok := r.plans[plan]
	if !ok {
		m = planHeaderMode(filepath.Join(r.ctx.repoRoot, plan))
		r.plans[plan] = m
	}
	return m
}

// phaseChoice is a phase's mode without any slice's own setting: what
// progress shows per phase.
func (r *modeResolver) phaseChoice(phase, plan string) modeChoice {
	if m := r.phaseMode(phase, plan); m != "" {
		return modeChoice{m, modeFromPhase}
	}
	if m := r.projectMode(); m.Mode != "" {
		return m
	}
	return modeChoice{config.ModeStep, modeFromDefault}
}

func (r *modeResolver) projectMode() modeChoice {
	if r.ov.project != "" {
		return modeChoice{r.ov.project, modeFromProject}
	}
	if m := r.ctx.cfg.Auto.ProjectMode(); m != "" {
		return modeChoice{m, modeFromProject}
	}
	return modeChoice{}
}

var planAutoRe = regexp.MustCompile(`^>\s*Auto:\s*(manual|step|auto)\b`)

// planHeaderMode reads a "> Auto: <mode>" line from a plan file's header
// (before its first H2).
func planHeaderMode(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "## ") {
			break
		}
		if m := planAutoRe.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

func init() {
	modeCmd.Flags().String("phase", "", "Set the mode of a phase (auto.phases.<phase>) instead of a slice")
	modeCmd.Flags().Bool("clear", false, "Remove the override so the level above applies again")
	rootCmd.AddCommand(modeCmd)
}

var modeCmd = &cobra.Command{
	Use:   "mode [<slice-id>] [manual|step|auto]",
	Short: "Show or set how far metis auto takes a slice or a phase",
	Long: `metis auto's mode decides how far one run goes: manual (the loop stops
before the slice), step (one slice to reviewed and archived, then stop) or
auto (continue until a stop rule fires).

The narrowest setting wins: slice, then phase, then risk (auto.by_risk),
then project (auto.mode), then step. Each level overrides the one above in
either direction.

  metis mode                          every open slice's effective mode, by phase
  metis mode <slice-id> <mode>        store the mode on the slice (ledger)
  metis mode <slice-id> --clear       back to the phase's mode
  metis mode --phase <phase> <mode>   auto.phases.<phase> in project.yaml
  metis mode --phase <phase> --clear  back to the project's mode

A phase is what 'metis progress' shows: the plan file's name, or the id's
phase-N prefix. Writes are committed, like 'metis edit'.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		phase, _ := cmd.Flags().GetString("phase")
		clear, _ := cmd.Flags().GetBool("clear")

		if phase != "" {
			if len(args) > 1 || (len(args) == 1) == clear {
				return fmt.Errorf("usage: metis mode --phase <phase> <manual|step|auto>, or --phase <phase> --clear")
			}
			mode := ""
			if !clear {
				mode = args[0]
				if !config.ValidMode(mode) {
					return fmt.Errorf("%q is not a mode (valid: manual, step, auto)", mode)
				}
			}
			if err := setPhaseMode(ctx.cfgPath, phase, mode); err != nil {
				return err
			}
			if clear {
				fmt.Printf("Cleared the mode of phase %s\n", phase)
				ctx.commitStateSoft(phase, "mode cleared", ctx.cfgPath)
			} else {
				fmt.Printf("Phase %s: %s\n", phase, mode)
				ctx.commitStateSoft(phase, "mode "+mode, ctx.cfgPath)
			}
			return nil
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			if clear {
				return fmt.Errorf("--clear needs a slice id or --phase")
			}
			return printModes(cmd, ctx, l.PendingSlices())
		}

		s := l.FindByID(args[0])
		if s == nil {
			return fmt.Errorf("slice %q not found", args[0])
		}
		switch {
		case clear && len(args) == 1:
			s.AutoMode = ""
		case !clear && len(args) == 2:
			if !config.ValidMode(args[1]) {
				return fmt.Errorf("%q is not a mode (valid: manual, step, auto)", args[1])
			}
			s.AutoMode = args[1]
		case len(args) == 1:
			eff := newModeResolver(ctx, modeOverrides{}).of(s)
			if jsonOutput() {
				return printJSON(cmd, map[string]string{"id": s.ID, "auto_mode": eff.Mode, "auto_mode_source": eff.Source})
			}
			fmt.Printf("%s: %s (from %s)\n", s.ID, eff.Mode, eff.Source)
			return nil
		default:
			return fmt.Errorf("usage: metis mode <slice-id> <manual|step|auto>, or <slice-id> --clear")
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}
		eff := newModeResolver(ctx, modeOverrides{}).of(s)
		msg := "mode " + s.AutoMode
		if clear {
			msg = "mode cleared"
		}
		fmt.Printf("%s: %s (from %s)\n", s.ID, eff.Mode, eff.Source)
		ctx.commitStateSoft(s.ID, msg, ctx.ledgerPath())
		return nil
	},
}

// modeRow is one slice in 'metis mode' output.
type modeRow struct {
	ID    string `json:"id"`
	Phase string `json:"phase"`
	modeChoice
}

func printModes(cmd *cobra.Command, ctx *context, pending []slice.Slice) error {
	r := newModeResolver(ctx, modeOverrides{})
	rows := []modeRow{}
	for i := range pending {
		rows = append(rows, modeRow{ID: pending[i].ID, Phase: progress.PhaseOf(&pending[i]), modeChoice: r.of(&pending[i])})
	}
	if jsonOutput() {
		return printJSON(cmd, rows)
	}
	if len(rows) == 0 {
		fmt.Println("No open slices.")
		return nil
	}
	last := ""
	for _, row := range rows {
		if row.Phase != last {
			ph := r.phaseChoice(row.Phase, planOf(pending, row.Phase))
			fmt.Printf("%s  (phase: %s, from %s)\n", row.Phase, ph.Mode, ph.Source)
			last = row.Phase
		}
		fmt.Printf("  %-28s %-7s from %s\n", row.ID, row.Mode, row.Source)
	}
	return nil
}

// planOf is the plan file of the first slice in a phase.
func planOf(slices []slice.Slice, phase string) string {
	for i := range slices {
		if progress.PhaseOf(&slices[i]) == phase {
			return slices[i].Plan
		}
	}
	return ""
}

// setPhaseMode writes auto.phases.<phase> (or removes it when mode is
// empty), preserving the rest of project.yaml.
func setPhaseMode(cfgPath, phase, mode string) error {
	if mode != "" {
		return config.SetInFile(cfgPath, "auto.phases."+phase, mode)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("phase %s has no mode set", phase)
	}
	phases := lookupNode(doc.Content[0], "auto", "phases")
	if phases == nil || !removeKey(phases, phase) {
		return fmt.Errorf("phase %s has no mode set (auto.phases.%s)", phase, phase)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(cfgPath, buf.Bytes(), 0o644)
}

func lookupNode(n *yaml.Node, path ...string) *yaml.Node {
	for _, key := range path {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				next = n.Content[i+1]
			}
		}
		n = next
	}
	return n
}

func removeKey(m *yaml.Node, key string) bool {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return true
		}
	}
	return false
}
