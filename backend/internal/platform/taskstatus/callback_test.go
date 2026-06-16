package taskstatus

import "testing"

func TestShouldApplyCallbackRejectsStaleAndTerminalTransitions(t *testing.T) {
	for _, tc := range []struct {
		current string
		next    string
		want    bool
	}{
		{current: "queued", next: "queued", want: true},
		{current: "queued", next: "running", want: true},
		{current: "queued", next: "done", want: true},
		{current: "running", next: "queued", want: false},
		{current: "running", next: "running", want: true},
		{current: "running", next: "failed", want: true},
		{current: "done", next: "running", want: false},
		{current: "failed", next: "done", want: false},
		{current: "cancelled", next: "running", want: false},
		{current: "unknown", next: "done", want: false},
		{current: "queued", next: " ", want: false},
	} {
		if got := ShouldApplyCallback(tc.current, tc.next); got != tc.want {
			t.Fatalf("expected %q -> %q apply=%v, got %v", tc.current, tc.next, tc.want, got)
		}
	}
}
