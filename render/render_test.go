package render

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"testing"
)

var missingFunc = regexp.MustCompile(`function "([^"]+)" not defined`)

// parseWithStubs parses a page the way a handler would. Handlers add their own template
// functions (naira, money, ...); this test only checks that the page parses, so whenever
// the parser reports a missing function it registers a do-nothing stand-in and retries.
// That way a new page function never needs a change here.
func parseWithStubs(name string) error {
	funcs := template.FuncMap{}
	for attempt := 0; attempt < 50; attempt++ {
		_, err := Page(name, funcs)
		if err == nil {
			return nil
		}
		m := missingFunc.FindStringSubmatch(err.Error())
		if m == nil {
			return err
		}
		funcs[m[1]] = func(...any) any { return nil }
	}
	return fmt.Errorf("%s: too many missing template functions", name)
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
		if err := parseWithStubs(name); err != nil {
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
