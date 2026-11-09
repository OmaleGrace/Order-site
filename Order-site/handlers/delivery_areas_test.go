package handlers

import "testing"

func TestParseDeliveryAreas(t *testing.T) {
	got, err := parseDeliveryAreas("lagos | Ikeja\nLagos | Surulere\nLAGOS | ikeja\nFCT (Abuja) | Abuja Municipal")
	if err != nil {
		t.Fatalf("parseDeliveryAreas returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 unique areas, got %d: %#v", len(got), got)
	}
	if got[0][0] != "Lagos" || got[0][1] != "Ikeja" {
		t.Fatalf("expected canonical state and trimmed LGA, got %#v", got[0])
	}
}

func TestParseDeliveryAreasRejectsInvalidLines(t *testing.T) {
	cases := []string{
		"Lagos Ikeja",
		"Atlantis | Ikeja",
		"Lagos | ",
	}
	for _, input := range cases {
		if _, err := parseDeliveryAreas(input); err == nil {
			t.Errorf("expected %q to be rejected", input)
		}
	}
}

func TestCanonicalState(t *testing.T) {
	state, ok := canonicalState("  lAgOs ")
	if !ok || state != "Lagos" {
		t.Fatalf("expected Lagos, got %q, %v", state, ok)
	}
	state, ok = canonicalState("Abuja")
	if !ok || state != "FCT (Abuja)" {
		t.Fatalf("expected FCT (Abuja), got %q, %v", state, ok)
	}
}
