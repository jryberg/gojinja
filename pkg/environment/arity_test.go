package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/jryberg/gojinja/pkg/runtime"
)

// Expected values come from rendering the same templates with the Jinja2
// pinned in tools/parity/requirements.txt. x is the list [1, 2].

// Every builtin filter and test rejects a call Python would fail with a
// TypeError, with CPython's message.
func TestBuiltinArity(t *testing.T) {
	cases := []struct{ src, msg string }{
		{`{{ x|abs(1, 2, 3, 4, 5, 6, 7, 8) }}`, "abs() takes exactly one argument (9 given)"},
		{`{{ x|abs(zzz=1) }}`, "abs() takes no keyword arguments"},
		{`{{ x|attr(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_attr() takes 3 positional arguments but 10 were given"},
		{`{{ x|attr(zzz=1) }}`, "do_attr() got an unexpected keyword argument 'zzz'"},
		{`{{ x|attr }}`, "do_attr() missing 1 required positional argument: 'name'"},
		{`{{ x|attr(1, name=1) }}`, "do_attr() got multiple values for argument 'name'"},
		{`{{ x|batch(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_batch() takes from 2 to 3 positional arguments but 9 were given"},
		{`{{ x|batch(zzz=1) }}`, "do_batch() got an unexpected keyword argument 'zzz'"},
		{`{{ x|batch }}`, "do_batch() missing 1 required positional argument: 'linecount'"},
		{`{{ x|capitalize(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_capitalize() takes 1 positional argument but 9 were given"},
		{`{{ x|capitalize(zzz=1) }}`, "do_capitalize() got an unexpected keyword argument 'zzz'"},
		{`{{ x|center(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_center() takes from 1 to 2 positional arguments but 9 were given"},
		{`{{ x|center(zzz=1) }}`, "do_center() got an unexpected keyword argument 'zzz'"},
		{`{{ x|count(1, 2, 3, 4, 5, 6, 7, 8) }}`, "len() takes exactly one argument (9 given)"},
		{`{{ x|count(zzz=1) }}`, "len() takes no keyword arguments"},
		{`{{ x|d(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_default() takes from 1 to 3 positional arguments but 9 were given"},
		{`{{ x|d(zzz=1) }}`, "do_default() got an unexpected keyword argument 'zzz'"},
		{`{{ x|default(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_default() takes from 1 to 3 positional arguments but 9 were given"},
		{`{{ x|default(zzz=1) }}`, "do_default() got an unexpected keyword argument 'zzz'"},
		{`{{ x|dictsort(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_dictsort() takes from 1 to 4 positional arguments but 9 were given"},
		{`{{ x|dictsort(zzz=1) }}`, "do_dictsort() got an unexpected keyword argument 'zzz'"},
		{`{{ x|e(1, 2, 3, 4, 5, 6, 7, 8) }}`, "escape() takes 1 positional argument but 9 were given"},
		{`{{ x|e(zzz=1) }}`, "escape() got an unexpected keyword argument 'zzz'"},
		{`{{ x|escape(1, 2, 3, 4, 5, 6, 7, 8) }}`, "escape() takes 1 positional argument but 9 were given"},
		{`{{ x|escape(zzz=1) }}`, "escape() got an unexpected keyword argument 'zzz'"},
		{`{{ x|filesizeformat(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_filesizeformat() takes from 1 to 2 positional arguments but 9 were given"},
		{`{{ x|filesizeformat(zzz=1) }}`, "do_filesizeformat() got an unexpected keyword argument 'zzz'"},
		{`{{ x|first(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_first() takes 2 positional arguments but 10 were given"},
		{`{{ x|first(zzz=1) }}`, "sync_do_first() got an unexpected keyword argument 'zzz'"},
		{`{{ x|float(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_float() takes from 1 to 2 positional arguments but 9 were given"},
		{`{{ x|float(zzz=1) }}`, "do_float() got an unexpected keyword argument 'zzz'"},
		{`{{ x|forceescape(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_forceescape() takes 1 positional argument but 9 were given"},
		{`{{ x|forceescape(zzz=1) }}`, "do_forceescape() got an unexpected keyword argument 'zzz'"},
		{`{{ x|groupby(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_groupby() takes from 3 to 5 positional arguments but 10 were given"},
		{`{{ x|groupby(zzz=1) }}`, "sync_do_groupby() got an unexpected keyword argument 'zzz'"},
		{`{{ x|groupby }}`, "sync_do_groupby() missing 1 required positional argument: 'attribute'"},
		{`{{ x|indent(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_indent() takes from 1 to 4 positional arguments but 9 were given"},
		{`{{ x|indent(zzz=1) }}`, "do_indent() got an unexpected keyword argument 'zzz'"},
		{`{{ x|int(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_int() takes from 1 to 3 positional arguments but 9 were given"},
		{`{{ x|int(zzz=1) }}`, "do_int() got an unexpected keyword argument 'zzz'"},
		{`{{ x|items(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_items() takes 1 positional argument but 9 were given"},
		{`{{ x|items(zzz=1) }}`, "do_items() got an unexpected keyword argument 'zzz'"},
		{`{{ x|join(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_join() takes from 2 to 4 positional arguments but 10 were given"},
		{`{{ x|join(zzz=1) }}`, "sync_do_join() got an unexpected keyword argument 'zzz'"},
		{`{{ x|last(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_last() takes 2 positional arguments but 10 were given"},
		{`{{ x|last(zzz=1) }}`, "do_last() got an unexpected keyword argument 'zzz'"},
		{`{{ x|length(1, 2, 3, 4, 5, 6, 7, 8) }}`, "len() takes exactly one argument (9 given)"},
		{`{{ x|length(zzz=1) }}`, "len() takes no keyword arguments"},
		{`{{ x|list(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_list() takes 1 positional argument but 9 were given"},
		{`{{ x|list(zzz=1) }}`, "sync_do_list() got an unexpected keyword argument 'zzz'"},
		{`{{ x|lower(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_lower() takes 1 positional argument but 9 were given"},
		{`{{ x|lower(zzz=1) }}`, "do_lower() got an unexpected keyword argument 'zzz'"},
		{`{{ x|max(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_max() takes from 2 to 4 positional arguments but 10 were given"},
		{`{{ x|max(zzz=1) }}`, "do_max() got an unexpected keyword argument 'zzz'"},
		{`{{ x|min(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_min() takes from 2 to 4 positional arguments but 10 were given"},
		{`{{ x|min(zzz=1) }}`, "do_min() got an unexpected keyword argument 'zzz'"},
		{`{{ x|pprint(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_pprint() takes 1 positional argument but 9 were given"},
		{`{{ x|pprint(zzz=1) }}`, "do_pprint() got an unexpected keyword argument 'zzz'"},
		{`{{ x|random(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_random() takes 2 positional arguments but 10 were given"},
		{`{{ x|random(zzz=1) }}`, "do_random() got an unexpected keyword argument 'zzz'"},
		{`{{ x|replace(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_replace() takes from 4 to 5 positional arguments but 10 were given"},
		{`{{ x|replace(zzz=1) }}`, "do_replace() got an unexpected keyword argument 'zzz'"},
		{`{{ x|replace }}`, "do_replace() missing 2 required positional arguments: 'old' and 'new'"},
		{`{{ x|replace(1, count=1) }}`, "do_replace() missing 1 required positional argument: 'new'"},
		{`{{ x|reverse(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_reverse() takes 1 positional argument but 9 were given"},
		{`{{ x|reverse(zzz=1) }}`, "do_reverse() got an unexpected keyword argument 'zzz'"},
		{`{{ x|round(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_round() takes from 1 to 3 positional arguments but 9 were given"},
		{`{{ x|round(zzz=1) }}`, "do_round() got an unexpected keyword argument 'zzz'"},
		{`{{ x|safe(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_mark_safe() takes 1 positional argument but 9 were given"},
		{`{{ x|safe(zzz=1) }}`, "do_mark_safe() got an unexpected keyword argument 'zzz'"},
		{`{{ x|slice(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_slice() takes from 2 to 3 positional arguments but 9 were given"},
		{`{{ x|slice(zzz=1) }}`, "sync_do_slice() got an unexpected keyword argument 'zzz'"},
		{`{{ x|slice }}`, "sync_do_slice() missing 1 required positional argument: 'slices'"},
		{`{{ x|sort(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_sort() takes from 2 to 5 positional arguments but 10 were given"},
		{`{{ x|sort(zzz=1) }}`, "do_sort() got an unexpected keyword argument 'zzz'"},
		{`{{ x|string(1, 2, 3, 4, 5, 6, 7, 8) }}`, "soft_str() takes 1 positional argument but 9 were given"},
		{`{{ x|string(zzz=1) }}`, "soft_str() got an unexpected keyword argument 'zzz'"},
		{`{{ x|striptags(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_striptags() takes 1 positional argument but 9 were given"},
		{`{{ x|striptags(zzz=1) }}`, "do_striptags() got an unexpected keyword argument 'zzz'"},
		{`{{ x|sum(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_sum() takes from 2 to 4 positional arguments but 10 were given"},
		{`{{ x|sum(zzz=1) }}`, "sync_do_sum() got an unexpected keyword argument 'zzz'"},
		{`{{ x|title(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_title() takes 1 positional argument but 9 were given"},
		{`{{ x|title(zzz=1) }}`, "do_title() got an unexpected keyword argument 'zzz'"},
		{`{{ x|tojson(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_tojson() takes from 2 to 3 positional arguments but 10 were given"},
		{`{{ x|tojson(zzz=1) }}`, "do_tojson() got an unexpected keyword argument 'zzz'"},
		{`{{ x|tojson(1, indent=1) }}`, "do_tojson() got multiple values for argument 'indent'"},
		{`{{ x|trim(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_trim() takes from 1 to 2 positional arguments but 9 were given"},
		{`{{ x|trim(zzz=1) }}`, "do_trim() got an unexpected keyword argument 'zzz'"},
		{`{{ x|truncate(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_truncate() takes from 2 to 6 positional arguments but 10 were given"},
		{`{{ x|truncate(zzz=1) }}`, "do_truncate() got an unexpected keyword argument 'zzz'"},
		{`{{ x|unique(1, 2, 3, 4, 5, 6, 7, 8) }}`, "sync_do_unique() takes from 2 to 4 positional arguments but 10 were given"},
		{`{{ x|unique(zzz=1) }}`, "sync_do_unique() got an unexpected keyword argument 'zzz'"},
		{`{{ x|upper(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_upper() takes 1 positional argument but 9 were given"},
		{`{{ x|upper(zzz=1) }}`, "do_upper() got an unexpected keyword argument 'zzz'"},
		{`{{ x|urlencode(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_urlencode() takes 1 positional argument but 9 were given"},
		{`{{ x|urlencode(zzz=1) }}`, "do_urlencode() got an unexpected keyword argument 'zzz'"},
		{`{{ x|urlize(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_urlize() takes from 2 to 7 positional arguments but 10 were given"},
		{`{{ x|urlize(zzz=1) }}`, "do_urlize() got an unexpected keyword argument 'zzz'"},
		{`{{ x|wordcount(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_wordcount() takes 1 positional argument but 9 were given"},
		{`{{ x|wordcount(zzz=1) }}`, "do_wordcount() got an unexpected keyword argument 'zzz'"},
		{`{{ x|wordwrap(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_wordwrap() takes from 2 to 6 positional arguments but 10 were given"},
		{`{{ x|wordwrap(zzz=1) }}`, "do_wordwrap() got an unexpected keyword argument 'zzz'"},
		{`{{ x|xmlattr(1, 2, 3, 4, 5, 6, 7, 8) }}`, "do_xmlattr() takes from 2 to 3 positional arguments but 10 were given"},
		{`{{ x|xmlattr(zzz=1) }}`, "do_xmlattr() got an unexpected keyword argument 'zzz'"},
		{`{{ x|xmlattr(1, autospace=1) }}`, "do_xmlattr() got multiple values for argument 'autospace'"},
		{`{{ x is boolean(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_boolean() takes 1 positional argument but 9 were given"},
		{`{{ x is boolean(zzz=1) }}`, "test_boolean() got an unexpected keyword argument 'zzz'"},
		{`{{ x is callable(1, 2, 3, 4, 5, 6, 7, 8) }}`, "callable() takes exactly one argument (9 given)"},
		{`{{ x is callable(zzz=1) }}`, "callable() takes no keyword arguments"},
		{`{{ x is defined(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_defined() takes 1 positional argument but 9 were given"},
		{`{{ x is defined(zzz=1) }}`, "test_defined() got an unexpected keyword argument 'zzz'"},
		{`{{ x is divisibleby(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_divisibleby() takes 2 positional arguments but 9 were given"},
		{`{{ x is divisibleby(zzz=1) }}`, "test_divisibleby() got an unexpected keyword argument 'zzz'"},
		{`{{ x is divisibleby }}`, "test_divisibleby() missing 1 required positional argument: 'num'"},
		{`{{ x is eq(1, 2, 3, 4, 5, 6, 7, 8) }}`, "eq expected 2 arguments, got 9"},
		{`{{ x is eq(zzz=1) }}`, "_operator.eq() takes no keyword arguments"},
		{`{{ x is eq }}`, "eq expected 2 arguments, got 1"},
		{`{{ x is equalto(1, 2, 3, 4, 5, 6, 7, 8) }}`, "eq expected 2 arguments, got 9"},
		{`{{ x is equalto(zzz=1) }}`, "_operator.eq() takes no keyword arguments"},
		{`{{ x is equalto }}`, "eq expected 2 arguments, got 1"},
		{`{{ x is escaped(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_escaped() takes 1 positional argument but 9 were given"},
		{`{{ x is escaped(zzz=1) }}`, "test_escaped() got an unexpected keyword argument 'zzz'"},
		{`{{ x is even(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_even() takes 1 positional argument but 9 were given"},
		{`{{ x is even(zzz=1) }}`, "test_even() got an unexpected keyword argument 'zzz'"},
		{`{{ x is false(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_false() takes 1 positional argument but 9 were given"},
		{`{{ x is false(zzz=1) }}`, "test_false() got an unexpected keyword argument 'zzz'"},
		{`{{ x is filter(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_filter() takes 2 positional arguments but 10 were given"},
		{`{{ x is filter(zzz=1) }}`, "test_filter() got an unexpected keyword argument 'zzz'"},
		{`{{ x is float(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_float() takes 1 positional argument but 9 were given"},
		{`{{ x is float(zzz=1) }}`, "test_float() got an unexpected keyword argument 'zzz'"},
		{`{{ x is ge(1, 2, 3, 4, 5, 6, 7, 8) }}`, "ge expected 2 arguments, got 9"},
		{`{{ x is ge(zzz=1) }}`, "_operator.ge() takes no keyword arguments"},
		{`{{ x is ge }}`, "ge expected 2 arguments, got 1"},
		{`{{ x is greaterthan(1, 2, 3, 4, 5, 6, 7, 8) }}`, "gt expected 2 arguments, got 9"},
		{`{{ x is greaterthan(zzz=1) }}`, "_operator.gt() takes no keyword arguments"},
		{`{{ x is greaterthan }}`, "gt expected 2 arguments, got 1"},
		{`{{ x is gt(1, 2, 3, 4, 5, 6, 7, 8) }}`, "gt expected 2 arguments, got 9"},
		{`{{ x is gt(zzz=1) }}`, "_operator.gt() takes no keyword arguments"},
		{`{{ x is gt }}`, "gt expected 2 arguments, got 1"},
		{`{{ x is in(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_in() takes 2 positional arguments but 9 were given"},
		{`{{ x is in(zzz=1) }}`, "test_in() got an unexpected keyword argument 'zzz'"},
		{`{{ x is in }}`, "test_in() missing 1 required positional argument: 'seq'"},
		{`{{ x is integer(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_integer() takes 1 positional argument but 9 were given"},
		{`{{ x is integer(zzz=1) }}`, "test_integer() got an unexpected keyword argument 'zzz'"},
		{`{{ x is iterable(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_iterable() takes 1 positional argument but 9 were given"},
		{`{{ x is iterable(zzz=1) }}`, "test_iterable() got an unexpected keyword argument 'zzz'"},
		{`{{ x is le(1, 2, 3, 4, 5, 6, 7, 8) }}`, "le expected 2 arguments, got 9"},
		{`{{ x is le(zzz=1) }}`, "_operator.le() takes no keyword arguments"},
		{`{{ x is le }}`, "le expected 2 arguments, got 1"},
		{`{{ x is lessthan(1, 2, 3, 4, 5, 6, 7, 8) }}`, "lt expected 2 arguments, got 9"},
		{`{{ x is lessthan(zzz=1) }}`, "_operator.lt() takes no keyword arguments"},
		{`{{ x is lessthan }}`, "lt expected 2 arguments, got 1"},
		{`{{ x is lower(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_lower() takes 1 positional argument but 9 were given"},
		{`{{ x is lower(zzz=1) }}`, "test_lower() got an unexpected keyword argument 'zzz'"},
		{`{{ x is lt(1, 2, 3, 4, 5, 6, 7, 8) }}`, "lt expected 2 arguments, got 9"},
		{`{{ x is lt(zzz=1) }}`, "_operator.lt() takes no keyword arguments"},
		{`{{ x is lt }}`, "lt expected 2 arguments, got 1"},
		{`{{ x is mapping(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_mapping() takes 1 positional argument but 9 were given"},
		{`{{ x is mapping(zzz=1) }}`, "test_mapping() got an unexpected keyword argument 'zzz'"},
		{`{{ x is ne(1, 2, 3, 4, 5, 6, 7, 8) }}`, "ne expected 2 arguments, got 9"},
		{`{{ x is ne(zzz=1) }}`, "_operator.ne() takes no keyword arguments"},
		{`{{ x is ne }}`, "ne expected 2 arguments, got 1"},
		{`{{ x is none(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_none() takes 1 positional argument but 9 were given"},
		{`{{ x is none(zzz=1) }}`, "test_none() got an unexpected keyword argument 'zzz'"},
		{`{{ x is number(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_number() takes 1 positional argument but 9 were given"},
		{`{{ x is number(zzz=1) }}`, "test_number() got an unexpected keyword argument 'zzz'"},
		{`{{ x is odd(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_odd() takes 1 positional argument but 9 were given"},
		{`{{ x is odd(zzz=1) }}`, "test_odd() got an unexpected keyword argument 'zzz'"},
		{`{{ x is sameas(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_sameas() takes 2 positional arguments but 9 were given"},
		{`{{ x is sameas(zzz=1) }}`, "test_sameas() got an unexpected keyword argument 'zzz'"},
		{`{{ x is sameas }}`, "test_sameas() missing 1 required positional argument: 'other'"},
		{`{{ x is sequence(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_sequence() takes 1 positional argument but 9 were given"},
		{`{{ x is sequence(zzz=1) }}`, "test_sequence() got an unexpected keyword argument 'zzz'"},
		{`{{ x is string(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_string() takes 1 positional argument but 9 were given"},
		{`{{ x is string(zzz=1) }}`, "test_string() got an unexpected keyword argument 'zzz'"},
		{`{{ x is test(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_test() takes 2 positional arguments but 10 were given"},
		{`{{ x is test(zzz=1) }}`, "test_test() got an unexpected keyword argument 'zzz'"},
		{`{{ x is true(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_true() takes 1 positional argument but 9 were given"},
		{`{{ x is true(zzz=1) }}`, "test_true() got an unexpected keyword argument 'zzz'"},
		{`{{ x is undefined(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_undefined() takes 1 positional argument but 9 were given"},
		{`{{ x is undefined(zzz=1) }}`, "test_undefined() got an unexpected keyword argument 'zzz'"},
		{`{{ x is upper(1, 2, 3, 4, 5, 6, 7, 8) }}`, "test_upper() takes 1 positional argument but 9 were given"},
		{`{{ x is upper(zzz=1) }}`, "test_upper() got an unexpected keyword argument 'zzz'"},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		_, err = tpl.RenderContext(context.Background(), map[string]any{"x": []any{1, 2}})
		if err == nil || !strings.HasSuffix(err.Error(), c.msg) {
			t.Errorf("%s: got error %v, want %q", c.src, err, c.msg)
		}
	}
}

// Keyword arguments bind like Python's, and arguments of the wrong type
// raise where Python raises.
func TestBuiltinArguments(t *testing.T) {
	cases := []struct {
		src     string
		want    string
		wantErr bool
	}{
		{`{{ 'x'|trim(3) }}`, "", true},
		{`{{ 'x'|trim(none) }}`, "x", false},
		{`{{ 'a'|center('x') }}`, "", true},
		{`{{ 'a'|center(3.5) }}`, "", true},
		{`{{ 'a b'|indent('>') }}`, "a b", false},
		{`{{ 'a b'|indent(2.5) }}`, "", true},
		{`{{ 'a b c'|truncate('x') }}`, "", true},
		{`{{ 'a b c'|truncate(3, leeway='x') }}`, "", true},
		{`{{ [1, 2]|batch('x')|list }}`, "[[1, 2]]", false},
		{`{{ [1, 2]|batch(0)|list }}`, "[[], [1, 2]]", false},
		{`{{ [1, 2]|slice('x')|list }}`, "", true},
		{`{{ [1, 2]|slice(0)|list }}`, "", true},
		{`{{ 1.5|round('x') }}`, "", true},
		{`{{ 1.5|round(1.5) }}`, "", true},
		{`{{ 'ff'|int(0, 'x') }}`, "0", false},
		{`{{ 'ff'|int(0, 16) }}`, "255", false},
		{`{{ 'a'|replace('a', 'b', 'x') }}`, "", true},
		{`{{ 'a'|replace(1, 2) }}`, "a", false},
		{`{{ 'a1'|replace(1, 2) }}`, "a2", false},
		{`{{ [1, 2]|join(1) }}`, "112", false},
		{`{{ {'a': 1}|dictsort(by='x') }}`, "", true},
		{`{{ {'a': 1}|dictsort(false, 'value') }}`, "[('a', 1)]", false},
		{`{{ [1, 2]|sum(start='x') }}`, "", true},
		{`{{ [1, 2]|sum(start=1.5) }}`, "4.5", false},
		{`{{ 1|first }}`, "", true},
		{`{{ 1|last }}`, "", true},
		{`{{ 1|random }}`, "", true},
		{`{{ 1|list }}`, "", true},
		{`{{ 1|length }}`, "", true},
		{`{{ 1|reverse }}`, "", true},
		{`{{ 1|sort }}`, "", true},
		{`{{ 1|unique|list }}`, "", true},
		{`{{ 1|min }}`, "", true},
		{`{{ 1|join }}`, "", true},
		{`{{ 1|batch(2)|list }}`, "", true},
		{`{{ [1]|tojson(indent='x') }}`, "[\nx1\n]", false},
		{`{{ [1]|xmlattr }}`, "", true},
		{`{{ 1 is divisibleby 'x' }}`, "", true},
		{`{{ 1 is divisibleby 0 }}`, "", true},
		{`{{ 1 is in 'abc' }}`, "", true},
		{`{{ 1 is in 5 }}`, "", true},
		{`{{ 'x' is in 5 }}`, "", true},
		{`{{ 'x' in 5 }}`, "", true},
		{`{{ [1, 'a']|sort }}`, "", true},
		{`{{ [1, 'a']|max }}`, "", true},
		{`{{ [1, none]|min }}`, "", true},
		{`{{ [{'a': 1}, {'a': 2}]|map(attribute='a')|sum }}`, "3", false},
		{`{{ [1, 2]|map('int', 'x')|list }}`, "[1, 2]", false},
		{`{{ 'abc'|filesizeformat }}`, "", true},
		{`{{ 1|abs }}`, "1", false},
		{`{{ 'x'|abs }}`, "", true},
		{`{{ 'x'|float('y', 2) }}`, "", true},
		{`{{ 'x'|default }}`, "x", false},
		{`{{ 'a'|wordcount }}`, "1", false},
		{`{{ 'a'|urlize(1.5) }}`, "a", false},
		{`{{ 'a'|striptags(1) }}`, "", true},
		{`{{ {'a': 1}|items|list }}`, "[('a', 1)]", false},
		{`{{ [1]|items|list }}`, "", true},
		{`{{ 1|string }}`, "1", false},
		{`{{ x is even }}`, "", true},
		{`{{ 'a' is even }}`, "", true},
		{`{{ x is filter }}`, "", true},
		{`{{ 1 < 'a' }}`, "", true},
		{`{{ [1] < [2] }}`, "True", false},
		{`{{ (1, 2) < (1, 3) }}`, "True", false},
		{`{{ 1 is lt 'a' }}`, "", true},
		{`{{ 1|attr(2) }}`, "", true},
		{`{{ [1, 2, 3]|batch(linecount=2, fill_with=0)|list }}`, "[[1, 2], [3, 0]]", false},
		{`{{ 'ab'|center(width=6) }}`, "  ab  ", false},
		{`{{ none|default(default_value='d') }}`, "None", false},
		{`{{ ''|default(default_value='d', boolean=true) }}`, "d", false},
		{`{{ {'b': 1, 'a': 2}|dictsort(by='value') }}`, "[('b', 1), ('a', 2)]", false},
		{`{{ {'b': 1, 'A': 2}|dictsort(case_sensitive=true) }}`, "[('A', 2), ('b', 1)]", false},
		{`{{ {'b': 1, 'a': 2}|dictsort(reverse=true) }}`, "[('b', 1), ('a', 2)]", false},
		{`{{ 1500|filesizeformat(binary=true) }}`, "1.5 KiB", false},
		{`{{ 'x'|float(default=2.5) }}`, "2.5", false},
		{`{{ [{'a': 1}, {'a': 1}, {'a': 2}]|groupby(attribute='a')|list }}`, "[(1, [{'a': 1}, {'a': 1}]), (2, [{'a': 2}])]", false},
		{`{{ [{'a': 1}, {'b': 1}]|groupby(attribute='a', default=9)|list }}`, "[(1, [{'a': 1}]), (9, [{'b': 1}])]", false},
		{`{{ ['B', 'b']|groupby(attribute=0, case_sensitive=true)|list }}`, "[('B', ['B']), ('b', ['b'])]", false},
		{`{{ 'a\nb'|indent(width=2, first=true) }}`, "  a\n  b", false},
		{`{{ 'a\n\nb'|indent(width=2, blank=true) }}`, "a\n  \n  b", false},
		{`{{ 'x'|int(default=3) }}`, "3", false},
		{`{{ 'ff'|int(base=16) }}`, "255", false},
		{`{{ [1, 2]|join(d='-') }}`, "1-2", false},
		{`{{ [{'a': 1}, {'a': 2}]|join(d=',', attribute='a') }}`, "1,2", false},
		{`{{ [{'a': 1}, {'a': 2}]|map(attribute='a')|list }}`, "[1, 2]", false},
		{`{{ [{'a': 1}]|map(attribute='b', default=0)|list }}`, "[0]", false},
		{`{{ ['B', 'a']|max(case_sensitive=true) }}`, "a", false},
		{`{{ [{'a': 1}, {'a': 2}]|max(attribute='a') }}`, "{'a': 2}", false},
		{`{{ ['B', 'a']|min(case_sensitive=true) }}`, "B", false},
		{`{{ 'aaa'|replace(old='a', new='b', count=2) }}`, "bba", false},
		{`{{ 1.55|round(precision=1) }}`, "1.6", false},
		{`{{ 1.55|round(precision=1, method='floor') }}`, "1.5", false},
		{`{{ [1, 2, 3]|slice(slices=2, fill_with=0)|list }}`, "[[1, 2], [3, 0]]", false},
		{`{{ [3, 1, 2]|sort(reverse=true) }}`, "[3, 2, 1]", false},
		{`{{ ['b', 'A']|sort(case_sensitive=true) }}`, "['A', 'b']", false},
		{`{{ [{'a': 2}, {'a': 1}]|sort(attribute='a') }}`, "[{'a': 1}, {'a': 2}]", false},
		{`{{ [{'a': 2}, {'a': 1}]|sum(attribute='a') }}`, "3", false},
		{`{{ [1, 2]|sum(start=10) }}`, "13", false},
		{`{{ [1]|tojson(indent=2) }}`, "[\n  1\n]", false},
		{`{{ {'a': [1, 2]}|tojson(indent=1) }}`, "{\n \"a\": [\n  1,\n  2\n ]\n}", false},
		{`{{ {'a': [1, 2]}|tojson(indent=0) }}`, "{\n\"a\": [\n1,\n2\n]\n}", false},
		{`{{ 'xax'|trim(chars='x') }}`, "a", false},
		{`{{ 'abc def ghi'|truncate(length=7, killwords=true, end='!', leeway=0) }}`, "abc de!", false},
		{`{{ ['a', 'A']|unique(case_sensitive=true)|list }}`, "['a', 'A']", false},
		{`{{ [{'a': 1}, {'a': 1}]|unique(attribute='a')|list }}`, "[{'a': 1}]", false},
		{`{{ 'go to example.com'|urlize(trim_url_limit=5) }}`, "go to <a href=\"https://example.com\" rel=\"noopener\">examp...</a>", false},
		{`{{ 'go to example.com'|urlize(nofollow=true) }}`, "go to <a href=\"https://example.com\" rel=\"nofollow noopener\">example.com</a>", false},
		{`{{ 'go to example.com'|urlize(target='_blank') }}`, "go to <a href=\"https://example.com\" rel=\"noopener\" target=\"_blank\">example.com</a>", false},
		{`{{ 'go to example.com'|urlize(rel='x') }}`, "go to <a href=\"https://example.com\" rel=\"noopener x\">example.com</a>", false},
		{`{{ {'a': 1}|xmlattr(autospace=false) }}`, "a=\"1\"", false},
		{`{{ [1, 2, 3]|select('odd')|list }}`, "[1, 3]", false},
		{`{{ [{'a': 1}, {'a': 0}]|selectattr('a')|list }}`, "[{'a': 1}]", false},
		{`{{ 5 is divisibleby(num=5) }}`, "True", false},
		{`{{ 1 is in(seq=[1]) }}`, "True", false},
		{`{{ 1 is sameas(other=1) }}`, "True", false},
		{`{{ {'a': 1}|attr('a') }}`, "", false},
		{`{{ [1, 2]|first }}`, "1", false},
		{`{{ 'abc'|truncate(5, true) }}`, "abc", false},
		{`{{ 'abc'|truncate(2, end='') }}`, "abc", false},
		{`{{ 'go to example.com and a@b.org'|urlize(10, true, '_blank', 'x y') }}`, "go to <a href=\"https://example.com\" rel=\"nofollow noopener x y\" target=\"_blank\">example.co...</a> and <a href=\"mailto:a@b.org\">a@b.org</a>", false},
		{`{{ 'go to example.com'|urlize(0) }}`, "go to <a href=\"https://example.com\" rel=\"noopener\">...</a>", false},
		{`{{ 'mail a@b.org now'|urlize(nofollow=true, target='t') }}`, "mail <a href=\"mailto:a@b.org\">a@b.org</a> now", false},
		{`{{ {'a': 1}|attr('items') is callable }}`, "True", false},
		{`{{ [3, 1]|attr('index')(1) }}`, "1", false},
		{`{{ [{'a': [1]}, {'a': [1]}]|unique(attribute='a')|list }}`, "", true},
		{`{{ 'go example.com'|urlize(1.5) }}`, "", true},
		{`{{ 'go example.com'|urlize(3) }}`, "go <a href=\"https://example.com\" rel=\"noopener\">exa...</a>", false},
		{`{{ x|urlencode }}`, "", true},
		{`{{ [('a', 1), ('b c', '&')]|urlencode }}`, "a=1&b+c=%26", false},
		{`{{ [[1, 2, 3]]|urlencode }}`, "", true},
		{`{{ 'a b/c'|urlencode }}`, "a%20b/c", false},
		{`{{ 5|urlencode }}`, "5", false},
		{`{{ {'a': 'b c'}|urlencode }}`, "a=b+c", false},
	}
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	for _, c := range cases {
		tpl, err := e.FromString(c.src)
		if err != nil {
			t.Fatalf("FromString(%q): %v", c.src, err)
		}
		got, err := tpl.RenderContext(context.Background(), map[string]any{"x": []any{1, 2}})
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

// User-registered filters and tests have no signature and receive their
// arguments unchecked.
func TestUserFiltersSkipArity(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}),
		WithFilter("count_args", Filter{Func: func(_ *Environment, _ *runtime.Context, _ any, args []any, kwargs map[string]any) (any, error) {
			return len(args) + len(kwargs), nil
		}}))
	if got := render(t, e, `{{ 1|count_args(1, 2, 3, x=4) }}`, nil); got != "4" {
		t.Errorf("got %q, want 4", got)
	}
}

// Every builtin filter and test is registered with its Python signature.
func TestBuiltinsHaveSignatures(t *testing.T) {
	e := mustEnv(t)
	for name, f := range e.filters {
		if f.sig == nil {
			t.Errorf("filter %q has no signature", name)
		}
	}
	for name, tt := range e.tests {
		if tt.sig == nil {
			t.Errorf("test %q has no signature", name)
		}
	}
}

// Shapes common in deployment templates (string placeholders replaced
// with possibly-undefined values, defaults, type tests over config
// values) keep rendering under the argument checks.
func TestArgumentChecksKeepCommonShapes(t *testing.T) {
	e := mustEnv(t, WithAutoescape(AutoescapeNever{}))
	cases := []struct{ src, want string }{
		{`{{ 'a{{ region }}b'|replace('{{ region }}', region) }}|{{ x|length > 0 }}|{{ none|default('true', true) }}|{{ ''|default('FALSE', true) }}`, "ab|True|true|FALSE"},
		{`{% for k, v in {'a': 130000, 'b': {'hard': 1, 'soft': 2}}.items() %}{% if v is string or v is number %}{{ v }}{% elif v is iterable %}it{% endif %}{% endfor %}`, "130000it"},
		{`{{ 'a,b'.split(',')|join(',') }}|{{ ['b', 'a']|unique|sort|join(',') }}|{{ {'b': 1, 'a': 2}.items()|sort }}|{{ 'X'|lower }}|{{ 7|string }}`, "a,b|a,b|[('a', 2), ('b', 1)]|x|7"},
		{`{% set ns = namespace(l=[]) %}{% for s in ' a , b '.split(',') %}{% set ns.l = ns.l + [s.strip()] %}{% endfor %}{{ ns.l }}|{{ ('u' ~ '/events')|tojson }}|{{ 'x'|indent(4) }}`, `['a', 'b']|"u/events"|x`},
	}
	for _, c := range cases {
		if got := render(t, e, c.src, map[string]any{"x": []any{1}}); got != c.want {
			t.Errorf("%s = %q, want %q", c.src, got, c.want)
		}
	}
}
