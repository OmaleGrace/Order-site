// Package render loads the site's HTML templates.
//
// Every page is parsed together with the shared partials in templates/partials
// (head, navigation, scripts, ...) and cached, so a page is parsed once instead
// of on every request.
package render

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"sync"

	"Order-site/brand"
	"Order-site/templates"
)

var cache sync.Map // page name -> *template.Template

// devDir returns the directory to read templates from when TEMPLATES_DIR is set.
// That switches off the cache so edits show up on refresh while developing:
//
//	TEMPLATES_DIR=templates go run .
func devDir() string { return os.Getenv("TEMPLATES_DIR") }

func source() fs.FS {
	if dir := devDir(); dir != "" {
		return os.DirFS(dir)
	}
	return templates.FS
}

// baseFuncs are available in every page.
func baseFuncs() template.FuncMap {
	f := brand.Funcs()
	f["dict"] = dict
	return f
}

// dict builds a map for passing several values to a partial:
//
//	{{template "admin-nav" dict "Class" "nav" "Active" "orders"}}
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key %v is not a string", pairs[i])
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// Page returns the template for a page such as "home.html", parsed together with
// the shared partials. extra holds page-specific template functions; they must not
// depend on the request, because the parsed page is cached and shared.
func Page(name string, extra ...template.FuncMap) (*template.Template, error) {
	dev := devDir() != ""
	if !dev {
		if t, ok := cache.Load(name); ok {
			return t.(*template.Template), nil
		}
	}

	t := template.New(name).Funcs(baseFuncs())
	for _, f := range extra {
		t = t.Funcs(f)
	}
	t, err := t.ParseFS(source(), "partials/*.html", name)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	if dev {
		return t, nil
	}
	actual, _ := cache.LoadOrStore(name, t)
	return actual.(*template.Template), nil
}

// MustPage is like Page but panics on error. Use it for pages that are always
// shipped with the binary, where a failure is a programming mistake.
func MustPage(name string, extra ...template.FuncMap) *template.Template {
	t, err := Page(name, extra...)
	if err != nil {
		panic(err)
	}
	return t
}

// PageNames lists every page template (not partials) in the embedded files.
func PageNames() ([]string, error) {
	entries, err := fs.ReadDir(source(), ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 5 && e.Name()[len(e.Name())-5:] == ".html" {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
