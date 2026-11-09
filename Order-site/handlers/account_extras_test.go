package handlers

import "testing"

func TestAccountCompleteness(t *testing.T) {
	if p, m := accountCompleteness(true, true, true, true); p != 100 || len(m) != 0 {
		t.Fatalf("complete profile = %d %v", p, m)
	}
	p, m := accountCompleteness(true, false, true, false)
	if p != 50 || len(m) != 2 || m[0] != "Add a profile photo" {
		t.Fatalf("half profile = %d %v", p, m)
	}
	if p, _ := accountCompleteness(false, false, false, false); p != 0 {
		t.Fatalf("empty profile = %d", p)
	}
}

func TestCleanText(t *testing.T) {
	if got := cleanText("  hello  ", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := cleanText("ééééé", 3); got != "ééé" {
		t.Fatalf("got %q", got)
	}
}
