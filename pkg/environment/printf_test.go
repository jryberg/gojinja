package environment

import (
	"context"
	"math"
	"testing"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt (CPython's str % formatting).

// Port of TestFilter.test_format plus printf-style formatting through
// both the % operator and the format filter: tuple, mapping and single
// arguments, flags, width/precision (including *), every conversion,
// Markup escaping, and CPython's errors.
func TestPercentFormatting(t *testing.T) {
	cases := []struct {
		src     string
		want    string
		wantErr bool
	}{
		{`{{ "%s|%s"|format("a", "b") }}`, "a|b", false},
		{`{{ '%s-%d' % ('a', 2) }}`, "a-2", false},
		{`{{ '%s' % 'x' }}`, "x", false},
		{`{{ '%s' % [1, 2] }}`, "[1, 2]", false},
		{`{{ '%s %s' % [1, 2] }}`, "", true},
		{`{{ '%s' % {'a': 1} }}`, "{'a': 1}", false},
		{`{{ '%(a)s and %(b)05.1f' % {'a': 'x', 'b': 2.25} }}`, "x and 002.2", false},
		{`{{ 'hello' % 5 }}`, "", true},
		{`{{ 'hello' % {'a': 1} }}`, "hello", false},
		{`{{ 'hello' % [1] }}`, "hello", false},
		{`{{ '%s %s' % (1,) }}`, "", true},
		{`{{ '%s' % (1, 2) }}`, "", true},
		{`{{ '%d|%i|%u|%5d|%-5d|%05d|%+d|% d|%.3d|%+.3d' % (42, 42, 42, 42, 42, 42, 42, 42, 5, -5) }}`, "42|42|42|   42|42   |00042|+42| 42|005|-005", false},
		{`{{ '%d' % 3.9 }}|{{ '%d' % -3.9 }}|{{ '%d' % true }}`, "3|-3|1", false},
		{`{{ '%d' % 'x' }}`, "", true},
		{`{{ '%x|%X|%#x|%#X|%o|%#o|%08x|%-#8x|' % (255, 255, 255, 255, 8, 8, 255, 255) }}`, "ff|FF|0xff|0XFF|10|0o10|000000ff|0xff    |", false},
		{`{{ '%x' % 2.0 }}`, "", true},
		{`{{ '%x' % -255 }}|{{ '%#o' % -8 }}`, "-ff|-0o10", false},
		{`{{ '%f|%.2f|%10.3f|%-10.1f|%+f|% f|%010.2f|%.0f|%#.0f' % (3.14159, 3.14159, 3.14159, 3.14159, 1.5, 1.5, -1.5, 2.5, 2.5) }}`, "3.141590|3.14|     3.142|3.1       |+1.500000| 1.500000|-000001.50|2|2.", false},
		{`{{ '%e|%E|%.2e|%10.1e|%#.0e' % (12345.678, 12345.678, 0.000123, 5, 3) }}`, "1.234568e+04|1.234568E+04|1.23e-04|   5.0e+00|3.e+00", false},
		{`{{ '%g|%G|%.3g|%g|%g|%g|%g|%#g|%.0g|%g' % (0.0001, 1e-05, 3.14159, 100000, 1000000, 1.5e300, 0, 1.5, 123, -0.0) }}`, "0.0001|1E-05|3.14|100000|1e+06|1.5e+300|0|1.50000|1e+02|-0", false},
		{`{{ '%f|%e|%g|%F|%E|%G' % (inf, inf, nan, inf, -inf, nan) }}`, "inf|inf|nan|INF|-INF|NAN", false},
		{`{{ '%5.1f|%05f' % (inf, inf) }}`, "  inf|00inf", false},
		{`{{ '%f' % 'x' }}`, "", true},
		{`{{ '%c|%c|%5c' % (65, 'z', 'q') }}`, "A|z|    q", false},
		{`{{ '%c' % 'ab' }}`, "", true},
		{`{{ '%r|%a|%5r|%.3r' % ('é', 'é', 1, 'abcdef') }}`, "'\u00e9'|'\\xe9'|    1|'ab", false},
		{`{{ '%s|%10s|%-10s|%.2s|%10.2s' % ('abc', 'abc', 'abc', 'abc', 'abc') }}`, "abc|       abc|abc       |ab|        ab", false},
		{`{{ '%*d|%-*d|%.*f|%*.*f' % (5, 1, 5, 1, 2, 3.14159, 8, 2, 3.14159) }}`, "    1|1    |3.14|    3.14", false},
		{`{{ '100%%' % () }}|{{ '%%s' % () }}`, "100%|%s", false},
		{`{{ '%' % () }}`, "", true},
		{`{{ '%q' % 1 }}`, "", true},
		{`{{ '%(a' % {'a': 1} }}`, "", true},
		{`{{ '%(a)s' % (1,) }}`, "", true},
		{`{{ '%(missing)s' % {'a': 1} }}`, "", true},
		{`{{ '%(a)s %s' % {'a': 1} }}`, "", true},
		{`{{ '%(a(b))s' % {'a(b)': 2} }}`, "2", false},
		{`{{ '%ld|%hd|%Lf' % (1, 2, 3.5) }}`, "1|2|3.500000", false},
		{`{{ '%s' % none }}|{{ '%s' % true }}|{{ '%s' % (none,) }}`, "None|True|None", false},
		{`{{ '%s|' % q }}|{{ 'x' % q }}`, "||x", false},
		{`{{ '%d' % q }}`, "", true},
		{`{{ '%(a)s' % q }}`, "", true},
		{`{{ '%s %s'|format(1, 'a') }}`, "1 a", false},
		{`{{ '%(x)s-%(y)d'|format(x='a', y=3) }}`, "a-3", false},
		{`{{ 1|format(2) }}`, "", true},
		{`{{ '%d'|format('x') }}`, "", true},
		{`{{ '%s %s'|format(1) }}`, "", true},
		{`{{ '%s'|format([1]) }}`, "[1]", false},
		{`{{ ('%s<' ~ '')|format('<>') }}`, "<><", false},
		{`{{ ('<%s>'|e)|format('<&>') }}`, "&lt;&lt;&amp;&gt;&gt;", false},
		{`{{ ('%s|%r|%d|%.1f'|e)|format('<', '<', 3, 2.5) }}`, "&lt;|&#39;&lt;&#39;|3|2.5", false},
		{`{{ ('<b>%s</b>'|e) % '<i>' }}`, "&lt;b&gt;&lt;i&gt;&lt;/b&gt;", false},
		{`{{ ('<b>%(x)s</b>'|e) % {'x': '<i>'} }}`, "&lt;b&gt;&lt;i&gt;&lt;/b&gt;", false},
		{`{{ ('%s'|e) % ('a&b'|e) }}`, "a&amp;b", false},
		{`{{ 'é%5sß' % 'ü' }}`, "\u00e9    \u00fc\u00df", false},
		{`{{ '%5s|' % 'éé' }}`, "   \u00e9\u00e9|", false},
		{`{{ '%s' % (1, 2)[0] }}`, "1", false},
		{`{{ '%s %(a)s' % {'a': 1} }}`, "{'a': 1} 1", false},
		{`{{ '%(a)s %(b)s' % {'a': 1, 'b': 2} }}`, "1 2", false},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		got, err := tpl.RenderContext(context.Background(), map[string]any{"inf": math.Inf(1), "nan": math.NaN()})
		switch {
		case c.wantErr && err == nil:
			t.Errorf("%s = %q, want an error", c.src, got)
		case !c.wantErr && err != nil:
			t.Errorf("%s: unexpected error %v", c.src, err)
		case !c.wantErr && got != c.want:
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}
