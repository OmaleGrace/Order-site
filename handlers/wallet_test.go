package handlers

import "testing"

func TestNormalizeGiftCode(t *testing.T) {
	cases := map[string]string{
		"abcd-efgh-jklm":   "ABCD-EFGH-JKLM",
		" ABCD EFGH JKLM ": "ABCD-EFGH-JKLM",
		"abcdefghjklm":     "ABCD-EFGH-JKLM",
		"short":            "",
		"":                 "",
		"ABCD-EFGH-JKLM-X": "",
	}
	for in, want := range cases {
		if got := normalizeGiftCode(in); got != want {
			t.Errorf("normalizeGiftCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewGiftCodeRoundTrips(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := newGiftCode()
		if err != nil {
			t.Fatal(err)
		}
		if normalizeGiftCode(code) != code {
			t.Fatalf("generated code %q does not survive normalisation", code)
		}
	}
}

func TestPassPriceDefault(t *testing.T) {
	t.Setenv("PASS_PRICE_KOBO", "")
	if passPriceKobo() != 100000 {
		t.Fatalf("expected 100000, got %d", passPriceKobo())
	}
}
