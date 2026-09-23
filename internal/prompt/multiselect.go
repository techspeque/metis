// Package prompt implements the small interactive terminal prompts metis
// offers a human at the keyboard. Every prompt has a non-interactive
// equivalent on the command line; a prompt is never the only way in.
package prompt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// ErrCancelled is returned when the user backs out of a prompt.
var ErrCancelled = errors.New("cancelled")

// ErrNotTerminal is returned when a prompt is asked of a stream that is not
// an interactive terminal.
var ErrNotTerminal = errors.New("not an interactive terminal")

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// MultiSelectTerminal shows a multi-select list of items on the terminal
// behind in and out and returns the indexes the user chose, in list order.
// It puts the terminal in raw mode for the duration and restores it after.
func MultiSelectTerminal(in, out *os.File, title string, items []string) ([]int, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNotTerminal
	}
	width := 80
	if w, _, err := term.GetSize(int(out.Fd())); err == nil && w > 0 {
		width = w
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("entering raw mode: %w", err)
	}
	defer func() { _ = term.Restore(fd, state) }()
	return MultiSelect(in, out, title, items, width)
}

// MultiSelect runs the multi-select over raw key input from in, drawing to
// out lines at most width columns wide. The caller owns the terminal mode.
//
// Keys: up/down or k/j move, space toggles, a toggles all, enter confirms,
// q, esc or ctrl-c cancels.
func MultiSelect(in io.Reader, out io.Writer, title string, items []string, width int) ([]int, error) {
	m := newModel(items)
	defer func() { _, _ = io.WriteString(out, "\x1b[?25h") }() // show the cursor again

	drawn := 0
	buf := make([]byte, 64)
	for {
		// Hide the cursor, move back over the last draw, clear, redraw.
		frame := "\x1b[?25l"
		if drawn > 0 {
			frame += fmt.Sprintf("\x1b[%dA", drawn)
		}
		lines := m.view(title, width)
		frame += "\r\x1b[J" + strings.Join(lines, "\r\n") + "\r\n"
		if _, err := io.WriteString(out, frame); err != nil {
			return nil, err
		}
		drawn = len(lines)

		n, err := in.Read(buf)
		if n == 0 && err != nil {
			if errors.Is(err, io.EOF) {
				return nil, ErrCancelled
			}
			return nil, err
		}
		for _, k := range keys(buf[:n]) {
			switch m.handle(k) {
			case outcomeConfirm:
				return m.chosen(), nil
			case outcomeCancel:
				return nil, ErrCancelled
			}
		}
	}
}

type key string

const (
	keyUp     key = "up"
	keyDown   key = "down"
	keyEscape key = "esc"
)

// keys splits one read of raw terminal input into keypresses. Arrow keys
// arrive as the three-byte sequences ESC [ A and ESC [ B; a lone ESC is
// the escape key.
func keys(b []byte) []key {
	var out []key
	for len(b) > 0 {
		if b[0] == 0x1b {
			if len(b) >= 3 && b[1] == '[' {
				switch b[2] {
				case 'A':
					out = append(out, keyUp)
				case 'B':
					out = append(out, keyDown)
				}
				b = b[3:]
				continue
			}
			out = append(out, keyEscape)
			b = b[1:]
			continue
		}
		out = append(out, key(b[:1]))
		b = b[1:]
	}
	return out
}

type outcome int

const (
	outcomeContinue outcome = iota
	outcomeConfirm
	outcomeCancel
)

// model is the multi-select state: the items, the cursor, and the choices.
type model struct {
	items    []string
	cursor   int
	selected []bool
}

func newModel(items []string) *model {
	return &model{items: items, selected: make([]bool, len(items))}
}

func (m *model) handle(k key) outcome {
	switch k {
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyDown, "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case " ":
		if len(m.items) > 0 {
			m.selected[m.cursor] = !m.selected[m.cursor]
		}
	case "a":
		all := true
		for _, s := range m.selected {
			all = all && s
		}
		for i := range m.selected {
			m.selected[i] = !all
		}
	case "\r", "\n":
		return outcomeConfirm
	case keyEscape, "q", "\x03":
		return outcomeCancel
	}
	return outcomeContinue
}

func (m *model) chosen() []int {
	var out []int
	for i, s := range m.selected {
		if s {
			out = append(out, i)
		}
	}
	return out
}

func (m *model) view(title string, width int) []string {
	lines := []string{
		truncate(title, width),
		truncate("  ↑/↓ move · space select · a all · enter confirm · q cancel", width),
	}
	for i, item := range m.items {
		pointer := "  "
		if i == m.cursor {
			pointer = "> "
		}
		box := "[ ]"
		if m.selected[i] {
			box = "[x]"
		}
		lines = append(lines, truncate(fmt.Sprintf("%s%s %d. %s", pointer, box, i+1, item), width))
	}
	return lines
}

// truncate shortens s to at most width runes so no line wraps — a wrapped
// line would throw off the redraw's cursor arithmetic.
func truncate(s string, width int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if width <= 1 || utf8.RuneCountInString(s) <= width {
		return s
	}
	r := []rune(s)
	return string(r[:width-1]) + "…"
}
