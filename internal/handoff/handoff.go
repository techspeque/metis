// Package handoff renders the prompt one agent session needs for a slice's
// next role, chosen from ledger state: code, rework, review, re-review or
// gate-review. It is the prompt a person would otherwise ask the coder to
// write for the reviewer, rendered from the same state every time.
package handoff

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/techspeque/metis/internal/brief"
	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/slice"
	"github.com/techspeque/metis/internal/templates"
)

// The handoff templates.
const (
	Code       = "code"
	Rework     = "rework"
	Review     = "review"
	ReReview   = "re-review"
	GateReview = "gate-review"
)

// Templates lists every template name.
var Templates = []string{Code, Rework, Review, ReReview, GateReview}

// ValidTemplate reports whether name is one of Templates.
func ValidTemplate(name string) bool {
	for _, t := range Templates {
		if t == name {
			return true
		}
	}
	return false
}

// OverrideDir is where a project overrides a template, relative to the
// repository root.
const OverrideDir = ".metis/templates/handoff"

// Choose returns the template for a slice's role in its current state.
func Choose(s *slice.Slice, role slice.Role) string {
	if role == slice.RoleReviewer {
		switch {
		case s.Type == slice.TypeGate:
			return GateReview
		case s.ReviewCycles > 0:
			return ReReview
		default:
			return Review
		}
	}
	if s.ReviewCycles > 0 {
		return Rework
	}
	return Code
}

// RoleOf returns the role a template is written for.
func RoleOf(tmpl string) slice.Role {
	switch tmpl {
	case Review, ReReview, GateReview:
		return slice.RoleReviewer
	default:
		return slice.RoleCoder
	}
}

// Input is everything a render reads.
type Input struct {
	Cfg       *config.Config
	RepoRoot  string
	Slice     *slice.Slice
	Role      slice.Role
	AgentSlug string
	// Findings are the slice's findings, in recorded order.
	Findings []findings.Finding
	// Commits are the slice's commits, oldest first.
	Commits []git.SliceCommit
	// Branch is the branch the session works on; empty means the
	// integration branch.
	Branch string
}

// Agent is the session's agent as the templates see it.
type Agent struct {
	Slug    string
	Label   string
	Surface string
	Model   string
}

// Data is the template data.
type Data struct {
	Slice             *slice.Slice
	Role              string
	Template          string
	Agent             Agent
	BriefPath         string
	BriefExists       bool
	BriefCommitted    bool
	ReviewSection     string
	EvidenceIndex     string
	Checklist         []string
	BlockReasons      []string
	Conventions       []string
	Cycle             int
	NextReview        int
	Narrowed          bool
	OpenFindings      []findings.Finding
	LastCycleFindings []findings.Finding
	// EarlierOpenFindings are open findings raised before the last review.
	EarlierOpenFindings []findings.Finding
	Commits             []git.SliceCommit
	ReviewedCommand     string
	Branch              string
	IntegrationBranch   string
}

// BuildData assembles the template data for one template.
func BuildData(in *Input, tmpl string) Data {
	s := in.Slice
	cfg := in.Cfg
	d := Data{
		Slice:             s,
		Role:              string(RoleOf(tmpl)),
		Template:          tmpl,
		Agent:             Agent{Slug: in.AgentSlug},
		Checklist:         cfg.ReviewChecklist,
		BlockReasons:      cfg.Review.Reasons(),
		Conventions:       cfg.Conventions,
		Cycle:             s.ReviewCycles,
		NextReview:        s.ReviewCycles + 1,
		Narrowed:          cfg.Review.Narrowed(s.ReviewCycles),
		Branch:            in.Branch,
		IntegrationBranch: cfg.Project.IntegrationBranch,
	}
	if d.Branch == "" {
		d.Branch = d.IntegrationBranch
	}
	if a, ok := cfg.Agents[in.AgentSlug]; ok {
		d.Agent.Label, d.Agent.Surface, d.Agent.Model = a.Label, a.Surface, a.Model
	}
	if cfg.Routing.Review == "self" {
		d.ReviewedCommand = fmt.Sprintf("`metis commit --flip reviewed --slice %s`", s.ID)
	} else {
		d.ReviewedCommand = fmt.Sprintf("`metis commit --flip reviewed --agent %s --slice %s`", in.AgentSlug, s.ID)
	}

	d.BriefPath = filepath.ToSlash(filepath.Join(cfg.Paths.Briefs, s.ID+".md"))
	var content string
	if data, err := git.FileAtHead(in.RepoRoot, d.BriefPath); err == nil {
		d.BriefExists, d.BriefCommitted = true, true
		content = string(data)
	} else if data, err := os.ReadFile(filepath.Join(in.RepoRoot, d.BriefPath)); err == nil {
		d.BriefExists = true
		content = string(data)
	}
	d.ReviewSection = brief.Section(content, "Review")
	d.EvidenceIndex = brief.Section(content, "Evidence")

	for i := range in.Findings {
		f := in.Findings[i]
		if f.Status != "open" {
			continue
		}
		d.OpenFindings = append(d.OpenFindings, f)
		if f.Cycle == s.ReviewCycles {
			d.LastCycleFindings = append(d.LastCycleFindings, f)
		} else {
			d.EarlierOpenFindings = append(d.EarlierOpenFindings, f)
		}
	}
	// Findings recorded before metis tracked their cycle cannot be placed:
	// the last review then stands for every open one.
	if len(d.LastCycleFindings) == 0 {
		d.LastCycleFindings, d.EarlierOpenFindings = d.OpenFindings, nil
	}
	d.Commits = commitsFor(in.Commits, tmpl)
	return d
}

// commitsFor picks the commits a template lists: every commit for a first
// review, the commits after the last block for a re-review.
func commitsFor(commits []git.SliceCommit, tmpl string) []git.SliceCommit {
	switch tmpl {
	case Review, GateReview:
		return commits
	case ReReview:
		last := -1
		for i, c := range commits {
			if strings.Contains(c.Subject, "block review") {
				last = i
			}
		}
		return commits[last+1:]
	}
	return nil
}

// Render renders a template, preferring a project override.
func Render(in *Input, tmpl string) (string, error) {
	if !ValidTemplate(tmpl) {
		return "", fmt.Errorf("unknown handoff template %q (valid: %s)", tmpl, strings.Join(Templates, ", "))
	}
	t, err := load(in.RepoRoot, tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, tmpl, BuildData(in, tmpl)); err != nil {
		return "", fmt.Errorf("rendering handoff template %s: %w", tmpl, err)
	}
	return tidy(buf.String()), nil
}

var funcs = template.FuncMap{
	"inc": func(i int) int { return i + 1 },
}

// load parses the shared definitions and the named template: each from
// the project's override directory when present, else built in.
func load(repoRoot, name string) (*template.Template, error) {
	t := template.New(name).Funcs(funcs)
	for _, file := range []string{"_header", name} {
		text, err := source(repoRoot, file)
		if err != nil {
			return nil, err
		}
		if _, err := t.New(file).Parse(text); err != nil {
			return nil, fmt.Errorf("parsing handoff template %s: %w", file, err)
		}
	}
	return t, nil
}

func source(repoRoot, name string) (string, error) {
	file := name + ".md.tmpl"
	if repoRoot != "" {
		if data, err := os.ReadFile(filepath.Join(repoRoot, OverrideDir, file)); err == nil {
			return string(data), nil
		}
	}
	data, err := fs.ReadFile(templates.Handoff, "handoff/"+file)
	if err != nil {
		return "", fmt.Errorf("handoff template %s: %w", name, err)
	}
	return string(data), nil
}

// tidy collapses runs of blank lines left by empty template sections.
func tidy(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var out []string
	blank := false
	for _, l := range lines {
		l = strings.TrimRight(l, " \t")
		if l == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n") + "\n"
}
