package launch

import (
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/config"
)

// RateLimitDefaults recognise a session stopped by a usage or rate limit,
// per known surface, for each key an agent's rate_limit leaves unset.
//
// opencode's pattern is from session logs of real stopped sessions
// (testdata/opencode-usage-limit*.log); that message names no reset time,
// so the surface has no reset pattern. The codex and claude-code entries
// are the messages those CLIs are known to print, not yet checked against
// a captured log: a project that sees another wording sets rate_limit.
var RateLimitDefaults = map[string]config.RateLimit{
	"opencode": {
		Patterns: []string{`usage limit reached`},
	},
	"codex": {
		Patterns:     []string{`usage limit reached`, `you.ve hit your usage limit`},
		ResetPattern: `try again (?:at|in) ([^.\n]+)`,
	},
	"claude-code": {
		Patterns:     []string{`usage limit reached`, `you.ve hit your (?:\w+ )?limit`},
		ResetPattern: `limit reached\|(\d{9,})|resets?(?: at| in)? ([^.\n]+)`,
	},
}

// RateLimitFor returns an agent's rate_limit with the surface default
// applied to each unset key.
func RateLimitFor(a *config.Agent) config.RateLimit {
	rl := RateLimitDefaults[a.Surface]
	if c := a.RateLimit; c != nil {
		if len(c.ExitCodes) > 0 {
			rl.ExitCodes = c.ExitCodes
		}
		if len(c.Patterns) > 0 {
			rl.Patterns = c.Patterns
		}
		if c.ResetPattern != "" {
			rl.ResetPattern = c.ResetPattern
		}
	}
	return rl
}

// rateLimitTail is how much of the end of a session log is searched: a
// limit ends the session, so its message is last.
const rateLimitTail = 16 * 1024

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// Limited reports whether a failed session was stopped by the agent's
// usage or rate limit: its exit code is listed, or the end of its log
// matches a pattern. A negative exitCode matches no list. reset is when
// the log says the limit ends, or zero when it does not say.
func Limited(a *config.Agent, exitCode int, logPath string, now time.Time) (limited bool, reset time.Time) {
	rl := RateLimitFor(a)
	for _, code := range rl.ExitCodes {
		if exitCode >= 0 && code == exitCode {
			limited = true
		}
	}
	tail := ansiRe.ReplaceAllString(logTail(logPath, rateLimitTail), "")
	for _, p := range rl.Patterns {
		if limited {
			break
		}
		if re, err := config.CompilePattern(p); err == nil && re.MatchString(tail) {
			limited = true
		}
	}
	if !limited || rl.ResetPattern == "" {
		return limited, time.Time{}
	}
	re, err := config.CompilePattern(rl.ResetPattern)
	if err != nil {
		return true, time.Time{}
	}
	// The last match is the limit that ended the session.
	if all := re.FindAllStringSubmatch(tail, -1); len(all) > 0 {
		for _, group := range all[len(all)-1][1:] {
			if group == "" {
				continue
			}
			if t, ok := ParseReset(group, now); ok {
				reset = t
			}
			break
		}
	}
	return true, reset
}

// logTail returns up to the last n bytes of a file, or "".
func logTail(path string, n int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	if info, err := f.Stat(); err == nil && info.Size() > n {
		if _, err := f.Seek(-n, io.SeekEnd); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(data)
}

var (
	stampRe = regexp.MustCompile(`^\d{9,11}\b`)
	spanRe  = regexp.MustCompile(`^(\d+)\s*([a-z]+)[\s,]*(?:and\s+)?`)
	clockRe = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*(a\.?m\b\.?|p\.?m\b\.?)?(?:\s*\(?([A-Za-z_]+(?:/[A-Za-z_]+)+|UTC)\)?)?`)
)

var spanUnits = map[string]time.Duration{
	"d": 24 * time.Hour, "day": 24 * time.Hour, "days": 24 * time.Hour,
	"h": time.Hour, "hr": time.Hour, "hrs": time.Hour, "hour": time.Hour, "hours": time.Hour,
	"m": time.Minute, "min": time.Minute, "mins": time.Minute, "minute": time.Minute, "minutes": time.Minute,
	"s": time.Second, "sec": time.Second, "secs": time.Second, "second": time.Second, "seconds": time.Second,
}

// ParseReset reads when a limit resets from the start of text, as agents
// print it: a Unix timestamp, an RFC 3339 time, a duration from now ("2h
// 5m", "3 hours 12 minutes"), or a clock time ("3pm", "3:15 PM", "15:04",
// each optionally followed by a zone such as "(Europe/London)"), which is
// its next occurrence after now. Anything else, or a time that is not
// after now, is not parsed.
func ParseReset(text string, now time.Time) (time.Time, bool) {
	text = strings.TrimSpace(text)
	t, ok := parseReset(text, now)
	if !ok || !t.After(now) {
		return time.Time{}, false
	}
	return t, true
}

func parseReset(text string, now time.Time) (time.Time, bool) {
	if m := stampRe.FindString(text); m != "" {
		secs, _ := strconv.ParseInt(m, 10, 64)
		return time.Unix(secs, 0), true
	}
	if fields := strings.Fields(text); len(fields) > 0 {
		if t, err := time.Parse(time.RFC3339, strings.TrimRight(fields[0], ".,;)")); err == nil {
			return t, true
		}
	}
	if d, ok := parseSpan(strings.ToLower(text)); ok {
		return now.Add(d), true
	}
	m := clockRe.FindStringSubmatch(text)
	if m == nil || (m[2] == "" && m[3] == "") {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	if half := strings.ToLower(m[3]); half != "" {
		if hour < 1 || hour > 12 {
			return time.Time{}, false
		}
		hour %= 12
		if half[0] == 'p' {
			hour += 12
		}
	}
	if hour > 23 || minute > 59 {
		return time.Time{}, false
	}
	loc := now.Location()
	if m[4] != "" {
		if l, err := time.LoadLocation(m[4]); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	t := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t, true
}

// parseSpan reads a duration written as numbers with units from the start
// of lower-case text.
func parseSpan(text string) (time.Duration, bool) {
	var total time.Duration
	found := false
	for {
		m := spanRe.FindStringSubmatch(text)
		if m == nil {
			return total, found
		}
		unit, ok := spanUnits[m[2]]
		if !ok {
			return total, found
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, false
		}
		total += time.Duration(n) * unit
		found = true
		text = text[len(m[0]):]
	}
}
