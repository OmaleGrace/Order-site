package database

import (
	"strings"
	"testing"
)

func TestCleanSchemaDumpRemovesPSQLAndSessionCommands(t *testing.T) {
	input := strings.Join([]string{
		`\restrict example`,
		`-- pg_dump comment`,
		`SET statement_timeout = 0;`,
		`SELECT pg_catalog.set_config('search_path', '', false);`,
		`CREATE SCHEMA public;`,
		`COMMENT ON SCHEMA public IS 'standard public schema';`,
		`CREATE TABLE public.users (id integer NOT NULL);`,
		`\unrestrict example`,
	}, "\n")
	got := cleanSchemaDump(input)
	if strings.Contains(got, `\restrict`) || strings.Contains(got, "SET statement_timeout") || strings.Contains(got, "CREATE SCHEMA public") {
		t.Fatalf("expected psql/session setup to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "CREATE TABLE public.users") {
		t.Fatalf("expected schema SQL to be retained, got:\n%s", got)
	}
}

func TestCleanSchemaDumpEmpty(t *testing.T) {
	if got := cleanSchemaDump("-- only comment\nSET statement_timeout = 0;"); got != "" {
		t.Fatalf("expected empty cleaned schema, got %q", got)
	}
}

func TestPoolerSafe(t *testing.T) {
	cases := map[string]string{
		"postgresql://u:p@host:6543/db":                  "postgresql://u:p@host:6543/db?binary_parameters=yes",
		"postgresql://u:p@host/db?sslmode=require":       "postgresql://u:p@host/db?binary_parameters=yes&sslmode=require",
		"postgresql://u:p@host/db?binary_parameters=no":  "postgresql://u:p@host/db?binary_parameters=no",
		"host=localhost user=u dbname=d sslmode=disable": "host=localhost user=u dbname=d sslmode=disable binary_parameters=yes",
		"host=localhost dbname=d binary_parameters=no":   "host=localhost dbname=d binary_parameters=no",
	}
	for in, want := range cases {
		if got := poolerSafe(in); got != want {
			t.Errorf("poolerSafe(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}
