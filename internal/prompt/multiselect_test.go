package prompt

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// chunks is a reader that hands back one scripted chunk per Read, the way
// a raw terminal delivers one keypress per read.
type chunks struct{ parts []string }

func (c *chunks) Read(p []byte) (int, error) {
	if len(c.parts) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.parts[0])
	c.parts = c.parts[1:]
	return n, nil
}

var rules = []string{"first rule", "second rule", "third rule"}

func TestMultiSelect_ChoosesInListOrder(t *testing.T) {
	// Select the third, then move up and select the first.
	in := &chunks{parts: []string{"\x1b[B", "\x1b[B", " ", "k", "k", " ", "\r"}}
	var out bytes.Buffer
	got, err := MultiSelect(in, &out, "Remove which?", rules, 80)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Errorf("chosen = %v, want [0 2]", got)
	}
	if !strings.Contains(out.String(), "[x] 3. third rule") {
		t.Errorf("final draw should show the third rule checked:\n%q", out.String())
	}
}

func TestMultiSelect_KeysInOneRead(t *testing.T) {
	in := &chunks{parts: []string{"j \x1b[A \r"}}
	got, err := MultiSelect(in, io.Discard, "t", rules, 80)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("chosen = %v, want [0 1]", got)
	}
}

func TestMultiSelect_ToggleAll(t *testing.T) {
	got, err := MultiSelect(&chunks{parts: []string{"a", "\r"}}, io.Discard, "t", rules, 80)
	if err != nil || !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Errorf("a: chosen = %v, err %v, want all", got, err)
	}
	// With everything selected, a clears the lot.
	got, err = MultiSelect(&chunks{parts: []string{"a", "a", "\r"}}, io.Discard, "t", rules, 80)
	if err != nil || got != nil {
		t.Errorf("a a: chosen = %v, err %v, want none", got, err)
	}
	// With some selected, a selects the rest.
	got, err = MultiSelect(&chunks{parts: []string{" ", "a", "\r"}}, io.Discard, "t", rules, 80)
	if err != nil || !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Errorf("space a: chosen = %v, err %v, want all", got, err)
	}
}

func TestMultiSelect_CursorStaysInBounds(t *testing.T) {
	in := &chunks{parts: []string{"k", "k", " ", "j", "j", "j", "j", " ", "\r"}}
	got, err := MultiSelect(in, io.Discard, "t", rules, 80)
	if err != nil || !reflect.DeepEqual(got, []int{0, 2}) {
		t.Errorf("chosen = %v, err %v, want [0 2]", got, err)
	}
}

func TestMultiSelect_Cancel(t *testing.T) {
	for name, keys := range map[string][]string{
		"q":      {" ", "q"},
		"esc":    {" ", "\x1b"},
		"ctrl-c": {" ", "\x03"},
		"eof":    {" "},
	} {
		_, err := MultiSelect(&chunks{parts: keys}, io.Discard, "t", rules, 80)
		if !errors.Is(err, ErrCancelled) {
			t.Errorf("%s: err = %v, want ErrCancelled", name, err)
		}
	}
}

func TestMultiSelect_RedrawsInPlace(t *testing.T) {
	var out bytes.Buffer
	if _, err := MultiSelect(&chunks{parts: []string{"j", "\r"}}, &out, "t", rules, 80); err != nil {
		t.Fatal(err)
	}
	// Title, key help, three items: each redraw moves up five lines.
	if !strings.Contains(out.String(), "\x1b[5A") {
		t.Errorf("redraw should move the cursor back over the list:\n%q", out.String())
	}
	if !strings.HasSuffix(out.String(), "\x1b[?25h") {
		t.Errorf("the cursor must be shown again on exit:\n%q", out.String())
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("żółw", 10); got != "żółw" {
		t.Errorf("short strings pass through, got %q", got)
	}
	if got := truncate("a\nb", 10); got != "a b" {
		t.Errorf("newlines must not break a line, got %q", got)
	}
}
