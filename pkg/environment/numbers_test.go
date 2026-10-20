package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestNumbers holds Python's unbounded ints, complex numbers and float arithmetic checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestNumbers(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "Number1", src: `{{ 2 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number2", src: `{{ 2 ** 64 - 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `18446744073709551615`, err: ``},
		{name: "Number3", src: `{{ -2 ** 63 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-9223372036854775808`, err: ``},
		{name: "Number4", src: `{{ (-2) ** 63 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-9223372036854775808`, err: ``},
		{name: "Number5", src: `{{ 9223372036854775807 + 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9223372036854775808`, err: ``},
		{name: "Number6", src: `{{ -9223372036854775808 - 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-9223372036854775809`, err: ``},
		{name: "Number7", src: `{{ 9223372036854775807 * 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `18446744073709551614`, err: ``},
		{name: "Number8", src: `{{ 123456789012345678901234567890 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `123456789012345678901234567890`, err: ``},
		{name: "Number9", src: `{{ -123456789012345678901234567890 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-123456789012345678901234567890`, err: ``},
		{name: "Number10", src: `{{ 123456789012345678901234567890 // 7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `17636684144620811271604938270`, err: ``},
		{name: "Number11", src: `{{ 123456789012345678901234567890 % 7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Number12", src: `{{ -123456789012345678901234567890 // 7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-17636684144620811271604938270`, err: ``},
		{name: "Number13", src: `{{ -123456789012345678901234567890 % 7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Number14", src: `{{ 123456789012345678901234567890 // -7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-17636684144620811271604938270`, err: ``},
		{name: "Number15", src: `{{ 123456789012345678901234567890 / 7 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.763668414462081e+28`, err: ``},
		{name: "Number16", src: `{{ 10 ** 400 / 10 ** 399 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `10.0`, err: ``},
		{name: "Number17", src: `{{ 2 ** 1024 + 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int too large to convert to float`},
		{name: "Number18", src: `{{ 2 ** 1024 * 1.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int too large to convert to float`},
		{name: "Number19", src: `{{ (2 ** 53 + 1) / 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9007199254740992.0`, err: ``},
		{name: "Number20", src: `{{ 2 ** 1023 * 2 / 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `integer division result too large for a float`},
		{name: "Number21", src: `{{ 2 ** 100 == 2.0 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number22", src: `{{ 2 ** 100 < 2.0 ** 101 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number23", src: `{{ 2 ** 100 > 1e30 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number24", src: `{{ 2 ** 64 == 18446744073709551616 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number25", src: `{{ {2 ** 100: 1, 2.0 ** 100: 2} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{1267650600228229401496703205376: 2}`, err: ``},
		{name: "Number26", src: `{{ {2 ** 70: 'a'}[2.0 ** 70] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `a`, err: ``},
		{name: "Number27", src: `{{ (2 ** 100)|abs }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number28", src: `{{ (-2 ** 100)|abs }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number29", src: `{{ '%d' % 2 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number30", src: `{{ '%x' % 2 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `10000000000000000000000000`, err: ``},
		{name: "Number31", src: `{{ '%e' % 2 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.267651e+30`, err: ``},
		{name: "Number32", src: `{{ '{:,}'.format(2 ** 100) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1,267,650,600,228,229,401,496,703,205,376`, err: ``},
		{name: "Number33", src: `{{ '{:x}'.format(2 ** 100) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `10000000000000000000000000`, err: ``},
		{name: "Number34", src: `{{ '{:.3e}'.format(2 ** 100) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.268e+30`, err: ``},
		{name: "Number35", src: `{{ (2 ** 100)|string|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `31`, err: ``},
		{name: "Number36", src: `{{ '123456789012345678901234567890'|int }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `123456789012345678901234567890`, err: ``},
		{name: "Number37", src: `{{ (2 ** 100)|float }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.2676506002282294e+30`, err: ``},
		{name: "Number38", src: `{{ (2 ** 2000)|float }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int too large to convert to float`},
		{name: "Number39", src: `{{ (2 ** 100)|round }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number40", src: `{{ (2 ** 100)|round(-20) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600200000000000000000000`, err: ``},
		{name: "Number41", src: `{{ [2 ** 100, 1]|sum }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205377`, err: ``},
		{name: "Number42", src: `{{ [2 ** 100, 1]|max }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number43", src: `{{ [2 ** 100, 2 ** 99]|sort }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[633825300114114700748351602688, 1267650600228229401496703205376]`, err: ``},
		{name: "Number44", src: `{{ (2 ** 100)|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number45", src: `{{ (2 ** 100)|filesizeformat }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650.6 YB`, err: ``},
		{name: "Number46", src: `{{ 2 ** -1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.5`, err: ``},
		{name: "Number47", src: `{{ 0 ** -1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `0.0 cannot be raised to a negative power`},
		{name: "Number48", src: `{{ (-8) ** (1/3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(1.0000000000000002+1.7320508075688772j)`, err: ``},
		{name: "Number49", src: `{{ 2 ** 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.4142135623730951`, err: ``},
		{name: "Number50", src: `{{ (-2) ** 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `4`, err: ``},
		{name: "Number51", src: `{{ (-2) ** 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(8.659560562354934e-17+1.4142135623730951j)`, err: ``},
		{name: "Number52", src: `{{ -(2 ** 63) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-9223372036854775808`, err: ``},
		{name: "Number53", src: `{{ (2 ** 100) is number }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number54", src: `{{ (2 ** 100) is integer }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number55", src: `{{ (2 ** 100) is odd }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Number56", src: `{{ (2 ** 100) is divisibleby 3 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "Number57", src: `{{ (10 ** 5000)|string|length }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Exceeds the limit (4300 digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit`},
		{name: "Number58", src: `{{ 10 ** 4300 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Exceeds the limit (4300 digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit`},
		{name: "Number59", src: `{{ 10 ** 4299 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000`, err: ``},
		{name: "Number60", src: `{{ '%d' % 10 ** 5000 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Exceeds the limit (4300 digits) for integer string conversion; use sys.set_int_max_str_digits() to increase the limit`},
		{name: "Number61", src: `{{ 10 ** 5000 > 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number62", src: `{{ (10 ** 5000) // 10 ** 4999 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `10`, err: ``},
		{name: "Number63", src: `{{ [1, 2][2 ** 100] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "Number64", src: `{{ 'a' * 2 ** 100 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `cannot fit 'int' into an index-sized integer`},
		{name: "Number65", src: `{{ range(2 ** 100) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Python int too large to convert to C ssize_t`},
		{name: "Number66", src: `{{ (2 ** 100) in [2 ** 100] }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number67", src: `{{ [2 ** 100]|unique|list }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `[1267650600228229401496703205376]`, err: ``},
		{name: "Number68", src: `{{ (2**100)|int }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number69", src: `{{ (2**100) ~ '' }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1267650600228229401496703205376`, err: ``},
		{name: "Number70", src: `{{ -(-9223372036854775808) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9223372036854775808`, err: ``},
		{name: "Number71", src: `{{ 9223372036854775807 + 1 - 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9223372036854775807`, err: ``},
		{name: "Number72", src: `{{ (2 ** 100) // (2 ** 99) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `2`, err: ``},
		{name: "Number73", src: `{{ 1 / 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `division by zero`},
		{name: "Number74", src: `{{ 1.0 / 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float division by zero`},
		{name: "Number75", src: `{{ 1 / 0.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float division by zero`},
		{name: "Number76", src: `{{ 1 // 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `integer division or modulo by zero`},
		{name: "Number77", src: `{{ 1.0 // 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float floor division by zero`},
		{name: "Number78", src: `{{ 1 // 0.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float floor division by zero`},
		{name: "Number79", src: `{{ 1 % 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `integer modulo by zero`},
		{name: "Number80", src: `{{ 1.0 % 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float modulo`},
		{name: "Number81", src: `{{ 1 % 0.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float modulo`},
		{name: "Number82", src: `{{ 10.0 ** 400 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `(34, 'Numerical result out of range')`},
		{name: "Number83", src: `{{ 10 ** 400.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `(34, 'Numerical result out of range')`},
		{name: "Number84", src: `{{ 2.0 ** -2000 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.0`, err: ``},
		{name: "Number85", src: `{{ 1e308 * 10 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `inf`, err: ``},
		{name: "Number86", src: `{{ 0.0 ** -1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `0.0 cannot be raised to a negative power`},
		{name: "Number87", src: `{{ 0 ** -1.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `0.0 cannot be raised to a negative power`},
		{name: "Number88", src: `{{ (-8) ** -1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-0.125`, err: ``},
		{name: "Number89", src: `{{ 2 ** 3.0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `8.0`, err: ``},
		{name: "Number90", src: `{{ (-2.0) ** 3 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-8.0`, err: ``},
		{name: "Number91", src: `{{ (-2.0) ** 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(8.659560562354934e-17+1.4142135623730951j)`, err: ``},
		{name: "Number92", src: `{{ 'inf'|float - 'inf'|float }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `nan`, err: ``},
		{name: "Number93", src: `{{ ('inf'|float) // 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `nan`, err: ``},
		{name: "Number94", src: `{{ ('inf'|float) % 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `nan`, err: ``},
		{name: "Number95", src: `{{ 5 % ('inf'|float) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5.0`, err: ``},
		{name: "Number96", src: `{{ -5 % ('inf'|float) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `inf`, err: ``},
		{name: "Number97", src: `{{ ('nan'|float) // 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `nan`, err: ``},
		{name: "Number98", src: `{{ -7.5 // 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-4.0`, err: ``},
		{name: "Number99", src: `{{ 7.5 % -2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-0.5`, err: ``},
		{name: "Number100", src: `{{ -0.0 % 5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.0`, err: ``},
		{name: "Number101", src: `{{ (2 ** 100) % 2.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0`, err: ``},
		{name: "Number102", src: `{{ (2 ** 100) // 2.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5.070602400912918e+29`, err: ``},
		{name: "Number103", src: `{{ -(2**63) // -1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `9223372036854775808`, err: ``},
		{name: "Number104", src: `{{ ((-8) ** (1/3)) * 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(2.0000000000000004+3.4641016151377544j)`, err: ``},
		{name: "Number105", src: `{{ ((-8) ** (1/3)) + 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(2+1.7320508075688772j)`, err: ``},
		{name: "Number106", src: `{{ ((-8) ** (1/3)) / 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `complex division by zero`},
		{name: "Number107", src: `{{ ((-1) ** 0.5) ** 2 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(-1+1.2246467991473532e-16j)`, err: ``},
		{name: "Number108", src: `{{ ((-1) ** 0.5) == ((-1) ** 0.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number109", src: `{{ ((-1) ** 0.5) < 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'<' not supported between instances of 'complex' and 'int'`},
		{name: "Number110", src: `{{ ((-1) ** 0.5)|abs }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0`, err: ``},
		{name: "Number111", src: `{{ -((-1) ** 0.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(-6.123233995736766e-17-1j)`, err: ``},
		{name: "Number112", src: `{{ ((-4) ** 0.5) // 1 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `unsupported operand type(s) for //: 'complex' and 'int'`},
		{name: "Number113", src: `{{ {(-1) ** 0.5: 1} }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `{(6.123233995736766e-17+1j): 1}`, err: ``},
		{name: "Number114", src: `{{ (-1) ** 0.5 is number }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-1`, err: ``},
		{name: "Number115", src: `{{ ((-1)**0.5)|string }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(6.123233995736766e-17+1j)`, err: ``},
		{name: "Number116", src: `{{ '%s' % ((-1) ** 0.5,) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(6.123233995736766e-17+1j)`, err: ``},
		{name: "Number117", src: `{{ ((-1) ** 0.5) ** 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(0.7071067811865476+0.7071067811865475j)`, err: ``},
		{name: "Number118", src: `{{ (((-1) ** 0.5) * 0) == 0 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number119", src: `{{ 0 ** ((-1) ** 0.5) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `0.0 to a negative or complex power`},
		{name: "Number120", src: `{{ ((-2) ** 0.5)|round }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `type complex doesn't define __round__ method`},
		{name: "Number121", src: `{{ ((-2) ** 0.5)|int }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "Number122", src: `{{ ((-2)**0.5)|tojson }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `Object of type complex is not JSON serializable`},
		{name: "Number123", src: `{{ not ((-1) ** 0.5 * 0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "Number124", src: `{{ (-1e308) ** 0.5 }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(6.123233995736766e+137+1e+154j)`, err: ``},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mustEnv(t, WithAutoescape(AutoescapeNever{}), WithLoader(loader.Dict(c.tpls)), WithLexerOptions(lexer.Options{TrimBlocks: c.trim}))
			vars, err := varsutil.JSONVars([]byte(c.vars))
			if err != nil {
				t.Fatal(err)
			}
			var tpl *Template
			if c.get != "" {
				tpl, err = e.GetTemplate(c.get)
			} else {
				tpl, err = e.FromString(c.src)
			}
			out := ""
			if err == nil {
				out, err = tpl.RenderContext(context.Background(), vars)
			}
			switch {
			case c.err == "*" && err == nil:
				t.Errorf("got %q; want an error", out)
			case c.err != "" && c.err != "*" && (err == nil || !strings.Contains(err.Error(), c.err)):
				t.Errorf("got %q, %v; want error containing %q", out, err, c.err)
			case c.err == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.err == "" && out != c.want:
				t.Errorf("got %q, want %q", out, c.want)
			}
		})
	}
}
