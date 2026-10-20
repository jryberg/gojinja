package unicodename

import "testing"

// Expected values come from the Python running the Jinja2 pinned in
// tools/parity/requirements.txt (unicodedata.name and "\\N{...}").
func TestName(t *testing.T) {
	cases := []struct {
		r    rune
		want string
	}{
		{0x20, "SPACE"},
		{0x41, "LATIN CAPITAL LETTER A"},
		{0xe9, "LATIN SMALL LETTER E WITH ACUTE"},
		{0x2668, "HOT SPRINGS"},
		{0xac01, "HANGUL SYLLABLE GAG"},
		{0xd7a3, "HANGUL SYLLABLE HIH"},
		{0x4e00, "CJK UNIFIED IDEOGRAPH-4E00"},
		{0x9fff, "CJK UNIFIED IDEOGRAPH-9FFF"},
		{0x20000, "CJK UNIFIED IDEOGRAPH-20000"},
		{0x323af, "CJK UNIFIED IDEOGRAPH-323AF"},
		{0xf900, "CJK COMPATIBILITY IDEOGRAPH-F900"},
		{0x1b170, "NUSHU CHARACTER-1B170"},
		{0x18b00, "KHITAN SMALL SCRIPT CHARACTER-18B00"},
		{0x1f600, "GRINNING FACE"},
		{0x0, ""},
		{0x7f, ""},
		{0xe000, ""},
		{0x10ffff, ""},
		{0x1f1e6, "REGIONAL INDICATOR SYMBOL LETTER A"},
		{0xfe0f, "VARIATION SELECTOR-16"},
	}
	for _, c := range cases {
		if got, _ := Name(c.r); got != c.want {
			t.Errorf("Name(%U) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		name string
		want rune // -1: not found
	}{
		{"HOT SPRINGS", 0x2668},
		{"hot springs", 0x2668},
		{"LATIN CAPITAL LETTER GHA", 0x1a2},
		{"latin capital letter gha", 0x1a2},
		{"BYTE ORDER MARK", 0xfeff},
		{"NULL", 0x0},
		{"HANGUL SYLLABLE GAG", 0xac01},
		{"hangul syllable GAG", -1},
		{"HANGUL SYLLABLE gag", -1},
		{"HANGUL SYLLABLE A", 0xc544},
		{"CJK UNIFIED IDEOGRAPH-4E00", 0x4e00},
		{"CJK UNIFIED IDEOGRAPH-04E00", 0x4e00},
		{"CJK UNIFIED IDEOGRAPH-4e00", -1},
		{"cjk unified ideograph-4E00", -1},
		{"CJK UNIFIED IDEOGRAPH-A000", -1},
		{"CJK COMPATIBILITY IDEOGRAPH-F900", 0xf900},
		{"cjk compatibility ideograph-f900", 0xf900},
		{"NUSHU CHARACTER-1B170", 0x1b170},
		{"NUSHU CHARACTER-01B170", -1},
		{"NOPE", -1},
		{"", -1},
		{"LATIN SMALL LETTER A ", -1},
	}
	for _, c := range cases {
		r, ok := Lookup(c.name)
		if !ok {
			r = -1
		}
		if r != c.want {
			t.Errorf("Lookup(%q) = %d, want %d", c.name, r, c.want)
		}
	}
}
