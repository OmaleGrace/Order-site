package brand

import (
	"os"
	"testing"
)

func TestNameUsesENIEForLegacyEnvironmentBrand(t *testing.T) {
	old, had := lookupEnvForTest("BUSINESS_NAME")
	t.Cleanup(func() { restoreEnvForTest("BUSINESS_NAME", old, had) })
	for _, legacy := range []string{"Grace's Kitchen", "Grace’s Kitchen", "Grace Ordering Site"} {
		t.Setenv("BUSINESS_NAME", legacy)
		if got := Name(); got != "ENIE" {
			t.Errorf("Name() with legacy value %q = %q, want ENIE", legacy, got)
		}
	}
}

func TestNameAllowsOtherConfiguredBrand(t *testing.T) {
	old, had := lookupEnvForTest("BUSINESS_NAME")
	t.Cleanup(func() { restoreEnvForTest("BUSINESS_NAME", old, had) })
	t.Setenv("BUSINESS_NAME", "ENIE Market")
	if got := Name(); got != "ENIE Market" {
		t.Fatalf("Name() = %q, want ENIE Market", got)
	}
}

func lookupEnvForTest(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	return v, ok
}

func restoreEnvForTest(key, value string, existed bool) {
	if existed {
		_ = os.Setenv(key, value)
	} else {
		_ = os.Unsetenv(key)
	}
}
