package brand

import (
	"os"
	"testing"
)

func TestNameUsesLaikiHubForLegacyEnvironmentBrand(t *testing.T) {
	old, had := lookupEnvForTest("BUSINESS_NAME")
	t.Cleanup(func() { restoreEnvForTest("BUSINESS_NAME", old, had) })
	for _, legacy := range []string{"Grace's Kitchen", "Grace’s Kitchen", "Grace Ordering Site", "ENIE", "enie"} {
		t.Setenv("BUSINESS_NAME", legacy)
		if got := Name(); got != "LaikiHub" {
			t.Errorf("Name() with legacy value %q = %q, want LaikiHub", legacy, got)
		}
	}
}

func TestNameAllowsOtherConfiguredBrand(t *testing.T) {
	old, had := lookupEnvForTest("BUSINESS_NAME")
	t.Cleanup(func() { restoreEnvForTest("BUSINESS_NAME", old, had) })
	t.Setenv("BUSINESS_NAME", "LaikiHub Market")
	if got := Name(); got != "LaikiHub Market" {
		t.Fatalf("Name() = %q, want LaikiHub Market", got)
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
