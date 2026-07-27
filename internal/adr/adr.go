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

// superseded is one ADR that is no longer current.
type superseded struct {
	token string // citation token, e.g. "ADR-0003"
	by    string // superseding ADR's token, or "" when only status marks it
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
				if seen[key] || !strings.Contains(content, s.token) {
					continue
				}
				seen[key] = true
				if s.by != "" {
					warnings = append(warnings, fmt.Sprintf("%s cites %s, superseded by %s — re-verify the quoted decision", relPath, s.token, s.by))
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

// collectSuperseded reads every ADR's frontmatter and returns the set of
// no-longer-current decisions.
func collectSuperseded(repoRoot, adrRel string) []superseded {
	dir := filepath.Join(repoRoot, adrRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type record struct {
		rel, id, status, supersedes string
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
		id, status, supersedes := parseFrontmatter(string(data))
		records = append(records, record{rel: rel, id: id, status: status, supersedes: supersedes})
		if id != "" {
			idFile[id] = rel
		}
	}

	byToken := map[string]*superseded{}
	mark := func(token string) *superseded {
		if byToken[token] == nil {
			byToken[token] = &superseded{token: token, file: idFile[token]}
		}
		return byToken[token]
	}
	for _, r := range records {
		if r.supersedes != "" {
			s := mark(r.supersedes)
			s.by = r.id
			s.via = r.rel
		}
		if r.id != "" && (r.status == "superseded" || r.status == "deprecated") {
			mark(r.id)
		}
	}

	var out []superseded
	for _, s := range byToken {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].token < out[j].token })
	return out
}

// parseFrontmatter extracts id, status, and supersedes from an ADR's YAML
// frontmatter block. IDs are normalized to the "ADR-NNNN" citation form.
// Unfilled template placeholders (angle brackets, alternative lists) yield "".
func parseFrontmatter(content string) (id, status, supersedes string) {
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
			id = normalizeToken(value)
		case "status":
			status = strings.ToLower(value)
		case "supersedes":
			supersedes = normalizeToken(value)
		}
	}
	return id, status, supersedes
}

// normalizeToken maps "0003", "adr-0003", or "ADR-0003" to "ADR-0003".
func normalizeToken(v string) string {
	v = strings.TrimSpace(v)
	upper := strings.ToUpper(v)
	if strings.HasPrefix(upper, "ADR-") {
		return "ADR-" + upper[len("ADR-"):]
	}
	return "ADR-" + v
}
