package main

import (
	"strings"
	"testing"
)

func TestErrorTraceSlotsOption(t *testing.T) {
	opts, err := parseArgs([]string{"--error-trace-slots", "2048", "input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.errorTraceSlots != 2048 {
		t.Fatalf("errorTraceSlots = %d, want 2048", opts.errorTraceSlots)
	}
}

func TestErrorTraceSlotsDefault(t *testing.T) {
	opts, err := parseArgs([]string{"input.mg"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.errorTraceSlots != 1024 {
		t.Fatalf("errorTraceSlots = %d, want 1024", opts.errorTraceSlots)
	}
}

func TestErrorTraceSlotsValidation(t *testing.T) {
	for _, value := range []string{"0", "3", "65537"} {
		_, err := parseArgs([]string{"--error-trace-slots", value, "input.mg"})
		if err == nil || !strings.Contains(err.Error(), "--error-trace-slots") {
			t.Errorf("value %s: error = %v", value, err)
		}
	}
}
