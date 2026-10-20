package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/lexer"
	"github.com/jryberg/gojinja/pkg/loader"
	"github.com/jryberg/gojinja/pkg/varsutil"
)

// TestNumberMethods holds the attributes and methods of Python's int, bool, float and complex checked against Jinja2. Expected values come from the Jinja2 pinned in
// tools/parity/requirements.txt; err is the expected error's message, or
// "*" when only an error is expected.
func TestNumberMethods(t *testing.T) {
	cases := []struct {
		name, src, get string
		tpls           map[string]string
		vars           string
		trim           bool
		want, err      string
	}{
		{name: "NumberMethod1", src: `{{ (255).bit_length() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `8`, err: ``},
		{name: "NumberMethod2", src: `{{ (-255).bit_length() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `8`, err: ``},
		{name: "NumberMethod3", src: `{{ (0).bit_length() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "NumberMethod4", src: `{{ (2**100).bit_length() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `101`, err: ``},
		{name: "NumberMethod5", src: `{{ (255).bit_count() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `8`, err: ``},
		{name: "NumberMethod6", src: `{{ (-7).bit_count() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "NumberMethod7", src: `{{ (5).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(5, 1)`, err: ``},
		{name: "NumberMethod8", src: `{{ (5).conjugate() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "NumberMethod9", src: `{{ (5).real }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "NumberMethod10", src: `{{ (5).imag }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "NumberMethod11", src: `{{ (5).numerator }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5`, err: ``},
		{name: "NumberMethod12", src: `{{ (5).denominator }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "NumberMethod13", src: `{{ (5).is_integer() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "NumberMethod14", src: `{{ true.real }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "NumberMethod15", src: `{{ true.numerator }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "NumberMethod16", src: `{{ true.bit_length() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "NumberMethod17", src: `{{ true.conjugate() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1`, err: ``},
		{name: "NumberMethod18", src: `{{ true.as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(1, 1)`, err: ``},
		{name: "NumberMethod19", src: `{{ false.imag }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "NumberMethod20", src: `{{ (1).bit_length(2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int.bit_length() takes no arguments (1 given)`},
		{name: "NumberMethod21", src: `{{ (1).bit_length(x=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int.bit_length() takes no keyword arguments`},
		{name: "NumberMethod22", src: `{{ (255).to_bytes(2, 'big') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\x00\xff'`, err: ``},
		{name: "NumberMethod23", src: `{{ (255).to_bytes(2, 'little') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\xff\x00'`, err: ``},
		{name: "NumberMethod24", src: `{{ (255).to_bytes() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\xff'`, err: ``},
		{name: "NumberMethod25", src: `{{ (255).to_bytes(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\xff'`, err: ``},
		{name: "NumberMethod26", src: `{{ (256).to_bytes(1, 'big') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int too big to convert`},
		{name: "NumberMethod27", src: `{{ (-1).to_bytes(1, 'big') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `can't convert negative int to unsigned`},
		{name: "NumberMethod28", src: `{{ (-1).to_bytes(1, 'big', signed=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\xff'`, err: ``},
		{name: "NumberMethod29", src: `{{ (-129).to_bytes(1, 'big', signed=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int too big to convert`},
		{name: "NumberMethod30", src: `{{ (-128).to_bytes(1, 'big', signed=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\x80'`, err: ``},
		{name: "NumberMethod31", src: `{{ (1).to_bytes(1, 'middle') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `byteorder must be either 'little' or 'big'`},
		{name: "NumberMethod32", src: `{{ (1).to_bytes(-1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `length argument must be non-negative`},
		{name: "NumberMethod33", src: `{{ (1).to_bytes(1, 'big', true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `to_bytes() takes at most 2 positional arguments (3 given)`},
		{name: "NumberMethod34", src: `{{ (1).to_bytes(length=2, byteorder='little') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\x01\x00'`, err: ``},
		{name: "NumberMethod35", src: `{{ (1).to_bytes(1, byteorder=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `to_bytes() argument 'byteorder' must be str, not int`},
		{name: "NumberMethod36", src: `{{ (1).to_bytes('a') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'str' object cannot be interpreted as an integer`},
		{name: "NumberMethod37", src: `{{ (1).to_bytes(2, length=2) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `argument for to_bytes() given by name ('length') and position (1)`},
		{name: "NumberMethod38", src: `{{ (1).to_bytes(bad=1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `'bad' is an invalid keyword argument for to_bytes()`},
		{name: "NumberMethod39", src: `{{ (0).to_bytes(0) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b''`, err: ``},
		{name: "NumberMethod40", src: `{{ (2**100).to_bytes(13, 'big') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `b'\x10\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00'`, err: ``},
		{name: "NumberMethod41", src: `{{ (0).from_bytes('ab'.encode(), 'big') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `24930`, err: ``},
		{name: "NumberMethod42", src: `{{ (0).from_bytes('\xff'.encode('latin-1'), 'big', signed=true) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-1`, err: ``},
		{name: "NumberMethod43", src: `{{ (0).from_bytes([1, 2]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `258`, err: ``},
		{name: "NumberMethod44", src: `{{ (0).from_bytes('ab') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `cannot convert 'str' object to bytes`},
		{name: "NumberMethod45", src: `{{ (0).from_bytes([1, 256]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `bytes must be in range(0, 256)`},
		{name: "NumberMethod46", src: `{{ (0).from_bytes([1, 2], 'little') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `513`, err: ``},
		{name: "NumberMethod47", src: `{{ (0).from_bytes() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `from_bytes() missing required argument 'bytes' (pos 1)`},
		{name: "NumberMethod48", src: `{{ (0).from_bytes(range(3)) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `258`, err: ``},
		{name: "NumberMethod49", src: `{{ (0).from_bytes([]) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0`, err: ``},
		{name: "NumberMethod50", src: `{{ (1.5).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(3, 2)`, err: ``},
		{name: "NumberMethod51", src: `{{ (0.1).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(3602879701896397, 36028797018963968)`, err: ``},
		{name: "NumberMethod52", src: `{{ (1e300).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(1000000000000000052504760255204420248704468581108159154915854115511802457988908195786371375080447864043704443832883878176942523235360430575644792184786706982848387200926575803737830233794788090059368953234970799945081119038967640880074652742780142494579258788820056842838115669472196386865459400540160, 1)`, err: ``},
		{name: "NumberMethod53", src: `{{ (1.5).conjugate() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.5`, err: ``},
		{name: "NumberMethod54", src: `{{ (1.5).real }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.5`, err: ``},
		{name: "NumberMethod55", src: `{{ (1.5).imag }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.0`, err: ``},
		{name: "NumberMethod56", src: `{{ (1.5).is_integer() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `False`, err: ``},
		{name: "NumberMethod57", src: `{{ (2.0).is_integer() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
		{name: "NumberMethod58", src: `{{ (1.5).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x1.8000000000000p+0`, err: ``},
		{name: "NumberMethod59", src: `{{ (0.0).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x0.0p+0`, err: ``},
		{name: "NumberMethod60", src: `{{ (-0.0).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-0x0.0p+0`, err: ``},
		{name: "NumberMethod61", src: `{{ (0.1).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x1.999999999999ap-4`, err: ``},
		{name: "NumberMethod62", src: `{{ (5e-324).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x0.0000000000001p-1022`, err: ``},
		{name: "NumberMethod63", src: `{{ (1e308).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x1.1ccf385ebc8a0p+1023`, err: ``},
		{name: "NumberMethod64", src: `{{ (-2.5).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-0x1.4000000000000p+1`, err: ``},
		{name: "NumberMethod65", src: `{{ (1.0).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x1.0000000000000p+0`, err: ``},
		{name: "NumberMethod66", src: `{{ (2.2250738585072014e-308).hex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0x1.0000000000000p-1022`, err: ``},
		{name: "NumberMethod67", src: `{{ (1.5).hex(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float.hex() takes no arguments (1 given)`},
		{name: "NumberMethod68", src: `{{ (1.0).fromhex('0x1.8p1') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3.0`, err: ``},
		{name: "NumberMethod69", src: `{{ (1.0).fromhex('  -0x1.8P+1  ') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-3.0`, err: ``},
		{name: "NumberMethod70", src: `{{ (1.0).fromhex('inf') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `inf`, err: ``},
		{name: "NumberMethod71", src: `{{ (1.0).fromhex('-Infinity') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `-inf`, err: ``},
		{name: "NumberMethod72", src: `{{ (1.0).fromhex('0x1p-1080') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.0`, err: ``},
		{name: "NumberMethod73", src: `{{ (1.0).fromhex('0x1p1024') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `hexadecimal value too large to represent as a float`},
		{name: "NumberMethod74", src: `{{ (1.0).fromhex('zz') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `invalid hexadecimal floating-point string`},
		{name: "NumberMethod75", src: `{{ (1.0).fromhex('1.8') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.5`, err: ``},
		{name: "NumberMethod76", src: `{{ (1.0).fromhex('0x.8') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.5`, err: ``},
		{name: "NumberMethod77", src: `{{ (1.0).fromhex('0x1.fffffffffffff8p1023') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `hexadecimal value too large to represent as a float`},
		{name: "NumberMethod78", src: `{{ (1.0).fromhex('0xa') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `10.0`, err: ``},
		{name: "NumberMethod79", src: `{{ (1.0).fromhex('0x1.000000000000080000001p0') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0000000000000002`, err: ``},
		{name: "NumberMethod80", src: `{{ (1.0).fromhex('0x1.00000000000008p0') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0`, err: ``},
		{name: "NumberMethod81", src: `{{ (1.0).fromhex('0x1.00000000000018p0') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0000000000000004`, err: ``},
		{name: "NumberMethod82", src: `{{ (1.0).fromhex('0x0.0000000000001p-1022') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5e-324`, err: ``},
		{name: "NumberMethod83", src: `{{ (1.0).fromhex('0x0.00000000000008p-1022') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `0.0`, err: ``},
		{name: "NumberMethod84", src: `{{ (1.0).fromhex('0x0.0000000000000801p-1022') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `5e-324`, err: ``},
		{name: "NumberMethod85", src: `{{ (1.0).fromhex('') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `invalid hexadecimal floating-point string`},
		{name: "NumberMethod86", src: `{{ (1.0).fromhex('0x') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `invalid hexadecimal floating-point string`},
		{name: "NumberMethod87", src: `{{ (1.0).fromhex('0x1p') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `invalid hexadecimal floating-point string`},
		{name: "NumberMethod88", src: `{{ (1.0).fromhex('0x1p+-1') }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `invalid hexadecimal floating-point string`},
		{name: "NumberMethod89", src: `{{ (1.0).fromhex(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `bad argument type for built-in operation`},
		{name: "NumberMethod90", src: `{{ (1.0).fromhex() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `float.fromhex() takes exactly one argument (0 given)`},
		{name: "NumberMethod91", src: `{{ ((-1) ** 0.5).real }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `6.123233995736766e-17`, err: ``},
		{name: "NumberMethod92", src: `{{ ((-1) ** 0.5).imag }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `1.0`, err: ``},
		{name: "NumberMethod93", src: `{{ ((-1) ** 0.5).conjugate() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(6.123233995736766e-17-1j)`, err: ``},
		{name: "NumberMethod94", src: `{{ (5).bogus }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "NumberMethod95", src: `{{ (1.5).bogus }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "NumberMethod96", src: `{{ '{0.real}{0.imag}'.format(3) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `30`, err: ``},
		{name: "NumberMethod97", src: `{{ (2**100).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(1267650600228229401496703205376, 1)`, err: ``},
		{name: "NumberMethod98", src: `{{ (3).is_integer(1) }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: `int.is_integer() takes no arguments (1 given)`},
		{name: "NumberMethod99", src: `{{ (3).real.real }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `3`, err: ``},
		{name: "NumberMethod100", src: `{{ (-0.0).as_integer_ratio() }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `(0, 1)`, err: ``},
		{name: "NumberMethod101", src: `{{ (5).__class__ }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: ``, err: ``},
		{name: "NumberMethod102", src: `{{ (5).bit_length is callable }}`, get: "", tpls: nil, vars: `{}`, trim: false, want: `True`, err: ``},
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
