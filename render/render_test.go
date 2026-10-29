package render

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// Functions that individual handlers add to their pages. They are stubs here:
// the test only checks that every page parses and that its partials resolve.
var stubFuncs = template.FuncMap{
	"naira": func(int) int { return 0 }, "money": func(int) string { return "" },
	"money2": func(int) string { return "" }, "date": func(any) string { return "" },
	"dateTime": func(any) string { return "" }, "sub": func(a, b int) int { return 0 },
	"sub1": func(int) int { return 0 }, "multiply": func(a, b int) int { return 0 },
	"abs": func(int) int { return 0 }, "add": func(a, b int) int { return 0 },
	"percent": func(a, b int) int { return 0 }, "commissionPercent": func() int { return 0 },
	"formatChatTime": func(any) string { return "" },
}

// Every page template must parse together with the shared partials. This catches
// a typo in a template at test time instead of when a visitor opens the page.
func TestEveryPageParses(t *testing.T) {
	names, err := PageNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 30 {
		t.Fatalf("expected the site's page templates to be embedded, found %d", len(names))
	}
	for _, name := range names {
		if _, err := Page(name, stubFuncs); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPageIsCached(t *testing.T) {
	a, err := Page("error.html")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Page("error.html")
	if a != b {
		t.Fatal("expected the second call to return the cached template")
	}
}

func TestDict(t *testing.T) {
	m, err := dict("Class", "nav", "Active", "orders")
	if err != nil || m["Class"] != "nav" || m["Active"] != "orders" {
		t.Fatalf("dict = %v, %v", m, err)
	}
	if _, err := dict("odd"); err == nil {
		t.Fatal("expected an error for an odd number of arguments")
	}
}

// The shared admin and vendor navigation marks exactly one link as the current page.
func TestNavPartialsMarkActivePage(t *testing.T) {
	tpl, err := Page("error.html")
	if err != nil {
		t.Fatal(err)
	}
	for name, active := range map[string]string{"admin-nav": "/admin/orders", "vendor-nav": "/vendor/coupons"} {
		var out bytes.Buffer
		if err := tpl.ExecuteTemplate(&out, name, map[string]any{"Class": "portal-nav", "Active": active}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		html := out.String()
		if n := strings.Count(html, `aria-current="page"`); n != 1 {
			t.Errorf("%s: %d links marked current, want 1", name, n)
		}
		if !strings.Contains(html, `href="`+active+`" class="active" aria-current="page"`) {
			t.Errorf("%s: %s is not marked as the current page:\n%s", name, active, html)
		}
		if !strings.Contains(html, `class="portal-nav"`) {
			t.Errorf("%s: missing nav class", name)
		}
	}
}
