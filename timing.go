package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type timingEntry struct {
	major    string
	minor    string
	duration time.Duration
}

type compilationTimings struct {
	enabled bool
	started time.Time
	entries []timingEntry
}

func newCompilationTimings(enabled bool) *compilationTimings {
	t := &compilationTimings{enabled: enabled}
	if enabled {
		t.started = time.Now()
	}
	return t
}

func (t *compilationTimings) start(major, minor string) func() {
	if !t.enabled {
		return func() {}
	}
	started := time.Now()
	return func() {
		t.entries = append(t.entries, timingEntry{
			major: major, minor: minor, duration: time.Since(started),
		})
	}
}

func (t *compilationTimings) report(w io.Writer) {
	if !t.enabled {
		return
	}
	total := time.Since(t.started)
	majorTotals := make(map[string]time.Duration)
	majorOrder := make([]string, 0, len(t.entries))
	for _, entry := range t.entries {
		if _, exists := majorTotals[entry.major]; !exists {
			majorOrder = append(majorOrder, entry.major)
		}
		majorTotals[entry.major] += entry.duration
	}

	categoryWidth := len("Category")
	phaseWidth := len("Phase")
	for _, major := range majorOrder {
		categoryWidth = max(categoryWidth, len(major))
	}
	for _, entry := range t.entries {
		phaseWidth = max(phaseWidth, len(entry.minor))
	}
	categoryWidth = max(categoryWidth, len("Total"))
	separator := "+-" + strings.Repeat("-", categoryWidth) + "-+-" +
		strings.Repeat("-", phaseWidth) + "-+------------+---------+\n"

	fprintf := func(category, phase string, duration time.Duration, percentage float64) {
		fmt.Fprintf(w, "| %-*s | %-*s | %10s | %6.1f%% |\n",
			categoryWidth, category, phaseWidth, phase, formatDuration(duration), percentage)
	}
	percentage := func(duration time.Duration) float64 {
		if total <= 0 {
			return 0
		}
		return float64(duration) * 100 / float64(total)
	}

	fmt.Fprintln(w, "\nCompilation timings")
	fmt.Fprint(w, separator)
	fmt.Fprintf(w, "| %-*s | %-*s | %10s | %7s |\n",
		categoryWidth, "Category", phaseWidth, "Phase", "Time", "Total")
	fmt.Fprint(w, separator)
	for _, major := range majorOrder {
		fprintf(major, "all", majorTotals[major], percentage(majorTotals[major]))
		for _, entry := range t.entries {
			if entry.major == major {
				fprintf("", entry.minor, entry.duration, percentage(entry.duration))
			}
		}
		fmt.Fprint(w, separator)
	}
	fprintf("Total", "wall clock", total, 100)
	fmt.Fprint(w, separator)
}

func formatDuration(duration time.Duration) string {
	if duration < time.Microsecond {
		return duration.Round(time.Nanosecond).String()
	}
	if duration < time.Millisecond {
		return duration.Round(time.Microsecond).String()
	}
	if duration < time.Second {
		return duration.Round(100 * time.Microsecond).String()
	}
	return duration.Round(time.Millisecond).String()
}
