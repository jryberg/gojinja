package environment

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// pformat is Python's pprint.pformat(obj) with its defaults (indent=1,
// width=80, sort_dicts=True): a port of PrettyPrinter._format and the
// dict, list, tuple and str pretty-printers. Widths count characters.
func pformat(v any) string {
	return pformatWith(v, 0, false)
}

// Pformat is pprint.pformat(obj, depth=depth, compact=compact) with the
// other defaults: depth 0 is unlimited; compact puts as many items of a
// long list or tuple on each line as fit.
func (e *Environment) Pformat(v any, depth int, compact bool) string {
	return pformatWith(v, depth, compact)
}

func pformatWith(v any, depth int, compact bool) string {
	p := &prettyPrinter{width: 80, context: map[uintptr]bool{}, maxLevels: depth, compact: compact}
	p.format(v, 0, 0, 0)
	return p.b.String()
}

type prettyPrinter struct {
	b         strings.Builder
	width     int
	context   map[uintptr]bool
	depth     int
	maxLevels int
	compact   bool
}

// maxPprintDepth bounds pprint's nesting. CPython's pprint recurses in
// Python code and fails near 330 levels; its cost also grows with the
// square of the depth, as every level re-renders the levels below.
const maxPprintDepth = 1000

// nest guards one level of pprint recursion.
func (p *prettyPrinter) nest() func() {
	p.depth++
	if p.depth > maxPprintDepth {
		panic(escape.RecursionError{})
	}
	return func() { p.depth-- }
}

type ppKind int

const (
	ppOther ppKind = iota
	ppDict
	ppList
	ppTuple
	ppStr
	ppProxy
)

// ppClassify maps v to the Python type whose pretty-printer applies and
// returns its items (dict items as key/value tuples).
func ppClassify(v any) (ppKind, []any) {
	switch x := v.(type) {
	case string:
		return ppStr, nil
	case runtime.Tuple:
		return ppTuple, x
	case []any:
		return ppList, x
	case *runtime.PyList:
		return ppList, x.Items()
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		return ppList, items
	case *runtime.OrderedDict:
		if x.IsProxy() {
			return ppProxy, nil
		}
		return ppDict, x.Items()
	case map[string]any:
		items := make([]any, 0, len(x))
		for k, val := range x {
			items = append(items, runtime.Tuple{k, val})
		}
		return ppDict, items
	case map[any]any:
		items := make([]any, 0, len(x))
		for k, val := range x {
			items = append(items, runtime.Tuple{k, val})
		}
		return ppDict, items
	}
	return ppOther, nil
}

// ppID identifies a container for recursion detection, like id().
func ppID(v any) (uintptr, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.Pointer() != 0 {
			return rv.Pointer(), true
		}
	}
	return 0, false
}

// sortedDictItems orders dict items like sorted(items, key=_safe_tuple):
// by key with `<`, falling back to the type name for unorderable keys.
func sortedDictItems(items []any) []any {
	out := append([]any(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].(runtime.Tuple)[0], out[j].(runtime.Tuple)[0]
		if lt, err := eval.Compare("lt", a, b); err == nil {
			return lt
		}
		return ppClassName(a) < ppClassName(b)
	})
	return out
}

func ppClassName(v any) string {
	return "<class '" + runtime.PyTypeName(v) + "'>"
}

func ppRecursion(v any, id uintptr) string {
	return fmt.Sprintf("<Recursion on %s with id=%d>", runtime.PyTypeName(v), id)
}

// safeRepr is PrettyPrinter._safe_repr: repr with dict keys sorted.
func (p *prettyPrinter) safeRepr(v any, context map[uintptr]bool, level int) string {
	defer p.nest()()
	kind, items := ppClassify(v)
	if kind == ppOther || kind == ppStr || kind == ppProxy {
		return escape.Repr(v)
	}
	open, closer := "(", ")"
	switch kind {
	case ppDict:
		open, closer = "{", "}"
	case ppList:
		open, closer = "[", "]"
	}
	if len(items) == 0 {
		return open + closer
	}
	if p.maxLevels > 0 && level >= p.maxLevels {
		return open + "..." + closer
	}
	level++
	id, hasID := ppID(v)
	if hasID {
		if context[id] {
			return ppRecursion(v, id)
		}
		context[id] = true
		defer delete(context, id)
	}
	parts := make([]string, len(items))
	if kind == ppDict {
		for i, it := range sortedDictItems(items) {
			kv := it.(runtime.Tuple)
			parts[i] = p.safeRepr(kv[0], context, level) + ": " + p.safeRepr(kv[1], context, level)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	for i, it := range items {
		parts[i] = p.safeRepr(it, context, level)
	}
	if kind == ppList {
		return "[" + strings.Join(parts, ", ") + "]"
	}
	if len(parts) == 1 {
		return "(" + parts[0] + ",)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func (p *prettyPrinter) repr(v any, level int) string {
	ctx := make(map[uintptr]bool, len(p.context))
	for k := range p.context {
		ctx[k] = true
	}
	return p.safeRepr(v, ctx, level)
}

// format is PrettyPrinter._format.
func (p *prettyPrinter) format(v any, indent, allowance, level int) {
	defer p.nest()()
	id, hasID := ppID(v)
	if hasID && p.context[id] {
		p.b.WriteString(ppRecursion(v, id))
		return
	}
	rep := p.repr(v, level)
	if utf8.RuneCountInString(rep) > p.width-indent-allowance {
		kind, items := ppClassify(v)
		if kind != ppOther {
			if hasID {
				p.context[id] = true
				defer delete(p.context, id)
			}
			switch kind {
			case ppDict:
				p.b.WriteByte('{')
				if len(items) > 0 {
					p.formatDictItems(sortedDictItems(items), indent, allowance+1, level+1)
				}
				p.b.WriteByte('}')
			case ppList:
				p.b.WriteByte('[')
				p.formatItems(items, indent, allowance+1, level+1)
				p.b.WriteByte(']')
			case ppTuple:
				end := ")"
				if len(items) == 1 {
					end = ",)"
				}
				p.b.WriteByte('(')
				p.formatItems(items, indent, allowance+len(end), level+1)
				p.b.WriteString(end)
			case ppStr:
				p.pprintStr(v.(string), indent, allowance, level+1)
			case ppProxy:
				// _pprint_mappingproxy formats a copy of the dict.
				p.b.WriteString("mappingproxy(")
				p.format(v.(*runtime.OrderedDict).Copy(), indent+13, allowance+1, level)
				p.b.WriteByte(')')
			}
			return
		}
	}
	p.b.WriteString(rep)
}

// formatDictItems is PrettyPrinter._format_dict_items.
func (p *prettyPrinter) formatDictItems(items []any, indent, allowance, level int) {
	indent++
	delim := ",\n" + strings.Repeat(" ", indent)
	for i, it := range items {
		kv := it.(runtime.Tuple)
		last := i == len(items)-1
		rep := p.repr(kv[0], level)
		p.b.WriteString(rep)
		p.b.WriteString(": ")
		a := 1
		if last {
			a = allowance
		}
		p.format(kv[1], indent+utf8.RuneCountInString(rep)+2, a, level)
		if !last {
			p.b.WriteString(delim)
		}
	}
}

// formatItems is PrettyPrinter._format_items.
func (p *prettyPrinter) formatItems(items []any, indent, allowance, level int) {
	indent++
	delimNL := ",\n" + strings.Repeat(" ", indent)
	delim := ""
	width := p.width - indent + 1
	maxWidth := width
	for i, it := range items {
		last := i == len(items)-1
		if last {
			maxWidth -= allowance
			width -= allowance
		}
		if p.compact {
			rep := p.repr(it, level)
			w := utf8.RuneCountInString(rep) + 2
			if width < w {
				width = maxWidth
				if delim != "" {
					delim = delimNL
				}
			}
			if width >= w {
				width -= w
				p.b.WriteString(delim)
				delim = ", "
				p.b.WriteString(rep)
				continue
			}
		}
		p.b.WriteString(delim)
		delim = delimNL
		a := 1
		if last {
			a = allowance
		}
		p.format(it, indent, a, level)
	}
}

// pprintStr is PrettyPrinter._pprint_str: long strings are split into
// adjacent literals at line ends and whitespace.
func (p *prettyPrinter) pprintStr(s string, indent, allowance, level int) {
	if level == 1 {
		indent++
		allowance++
	}
	reprLen := func(x string) int { return utf8.RuneCountInString(escape.Repr(x)) }
	var chunks []string
	lines := pySplitlines(s, true)
	maxWidth1 := p.width - indent
	maxWidth := maxWidth1
	rep := ""
	for i, line := range lines {
		rep = escape.Repr(line)
		if i == len(lines)-1 {
			maxWidth1 -= allowance
		}
		if utf8.RuneCountInString(rep) <= maxWidth1 {
			chunks = append(chunks, rep)
			continue
		}
		parts := spaceParts(line)
		maxWidth2 := maxWidth
		current := ""
		for j, part := range parts {
			candidate := current + part
			if j == len(parts)-1 && i == len(lines)-1 {
				maxWidth2 -= allowance
			}
			if reprLen(candidate) > maxWidth2 {
				if current != "" {
					chunks = append(chunks, escape.Repr(current))
				}
				current = part
			} else {
				current = candidate
			}
		}
		if current != "" {
			chunks = append(chunks, escape.Repr(current))
		}
	}
	if len(chunks) == 1 {
		p.b.WriteString(rep)
		return
	}
	if level == 1 {
		p.b.WriteByte('(')
	}
	for i, c := range chunks {
		if i > 0 {
			p.b.WriteString("\n" + strings.Repeat(" ", indent))
		}
		p.b.WriteString(c)
	}
	if level == 1 {
		p.b.WriteByte(')')
	}
}

// spaceParts is re.findall(r'\S*\s*', line) without the final empty
// match: alternating runs of non-space then space.
func spaceParts(line string) []string {
	rs := []rune(line)
	var parts []string
	for i := 0; i < len(rs); {
		j := i
		for j < len(rs) && !pyIsSpace(rs[j]) {
			j++
		}
		for j < len(rs) && pyIsSpace(rs[j]) {
			j++
		}
		parts = append(parts, string(rs[i:j]))
		i = j
	}
	return parts
}
