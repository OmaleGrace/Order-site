package database

import "testing"

func TestParseMigrationName(t *testing.T) {
	good := map[string]struct {
		v    int
		name string
	}{
		"0002_uploaded_images_cloud.sql": {2, "uploaded_images_cloud"},
		"0010_add_thing.sql":             {10, "add_thing"},
	}
	for file, want := range good {
		v, name, err := parseMigrationName(file)
		if err != nil || v != want.v || name != want.name {
			t.Errorf("parseMigrationName(%q) = %d, %q, %v; want %d, %q", file, v, name, err, want.v, want.name)
		}
	}
	for _, file := range []string{"notes.sql", "abc_thing.sql", "0001_baseline.sql", "0000_x.sql", "0005_.sql"} {
		if _, _, err := parseMigrationName(file); err == nil {
			t.Errorf("parseMigrationName(%q) should be rejected", file)
		}
	}
}

func TestLoadMigrationsAreOrderedAndUnique(t *testing.T) {
	list, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 || list[0].version != 1 || list[0].name != "baseline" {
		t.Fatalf("the baseline must come first, got %+v", list)
	}
	for i := 1; i < len(list); i++ {
		if list[i].version <= list[i-1].version {
			t.Fatalf("migrations out of order or duplicated at %d: %+v", i, list)
		}
	}
}
