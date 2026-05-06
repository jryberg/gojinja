package environment

import (
	"fmt"
	"strings"
	"unicode"
)

// stringMethod returns a synthetic Go callable mirroring Python's str
// instance methods. Templates valid in Python Jinja2 must work in
// gojinja — these keep `'foo'.split('-')`, `'  hi  '.strip()`,
// `'abc'.startswith('a')` etc. dispatching just like Python.
//
// Each closure rebinds s by value so calling the method later sees the
// snapshot at the time of attribute lookup. Methods that take optional
// arguments accept variadic any and coerce defensively (mirroring
// Python's flexible *args / **kwargs).
func stringMethod(s, attr string) any {
	switch attr {
	case "upper":
		return func() string { return strings.ToUpper(s) }
	case "lower":
		return func() string { return strings.ToLower(s) }
	case "title":
		return func() string {
			// Python title-cases word starts; runs of non-letters keep
			// the next letter capitalised.
			return pythonTitle(s)
		}
	case "capitalize":
		return func() string {
			if s == "" {
				return ""
			}
			rs := []rune(s)
			rs[0] = unicode.ToUpper(rs[0])
			for i := 1; i < len(rs); i++ {
				rs[i] = unicode.ToLower(rs[i])
			}
			return string(rs)
		}
	case "swapcase":
		return func() string {
			rs := []rune(s)
			for i, r := range rs {
				switch {
				case unicode.IsUpper(r):
					rs[i] = unicode.ToLower(r)
				case unicode.IsLower(r):
					rs[i] = unicode.ToUpper(r)
				}
			}
			return string(rs)
		}
	case "strip":
		return func(args ...any) string { return stripFunc(s, argString(args, 0, ""), strings.Trim) }
	case "lstrip":
		return func(args ...any) string {
			return stripFunc(s, argString(args, 0, ""), strings.TrimLeft)
		}
	case "rstrip":
		return func(args ...any) string {
			return stripFunc(s, argString(args, 0, ""), strings.TrimRight)
		}
	case "split":
		return func(args ...any) []any {
			sep := argString(args, 0, "")
			max := argInt(args, 1, -1)
			return splitToAny(s, sep, max)
		}
	case "rsplit":
		return func(args ...any) []any {
			sep := argString(args, 0, "")
			max := argInt(args, 1, -1)
			return rsplitToAny(s, sep, max)
		}
	case "splitlines":
		return func(args ...any) []any {
			keep := argBool(args, 0, false)
			return splitlines(s, keep)
		}
	case "join":
		return func(iter any) (string, error) {
			parts, err := iterToStrings(iter)
			if err != nil {
				return "", err
			}
			return strings.Join(parts, s), nil
		}
	case "startswith":
		return func(prefix any, _ ...any) bool {
			return matchAffix(prefix, s, strings.HasPrefix)
		}
	case "endswith":
		return func(suffix any, _ ...any) bool {
			return matchAffix(suffix, s, strings.HasSuffix)
		}
	case "find":
		return func(args ...any) int {
			sub := argString(args, 0, "")
			start := argInt(args, 1, 0)
			end := argInt(args, 2, len(s))
			return findIndex(s, sub, start, end, false)
		}
	case "rfind":
		return func(args ...any) int {
			sub := argString(args, 0, "")
			start := argInt(args, 1, 0)
			end := argInt(args, 2, len(s))
			return rfindIndex(s, sub, start, end, false)
		}
	case "index":
		return func(args ...any) (int, error) {
			sub := argString(args, 0, "")
			start := argInt(args, 1, 0)
			end := argInt(args, 2, len(s))
			i := findIndex(s, sub, start, end, true)
			if i < 0 {
				return -1, fmt.Errorf("substring not found")
			}
			return i, nil
		}
	case "rindex":
		return func(args ...any) (int, error) {
			sub := argString(args, 0, "")
			start := argInt(args, 1, 0)
			end := argInt(args, 2, len(s))
			i := rfindIndex(s, sub, start, end, true)
			if i < 0 {
				return -1, fmt.Errorf("substring not found")
			}
			return i, nil
		}
	case "count":
		return func(args ...any) int {
			sub := argString(args, 0, "")
			if sub == "" {
				return len(s) + 1
			}
			return strings.Count(s, sub)
		}
	case "replace":
		return func(old any, new any, args ...any) string {
			oldS := stringifyKey(old)
			newS := stringifyKey(new)
			n := argInt(args, 0, -1)
			return strings.Replace(s, oldS, newS, n)
		}
	case "format":
		return func(args ...any) string {
			return fmt.Sprintf(s, args...)
		}
	case "isdigit":
		return func() bool {
			if s == "" {
				return false
			}
			for _, r := range s {
				if !unicode.IsDigit(r) {
					return false
				}
			}
			return true
		}
	case "isalpha":
		return func() bool {
			if s == "" {
				return false
			}
			for _, r := range s {
				if !unicode.IsLetter(r) {
					return false
				}
			}
			return true
		}
	case "isalnum":
		return func() bool {
			if s == "" {
				return false
			}
			for _, r := range s {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					return false
				}
			}
			return true
		}
	case "isspace":
		return func() bool {
			if s == "" {
				return false
			}
			for _, r := range s {
				if !unicode.IsSpace(r) {
					return false
				}
			}
			return true
		}
	case "isupper":
		return func() bool {
			seen := false
			for _, r := range s {
				if unicode.IsLower(r) {
					return false
				}
				if unicode.IsUpper(r) {
					seen = true
				}
			}
			return seen
		}
	case "islower":
		return func() bool {
			seen := false
			for _, r := range s {
				if unicode.IsUpper(r) {
					return false
				}
				if unicode.IsLower(r) {
					seen = true
				}
			}
			return seen
		}
	case "encode":
		return func(args ...any) []byte {
			// Python's str.encode returns bytes; we return []byte. The
			// encoding argument is accepted but ignored — we always
			// produce UTF-8 (Go strings are UTF-8 by definition).
			return []byte(s)
		}
	case "zfill":
		return func(width any) string {
			w := argInt([]any{width}, 0, 0)
			if len(s) >= w {
				return s
			}
			pad := strings.Repeat("0", w-len(s))
			if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
				return string(s[0]) + pad + s[1:]
			}
			return pad + s
		}
	case "ljust":
		return func(args ...any) string {
			w := argInt(args, 0, 0)
			fill := argString(args, 1, " ")
			if fill == "" {
				fill = " "
			}
			if len(s) >= w {
				return s
			}
			return s + strings.Repeat(fill[:1], w-len(s))
		}
	case "rjust":
		return func(args ...any) string {
			w := argInt(args, 0, 0)
			fill := argString(args, 1, " ")
			if fill == "" {
				fill = " "
			}
			if len(s) >= w {
				return s
			}
			return strings.Repeat(fill[:1], w-len(s)) + s
		}
	case "center":
		return func(args ...any) string {
			w := argInt(args, 0, 0)
			fill := argString(args, 1, " ")
			if fill == "" {
				fill = " "
			}
			if len(s) >= w {
				return s
			}
			total := w - len(s)
			left := total / 2
			right := total - left
			return strings.Repeat(fill[:1], left) + s + strings.Repeat(fill[:1], right)
		}
	}
	return nil
}

// stripFunc applies one of the stdlib trim functions, defaulting cutset
// to whitespace when chars is empty (Python: `s.strip()` with no arg
// strips whitespace).
func stripFunc(s, chars string, fn func(string, string) string) string {
	if chars == "" {
		return strings.TrimFunc(s, unicode.IsSpace)
	}
	return fn(s, chars)
}

func splitToAny(s, sep string, max int) []any {
	var parts []string
	if sep == "" {
		parts = strings.Fields(s)
	} else if max < 0 {
		parts = strings.Split(s, sep)
	} else {
		parts = strings.SplitN(s, sep, max+1)
	}
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

func rsplitToAny(s, sep string, max int) []any {
	if sep == "" || max < 0 {
		return splitToAny(s, sep, max)
	}
	// Right-split: split from the end.
	parts := []string{}
	for i := 0; i < max; i++ {
		idx := strings.LastIndex(s, sep)
		if idx < 0 {
			break
		}
		parts = append([]string{s[idx+len(sep):]}, parts...)
		s = s[:idx]
	}
	parts = append([]string{s}, parts...)
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

func splitlines(s string, keep bool) []any {
	out := []any{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			end := i
			if keep {
				end = i + 1
				if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
					end = i + 2
					i++
				}
			} else if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			out = append(out, s[start:end])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func matchAffix(prefix any, s string, fn func(string, string) bool) bool {
	switch p := prefix.(type) {
	case string:
		return fn(s, p)
	case []any:
		for _, it := range p {
			if str, ok := it.(string); ok && fn(s, str) {
				return true
			}
		}
		return false
	}
	return false
}

func findIndex(s, sub string, start, end int, _ bool) int {
	start, end = clampSlice(len(s), start, end)
	if start > end {
		return -1
	}
	if sub == "" {
		return start
	}
	idx := strings.Index(s[start:end], sub)
	if idx < 0 {
		return -1
	}
	return idx + start
}

func rfindIndex(s, sub string, start, end int, _ bool) int {
	start, end = clampSlice(len(s), start, end)
	if start > end {
		return -1
	}
	if sub == "" {
		return end
	}
	idx := strings.LastIndex(s[start:end], sub)
	if idx < 0 {
		return -1
	}
	return idx + start
}

func clampSlice(length, start, end int) (int, int) {
	if start < 0 {
		start += length
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end += length
	}
	if end > length {
		end = length
	}
	return start, end
}

// pythonTitle implements str.title — capitalises the first letter of each
// "word" where a word is a maximal run of letters.
func pythonTitle(s string) string {
	var b strings.Builder
	prevAlpha := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			if prevAlpha {
				b.WriteRune(unicode.ToLower(r))
			} else {
				b.WriteRune(unicode.ToUpper(r))
			}
			prevAlpha = true
		} else {
			b.WriteRune(r)
			prevAlpha = false
		}
	}
	return b.String()
}

func argString(args []any, i int, def string) string {
	if i >= len(args) || args[i] == nil {
		return def
	}
	if s, ok := args[i].(string); ok {
		return s
	}
	return fmt.Sprint(args[i])
}

func argInt(args []any, i int, def int) int {
	if i >= len(args) || args[i] == nil {
		return def
	}
	switch x := args[i].(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return def
}

func argBool(args []any, i int, def bool) bool {
	if i >= len(args) || args[i] == nil {
		return def
	}
	if b, ok := args[i].(bool); ok {
		return b
	}
	return def
}

func iterToStrings(v any) ([]string, error) {
	switch x := v.(type) {
	case []any:
		out := make([]string, len(x))
		for i, it := range x {
			out[i] = stringifyKey(it)
		}
		return out, nil
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out, nil
	}
	return nil, fmt.Errorf("can only join an iterable of strings, got %T", v)
}
