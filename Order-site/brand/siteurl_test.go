package brand

import "testing"

func TestSiteURLAcceptsAnyOfTheThreeNames(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"none set", map[string]string{}, ""},
		{"preferred name", map[string]string{"APP_BASE_URL": "https://a.example/"}, "https://a.example"},
		{"older public name", map[string]string{"PUBLIC_BASE_URL": "https://b.example"}, "https://b.example"},
		{"older site name", map[string]string{"SITE_URL": " https://c.example/ "}, "https://c.example"},
		{"preferred wins", map[string]string{"APP_BASE_URL": "https://a.example", "PUBLIC_BASE_URL": "https://b.example", "SITE_URL": "https://c.example"}, "https://a.example"},
		{"blank preferred falls through", map[string]string{"APP_BASE_URL": "  ", "SITE_URL": "https://c.example"}, "https://c.example"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, k := range []string{"APP_BASE_URL", "PUBLIC_BASE_URL", "SITE_URL"} {
				t.Setenv(k, c.env[k]) // empty when not listed; t.Setenv restores the real value afterwards
			}
			if got := SiteURL(); got != c.want {
				t.Fatalf("SiteURL() = %q, want %q", got, c.want)
			}
		})
	}
}
