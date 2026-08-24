package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/brohd11/goutil/stream"
)

// The transfer flow streams scp, whose progress meter is carriage-return delimited: each
// update overwrites the previous one with \r and no newline. This is the reason the flow
// uses goutil/stream rather than a bufio.Scanner — Scanner's default split is \n only, so
// a whole meter arrives as a single unreadable line (and the last update, having no
// trailing newline at all, arrives only because Scanner flushes at EOF).
//
// The assertion is on stream.Cmd because that is the contract the flow now depends on;
// if it ever regresses to \n-only splitting, the copy progress silently stops moving.
func TestScpProgressArrivesLineByLine(t *testing.T) {
	var lines []string
	report := func(format string, args ...any) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}

	// Three CR-separated meter updates and a final newline-terminated summary, the shape
	// scp actually emits.
	const out = `file.iso   10%  100MB` + "\r" +
		`file.iso   55%  550MB` + "\r" +
		`file.iso  100%  1.0GB` + "\r" +
		"done\n"

	if err := stream.Cmd(context.Background(), "", nil, report, "printf", "%s", out); err != nil {
		t.Fatalf("stream.Cmd: %v", err)
	}

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (three meter updates + summary): %q", len(lines), lines)
	}
	for i, want := range []string{"10%", "55%", "100%", "done"} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d = %q, want it to contain %q", i, lines[i], want)
		}
	}
}
