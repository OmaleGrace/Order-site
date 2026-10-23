package handlers

import (
	"html/template"
	"strings"
	"testing"
)

func TestOrderEmailTemplateParses(t *testing.T) {
	_, err := template.New("order-email").Funcs(template.FuncMap{
		"naira": func(kobo int) int { return kobo / 100 },
	}).Parse(orderEmailTemplate)
	if err != nil {
		t.Fatalf("order email template should parse: %v", err)
	}
}

func TestEmailImageURLUsesConfiguredPublicBaseURL(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://enie.example")
	t.Setenv("APP_BASE_URL", "http://localhost:8080")
	got := emailImageURL("/static/dishes/jollof.jpg")
	if got != "https://enie.example/static/dishes/jollof.jpg" {
		t.Fatalf("unexpected absolute image URL: %q", got)
	}
	if got := emailImageURL("https://cdn.example/dish.jpg"); !strings.EqualFold(got, "https://cdn.example/dish.jpg") {
		t.Fatalf("absolute image URL should be preserved, got %q", got)
	}
}
