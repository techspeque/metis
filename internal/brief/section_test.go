package brief

import (
	"reflect"
	"testing"
)

const sectionBrief = `# s-1 — Title

## Goal

Do it.

## Review

- Check the retry bound

` + "```" + `
## not a heading inside a fence
` + "```" + `

## Human decisions

- [x] Use UTC? Yes.
- [ ] Which vendor for SMS?
* [ ] Keep the v1 endpoint?
`

func TestSection(t *testing.T) {
	got := Section(sectionBrief, "review")
	want := "- Check the retry bound\n\n```\n## not a heading inside a fence\n```"
	if got != want {
		t.Errorf("Section(review) = %q, want %q", got, want)
	}
	if got := Section(sectionBrief, "Out-of-scope"); got != "" {
		t.Errorf("absent section = %q", got)
	}
}

func TestUnansweredDecisions(t *testing.T) {
	got := UnansweredDecisions(sectionBrief)
	want := []string{"Which vendor for SMS?", "Keep the v1 endpoint?"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnansweredDecisions = %v, want %v", got, want)
	}
}
