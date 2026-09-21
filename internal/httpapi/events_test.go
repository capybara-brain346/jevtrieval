package httpapi

import "testing"

func TestEventCursorParsing(t *testing.T) {
	if got := maxEventID(parseEventID("41"), parseEventID("42")); got != 42 {
		t.Fatalf("got %d", got)
	}
	if got := parseEventID("not-an-id"); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := parseEventID("-1"); got != 0 {
		t.Fatalf("got %d", got)
	}
}
