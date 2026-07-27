// Package adr implements the reverse citation walk for Architecture Decision
// Records: an ADR that amends another declares it (supersedes: frontmatter),
// but nothing finds the documents still quoting the amended decision. This
// package does that walk, so 'metis check' can flag prose citing a rule that
// has since changed.
package adr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// superseded is one ADR that is no longer current (or no longer whole).
type superseded struct {
	token string // citation token, e.g. "ADR-0003"
	by    string // superseding/amending ADR's token, or "" when only status marks it
	kind  string // "superseded" or "amended"
	file  string // repo-relative path of the superseded ADR, if present
	via   string // repo-relative path of the superseding ADR, if any
}

// CheckCitations scans the ADR directory for decisions marked superseded —
// either named in another ADR's supersedes: field, or carrying a
// superseded/deprecated status themselves — and returns a warning for every
// project document that still cites one. The superseding ADR itself and the
// superseded file are legitimate citation sites and are not flagged.
//
// adrRel and scanRels are repo-relative; scanRels entries may be files or
// directories (walked for .md files). Missing paths are skipped silently —
// this is a best-effort advisory, not a gate.
func CheckCitations(repoRoot, adrRel string, scanRels []string) []string {
	stale := collectSuperseded(repoRoot, adrRel)
	if len(stale) == 0 {
		return nil
	}

	var warnings []string
	seen := map[string]bool{}
	for _, rel := range scanRels {
		root := filepath.Join(repoRoot, rel)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			if strings.HasPrefix(filepath.Base(path), "_") {
				return nil // templates
			}
			relPath, rerr := filepath.Rel(repoRoot, path)
			if rerr != nil {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			content := string(data)
			for _, s := range stale {
				if relPath == s.file || relPath == s.via {
					continue
				}
				key := relPath + "\x00" + s.token
				if seen[key] || !citesToken(content, s.token) {
					continue
				}
				seen[key] = true
				if s.by != "" {
					warnings = append(warnings, fmt.Sprintf("%s cites %s, %s by %s — re-verify the quoted decision", relPath, s.token, s.kind, s.by))
				} else {
					warnings = append(warnings, fmt.Sprintf("%s cites %s, which is no longer current — re-verify the quoted decision", relPath, s.token))
				}
			}
			return nil
		})
	}
	sort.Strings(warnings)
	return warnings
}

// citesToken reports whether content cites the token as a whole ID — a
// digit right after the match means a longer ID that merely shares the
// prefix (ADR-001 inside ADR-0011 is not a citation of ADR-001).
func citesToken(content, token string) bool {
	for from := 0; ; {
		idx := strings.Index(content[from:], token)
		if idx < 0 {
			return false
		}
		end := from + idx + len(token)
		if end >= len(content) || content[end] < '0' || content[end] > '9' {
			return true
		}
		from = end
	}
}

// collectSuperseded reads every ADR's frontmatter and returns the set of
// no-longer-current decisions.
func collectSuperseded(repoRoot, adrRel string) []superseded {
	dir := filepath.Join(repoRoot, adrRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type record struct {
		rel, id, status, supersedes, amends string
	}
	var records []record
	idFile := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(adrRel, name))
		fm := parseFrontmatter(string(data))
		records = append(records, record{rel: rel, id: fm.id, status: fm.status, supersedes: fm.supersedes, amends: fm.amends})
		if fm.id != "" {
			idFile[fm.id] = rel
		}
	}

	byToken := map[string]*superseded{}
	mark := func(token, kind string) *superseded {
		if byToken[token] == nil {
			byToken[token] = &superseded{token: token, kind: kind, file: idFile[token]}
		}
		if kind == "superseded" {
			byToken[token].kind = kind // full replacement outranks amendment
		}
		return byToken[token]
	}
	for _, r := range records {
		if r.supersedes != "" {
			s := mark(r.supersedes, "superseded")
			s.by = r.id
			s.via = r.rel
		}
		if r.amends != "" {
			s := mark(r.amends, "amended")
			if s.by == "" {
				s.by = r.id
				s.via = r.rel
			}
		}
		if r.id != "" && (r.status == "superseded" || r.status == "deprecated") {
			mark(r.id, "superseded")
		}
	}

	var out []superseded
	for _, s := range byToken {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].token < out[j].token })
	return out
}

// frontmatter is the subset of ADR frontmatter the citation walk reads.
type frontmatter struct {
	id, status, supersedes, amends string
}

// parseFrontmatter extracts the citation-relevant keys from an ADR's YAML
// frontmatter block. IDs are normalized to the "ADR-NNNN" citation form.
// Unfilled template placeholders (angle brackets, alternative lists) yield "".
func parseFrontmatter(content string) (fm frontmatter) {
	lines := strings.Split(content, "\n")
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inBlock {
				break
			}
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "<") || strings.Contains(value, "|") {
			continue // unfilled template placeholder
		}
		switch strings.TrimSpace(key) {
		case "id":
			fm.id = normalizeToken(value)
		case "status":
			fm.status = strings.ToLower(value)
		case "supersedes":
			fm.supersedes = normalizeToken(value)
		case "amends":
			fm.amends = normalizeToken(value)
		}
	}
	return fm
}

// normalizeToken maps "0003", "adr-0003", or "ADR-0003" to "ADR-0003",
// dropping any trailing qualifier ("ADR-0002 (script-purpose clause only)").
func normalizeToken(v string) string {
	if fields := strings.Fields(v); len(fields) > 0 {
		v = fields[0]
	}
	upper := strings.ToUpper(v)
	if strings.HasPrefix(upper, "ADR-") {
		return "ADR-" + upper[len("ADR-"):]
	}
	return "ADR-" + v
}
