package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/techspeque/metis/internal/config"
)

func writeLog(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "1-reviewer.log")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The fixtures are the logs of real reviewer sessions (opencode on a
// ChatGPT account) that a usage limit stopped: one at its first request,
// one in the middle of a review.
func TestLimitedOnRealLogs(t *testing.T) {
	now := time.Now()
	for _, name := range []string{"opencode-usage-limit.log", "opencode-usage-limit-mid-session.log"} {
		limited, reset := Limited(&config.Agent{Surface: "opencode"}, 1, filepath.Join("testdata", name), now)
		if !limited {
			t.Errorf("%s: not classified as limited", name)
		}
		if !reset.IsZero() {
			t.Errorf("%s: reset = %s, the message names none", name, reset)
		}
	}
}

func TestLimited(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	own := &config.RateLimit{ExitCodes: []int{75}, Patterns: []string{`quota exceeded`}, ResetPattern: `resets? (?:at|in) (.+)`}
	cases := []struct {
		name  string
		agent config.Agent
		exit  int
		log   string
		want  bool
		reset time.Time
	}{
		{name: "another failure", agent: config.Agent{Surface: "opencode"}, exit: 1, log: "panic: nil map\n"},
		{name: "a surface without defaults", agent: config.Agent{Surface: "fake"}, exit: 1, log: "usage limit reached\n"},
		{name: "case and colour are ignored", agent: config.Agent{Surface: "opencode"}, exit: 1, log: "Usage \x1b[1mLIMIT\x1b[0m reached\n", want: true},
		{name: "a listed exit code", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: 75, log: "", want: true},
		{name: "an unlisted exit code", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: 1, log: ""},
		{name: "a timed-out session matches no exit code", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: -1, log: ""},
		{name: "the agent's patterns replace the surface's", agent: config.Agent{Surface: "opencode", RateLimit: own}, exit: 1, log: "usage limit reached\n"},
		{name: "the agent's pattern with a duration", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: 1, log: "Quota exceeded, resets in 2h 5m\n", want: true, reset: now.Add(125 * time.Minute)},
		{name: "the last reset time counts", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: 1, log: "quota exceeded, resets in 1h\nquota exceeded, resets in 3h\n", want: true, reset: now.Add(3 * time.Hour)},
		{name: "an unreadable reset time", agent: config.Agent{Surface: "fake", RateLimit: own}, exit: 1, log: "quota exceeded, resets at some point\n", want: true},
		{name: "only the end of the log is read", agent: config.Agent{Surface: "opencode"}, exit: 1, log: "usage limit reached\n" + strings.Repeat("x", rateLimitTail)},
		{name: "codex names a duration", agent: config.Agent{Surface: "codex"}, exit: 1, log: "You've hit your usage limit. Upgrade to Pro or try again in 3 hours 12 minutes.\n", want: true, reset: now.Add(192 * time.Minute)},
		{name: "claude code names a timestamp", agent: config.Agent{Surface: "claude-code"}, exit: 1, log: "Claude AI usage limit reached|1795780800\n", want: true, reset: time.Unix(1795780800, 0)},
		{name: "claude code names a clock time", agent: config.Agent{Surface: "claude-code"}, exit: 1, log: "You've hit your session limit · resets 3pm (UTC)\n", want: true, reset: time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limited, reset := Limited(&tc.agent, tc.exit, writeLog(t, tc.log), now)
			if limited != tc.want || !reset.Equal(tc.reset) {
				t.Errorf("Limited = %t, %s; want %t, %s", limited, reset, tc.want, tc.reset)
			}
		})
	}
	if limited, _ := Limited(&config.Agent{Surface: "opencode"}, 1, filepath.Join(t.TempDir(), "missing.log"), now); limited {
		t.Error("a session without a log is not limited")
	}
}

func TestParseReset(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC)
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, time.UTC) }
	cases := []struct {
		text string
		want time.Time
	}{
		{"45 minutes", now.Add(45 * time.Minute)},
		{"3 hours 12 minutes", now.Add(192 * time.Minute)},
		{"1 hour and 5 minutes, or upgrade", now.Add(65 * time.Minute)},
		{"2h5m", now.Add(125 * time.Minute)},
		{"90s", now.Add(90 * time.Second)},
		{"1 day 2 hours", now.Add(26 * time.Hour)},
		{"3pm", at(27, 15, 0)},
		{"3:15 PM", at(27, 15, 15)},
		{"11:05 p.m.", at(27, 23, 5)},
		{"15:04", at(27, 15, 4)},
		{"9am", at(28, 9, 0)},     // already past today
		{"12am", at(28, 0, 0)},    // midnight
		{"10:30", at(28, 10, 30)}, // now is not after now
		{"5pm (Europe/Warsaw)", at(27, 16, 0)},
		{"1795780800", time.Unix(1795780800, 0)},
		{"2026-11-27T12:00:00Z.", at(27, 12, 0)},
		{"soon", time.Time{}},
		{"5", time.Time{}}, // a bare number is neither a clock time nor a duration
		{"13pm", time.Time{}},
		{"25:00", time.Time{}},
		{"Nov 28th, 2026 9:00 AM", time.Time{}},
		{"2026-11-27T09:00:00Z", time.Time{}}, // in the past
		{"", time.Time{}},
	}
	for _, tc := range cases {
		got, ok := ParseReset(tc.text, now)
		if !got.Equal(tc.want) || ok == tc.want.IsZero() {
			t.Errorf("ParseReset(%q) = %s, %t; want %s", tc.text, got, ok, tc.want)
		}
	}
}

func TestRateLimitDefaultsCompile(t *testing.T) {
	for surface, rl := range RateLimitDefaults {
		for _, p := range append([]string{rl.ResetPattern}, rl.Patterns...) {
			if _, err := config.CompilePattern(p); err != nil {
				t.Errorf("%s: %q: %v", surface, p, err)
			}
		}
		if _, ok := Defaults[surface]; !ok {
			t.Errorf("%s has rate-limit defaults and no launcher", surface)
		}
	}
}
