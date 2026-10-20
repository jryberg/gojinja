package main

import "testing"

func TestArchivePath(t *testing.T) {
	ok := []struct{ name, want string }{
		{"pallets-jinja-5ef7011", ""},
		{"pallets-jinja-5ef7011/", ""},
		{"pallets-jinja-5ef7011/LICENSE.txt", "LICENSE.txt"},
		{"pallets-jinja-5ef7011/tests/res/templates/foo.html", "tests/res/templates/foo.html"},
		{"pallets-jinja-5ef7011/docs/a..b.html", "docs/a..b.html"},
	}
	for _, tc := range ok {
		got, err := archivePath(tc.name)
		if err != nil || got != tc.want {
			t.Errorf("archivePath(%q) = %q, %v; want %q, nil", tc.name, got, err, tc.want)
		}
	}
	for _, name := range []string{
		"pallets-jinja-5ef7011/tests/res/templates/../../../../evil.html",
		"pallets-jinja-5ef7011/../evil.html",
		"pallets-jinja-5ef7011/..",
		"pallets-jinja-5ef7011//etc/evil.html",
	} {
		if got, err := archivePath(name); err == nil {
			t.Errorf("archivePath(%q) = %q, nil; want an error", name, got)
		}
	}
}
