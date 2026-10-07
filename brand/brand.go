package brand

import (
	"html/template"
	"os"
	"path/filepath"
)

// Name returns the business name for this copy of the app.
func Name() string {
	if n := os.Getenv("BUSINESS_NAME"); n != "" {
		return n
	}
	return "Grace's Kitchen"
}

// Funcs makes {{brandName}} available inside templates.
func Funcs() template.FuncMap {
	return template.FuncMap{"brandName": Name}
}

// Parse loads template files with {{brandName}} available.
// The first file's name becomes the template's name.
func Parse(files ...string) *template.Template {
	return template.Must(
		template.New(filepath.Base(files[0])).
			Funcs(Funcs()).
			ParseFiles(files...),
	)
}

// ParseE is like Parse but returns the error instead of panicking.
func ParseE(files ...string) (*template.Template, error) {
	return template.New(filepath.Base(files[0])).
		Funcs(Funcs()).
		ParseFiles(files...)
}