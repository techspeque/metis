package brief

import (
	"strings"
)

// Section returns the body of the first H2 section whose heading starts
// with the given title (case-insensitive), trimmed; empty when absent.
// The body runs to the next H1 or H2 heading.
func Section(content, title string) string {
	want := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	in := false
	fence := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fence = !fence
		}
		if !fence && (strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ")) {
			if in {
				break
			}
			heading := strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			in = strings.HasPrefix(heading, want)
			continue
		}
		if in {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

// UnansweredDecisions returns the open items of a brief's "## Human
// decisions" section: each unchecked "- [ ]" line, without its checkbox.
// A checked "- [x]" item is answered.
func UnansweredDecisions(content string) []string {
	var open []string
	for _, line := range strings.Split(Section(content, "Human decisions"), "\n") {
		trimmed := strings.TrimSpace(line)
		for _, box := range []string{"- [ ]", "* [ ]"} {
			if strings.HasPrefix(trimmed, box) {
				open = append(open, strings.TrimSpace(strings.TrimPrefix(trimmed, box)))
			}
		}
	}
	return open
}
