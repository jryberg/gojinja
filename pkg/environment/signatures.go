package environment

import (
	"fmt"
	"sort"
	"strings"
)

// sigStyle selects which CPython TypeError wording a builtin uses: Python
// functions, one-argument C builtins (abs, len, callable), or the operator
// module's comparison functions.
type sigStyle int

const (
	stylePython sigStyle = iota
	styleCOneArg
	styleOperator
)

// signature is the Python signature of a builtin filter or test, used to
// reject calls that Jinja2 would fail with a TypeError. The tables in
// signatures_table.go are generated from the Jinja2 pinned in
// tools/parity/requirements.txt (`make signatures`).
type signature struct {
	fn       string   // Python callable name used in the messages
	params   []string // positional parameters, including any pass_* argument and the value
	required int      // leading params without a default
	posOnly  int      // leading positional-only params
	hidden   bool     // first param is the pass_context/environment/eval_context argument
	varargs  bool
	varkw    bool
	style    sigStyle
}

// check reports the TypeError CPython raises when the callable is invoked
// with the value, nargs further positional arguments and kwargs.
func (s *signature) check(nargs int, kwargs map[string]any) string {
	given := 1 + nargs
	if s.hidden {
		given++
	}
	switch s.style {
	case styleCOneArg:
		if len(kwargs) > 0 {
			return s.fn + "() takes no keyword arguments"
		}
		if given != 1 {
			return fmt.Sprintf("%s() takes exactly one argument (%d given)", s.fn, given)
		}
		return ""
	case styleOperator:
		if len(kwargs) > 0 {
			return "_operator." + s.fn + "() takes no keyword arguments"
		}
		if given != 2 {
			return fmt.Sprintf("%s expected 2 arguments, got %d", s.fn, given)
		}
		return ""
	}
	names := make([]string, 0, len(kwargs))
	for k := range kwargs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		idx := s.index(k)
		switch {
		case idx >= 0 && idx < s.posOnly && !s.varkw:
			return fmt.Sprintf("%s() got some positional-only arguments passed as keyword arguments: '%s'", s.fn, k)
		case idx < 0 && !s.varkw:
			return fmt.Sprintf("%s() got an unexpected keyword argument '%s'", s.fn, k)
		case idx >= s.posOnly && idx < given:
			return fmt.Sprintf("%s() got multiple values for argument '%s'", s.fn, k)
		}
	}
	if given > len(s.params) && !s.varargs {
		takes := fmt.Sprintf("%d positional argument%s", len(s.params), plural(len(s.params)))
		if s.required < len(s.params) {
			takes = fmt.Sprintf("from %d to %d positional arguments", s.required, len(s.params))
		}
		were := "were"
		if given == 1 {
			were = "was"
		}
		return fmt.Sprintf("%s() takes %s but %d %s given", s.fn, takes, given, were)
	}
	var missing []string
	for i := given; i < s.required; i++ {
		if _, ok := kwargs[s.params[i]]; !ok {
			missing = append(missing, "'"+s.params[i]+"'")
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("%s() missing %d required positional argument%s: %s", s.fn, len(missing), plural(len(missing)), pyNameList(missing))
	}
	return ""
}

// bind binds a call of a Python function that is not handed a value, as
// a global is: it reports CPython's TypeError for the call, or returns
// every parameter's argument, with defaults for those not passed.
func (s *signature) bind(args []any, kwargs map[string]any, defaults []any) ([]any, string) {
	if msg := s.check(len(args)-1, kwargs); msg != "" {
		return nil, msg
	}
	out := append([]any(nil), defaults...)
	copy(out, args)
	for i, p := range s.params {
		if v, ok := kwargs[p]; ok {
			out[i] = v
		}
	}
	return out, ""
}

// normalize moves keyword arguments that fill the next positional slots
// into args, so a call like join(d='-') reaches the filter exactly as
// join('-') would. Keywords that skip an earlier parameter stay in
// kwargs. kwargs is not modified.
func (s *signature) normalize(args []any, kwargs map[string]any) ([]any, map[string]any) {
	if s.varargs || len(kwargs) == 0 {
		return args, kwargs
	}
	pos := 1 + len(args)
	if s.hidden {
		pos++
	}
	var rest map[string]any
	for ; pos < len(s.params); pos++ {
		v, ok := kwargs[s.params[pos]]
		if !ok {
			break
		}
		if rest == nil {
			rest = make(map[string]any, len(kwargs))
			for k, kv := range kwargs {
				rest[k] = kv
			}
			args = append([]any(nil), args...)
		}
		args = append(args, v)
		delete(rest, s.params[pos])
	}
	if rest == nil {
		return args, kwargs
	}
	return args, rest
}

func (s *signature) index(name string) int {
	for i, p := range s.params {
		if p == name {
			return i
		}
	}
	return -1
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// pyNameList joins names the way CPython lists missing arguments:
// 'a' · 'a' and 'b' · 'a', 'b', and 'c'.
func pyNameList(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
}
