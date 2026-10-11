package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// autoEvent is one line of .metis/runs/auto.jsonl: a run starting, a
// session, a slice archived or merged, or the stop that ended the run.
type autoEvent struct {
	Time  string `json:"time"`
	Run   string `json:"run"`
	Event string `json:"event"` // start, session, done, stop

	Slice string `json:"slice,omitempty"`

	// start
	Mode  string `json:"mode,omitempty"`
	Until string `json:"until,omitempty"`

	// session
	Session          int      `json:"session,omitempty"`
	Role             string   `json:"role,omitempty"`
	Agent            string   `json:"agent,omitempty"`
	Model            string   `json:"model,omitempty"`
	Template         string   `json:"template,omitempty"`
	DurationSeconds  float64  `json:"duration_s,omitempty"`
	ExitCode         *int     `json:"exit_code,omitempty"`
	TimedOut         bool     `json:"timed_out,omitempty"`
	Outcome          string   `json:"outcome,omitempty"`
	FindingsOpened   []string `json:"findings_opened,omitempty"`
	FindingsResolved []string `json:"findings_resolved,omitempty"`
	Log              string   `json:"log,omitempty"`

	// done
	ReviewCycles *int   `json:"review_cycles,omitempty"`
	Merge        string `json:"merge,omitempty"`

	// stop
	Rule       string `json:"rule,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Exit       *int   `json:"exit,omitempty"`
	Escalation string `json:"escalation,omitempty"`
}

func (c *context) autoLogPath() string {
	return filepath.Join(c.repoRoot, c.cfg.Paths.Runs, "auto.jsonl")
}

// appendAutoEvent appends one event; the run log is observability, so a
// write failure warns and the run goes on.
func (c *context) appendAutoEvent(e *autoEvent) {
	if e.Time == "" {
		e.Time = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(e)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(c.autoLogPath()), 0o755)
	}
	if err == nil {
		var f *os.File
		f, err = os.OpenFile(c.autoLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, err = f.Write(append(data, '\n'))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: run log not written (%v)\n", err)
	}
}

// readAutoEvents reads the whole run log; unreadable lines are skipped.
func (c *context) readAutoEvents() []autoEvent {
	f, err := os.Open(c.autoLogPath())
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var events []autoEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var e autoEvent
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			events = append(events, e)
		}
	}
	return events
}

// sliceBudgetUse counts a slice's sessions and their wall clock since its
// last stop: a stop means a person looked, so the budget starts again.
func sliceBudgetUse(events []autoEvent, sliceID string) (sessions int, wall time.Duration) {
	for i := range events {
		e := &events[i]
		if e.Slice != sliceID {
			continue
		}
		switch e.Event {
		case "stop":
			sessions, wall = 0, 0
		case "session":
			sessions++
			wall += time.Duration(e.DurationSeconds * float64(time.Second))
		}
	}
	return sessions, wall
}

// autoStatus summarises the last run in the log.
type autoStatus struct {
	Run      string           `json:"run"`
	Started  string           `json:"started"`
	Mode     string           `json:"mode,omitempty"`
	Until    string           `json:"until,omitempty"`
	Done     []string         `json:"slices_done"`
	Sessions int              `json:"sessions"`
	Slices   []autoSliceCount `json:"slices"`
	Stop     *autoEvent       `json:"stop,omitempty"`
}

type autoSliceCount struct {
	Slice        string `json:"slice"`
	Sessions     int    `json:"sessions"`
	ReviewCycles int    `json:"review_cycles"`
}

func lastAutoRun(events []autoEvent) *autoStatus {
	start := -1
	for i := range events {
		if events[i].Event == "start" {
			start = i
		}
	}
	if start < 0 {
		return nil
	}
	st := &autoStatus{Run: events[start].Run, Started: events[start].Time, Mode: events[start].Mode, Until: events[start].Until, Done: []string{}, Slices: []autoSliceCount{}}
	index := map[string]int{}
	count := func(id string) *autoSliceCount {
		if i, ok := index[id]; ok {
			return &st.Slices[i]
		}
		index[id] = len(st.Slices)
		st.Slices = append(st.Slices, autoSliceCount{Slice: id})
		return &st.Slices[len(st.Slices)-1]
	}
	for i := start + 1; i < len(events); i++ {
		e := events[i]
		if e.Run != st.Run {
			continue
		}
		switch e.Event {
		case "session":
			st.Sessions++
			count(e.Slice).Sessions++
		case "done":
			st.Done = append(st.Done, e.Slice)
			if e.ReviewCycles != nil {
				count(e.Slice).ReviewCycles = *e.ReviewCycles
			}
		case "stop":
			stop := e
			st.Stop = &stop
		}
	}
	return st
}
