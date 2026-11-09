package handlers

import (
	"strings"
	"testing"
)

func TestTrackingTokenIsSignedPerOrder(t *testing.T) {
	t.Setenv("ORDER_TRACKING_SECRET", "test-secret")

	a := trackingToken(1)
	if a == "" {
		t.Fatal("expected a token when a secret is configured")
	}
	if a == trackingToken(2) {
		t.Fatal("different orders must not share a token")
	}
	if !validTrackingToken(1, a) {
		t.Fatal("a freshly made token should validate")
	}
	if validTrackingToken(2, a) {
		t.Fatal("a token must only open its own order")
	}
	if validTrackingToken(1, a[:len(a)-1]+"0") && a[len(a)-1] != '0' {
		t.Fatal("a tampered token must not validate")
	}
}

func TestTrackingTokenNeedsSecret(t *testing.T) {
	t.Setenv("ORDER_TRACKING_SECRET", "")
	t.Setenv("PAYSTACK_SECRET_KEY", "")
	if trackingToken(1) != "" || validTrackingToken(1, "") {
		t.Fatal("without a secret no link may be issued or accepted")
	}
}

func TestTrackingURLUsesPublicBase(t *testing.T) {
	t.Setenv("ORDER_TRACKING_SECRET", "test-secret")
	t.Setenv("APP_BASE_URL", "https://laikihub.example/")
	got := trackingURL(7)
	if !strings.HasPrefix(got, "https://laikihub.example/track/7/") {
		t.Fatalf("unexpected tracking URL %q", got)
	}
	t.Setenv("APP_BASE_URL", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	if trackingURL(7) != "" {
		t.Fatal("no base URL means no emailable link")
	}
}

func TestDescribeTrackStatus(t *testing.T) {
	p := TrackPage{Status: "preparing", IsDelivery: true}
	describeTrackStatus(&p)
	if p.StatusLabel != "Being prepared" || len(p.Steps) != 4 {
		t.Fatalf("unexpected page %+v", p)
	}
	if p.Steps[0].State != "done" || p.Steps[1].State != "current" || p.Steps[2].State != "todo" {
		t.Fatalf("unexpected steps %+v", p.Steps)
	}

	done := TrackPage{Status: "completed"}
	describeTrackStatus(&done)
	if !done.Finished || done.Steps[3].State != "done" || done.Steps[3].Label != "Collected" {
		t.Fatalf("unexpected completed page %+v", done)
	}

	cancelled := TrackPage{Status: "cancelled"}
	describeTrackStatus(&cancelled)
	if !cancelled.Cancelled || !cancelled.Finished {
		t.Fatalf("unexpected cancelled page %+v", cancelled)
	}
}
