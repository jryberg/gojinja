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
	p := &prettyPrinter{width: 80, context: map[uintptr]bool{}}
	p.format(v, 0, 0, 0)
	return p.b.String()
}

type prettyPrinter struct {
	b       strings.Builder
	width   int
	context map[uintptr]bool
}

type ppKind int

const (
	ppOther ppKind = iota
	ppDict
	ppList
	ppTuple
	ppStr
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
func (p *prettyPrinter) safeRepr(v any, context map[uintptr]bool) string {
	kind, items := ppClassify(v)
	if kind == ppOther || kind == ppStr {
		return escape.Repr(v)
	}
	if len(items) == 0 {
		switch kind {
		case ppDict:
			return "{}"
		case ppList:
			return "[]"
		}
		return "()"
	}
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
			parts[i] = p.safeRepr(kv[0], context) + ": " + p.safeRepr(kv[1], context)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	for i, it := range items {
		parts[i] = p.safeRepr(it, context)
	}
	if kind == ppList {
		return "[" + strings.Join(parts, ", ") + "]"
	}
	if len(parts) == 1 {
		return "(" + parts[0] + ",)"
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func (p *prettyPrinter) repr(v any) string {
	ctx := make(map[uintptr]bool, len(p.context))
	for k := range p.context {
		ctx[k] = true
	}
	return p.safeRepr(v, ctx)
}

// format is PrettyPrinter._format.
func (p *prettyPrinter) format(v any, indent, allowance, level int) {
	id, hasID := ppID(v)
	if hasID && p.context[id] {
		p.b.WriteString(ppRecursion(v, id))
		return
	}
	rep := p.repr(v)
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
		rep := p.repr(kv[0])
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

// formatItems is PrettyPrinter._format_items without compact mode.
func (p *prettyPrinter) formatItems(items []any, indent, allowance, level int) {
	indent++
	delim := ",\n" + strings.Repeat(" ", indent)
	for i, it := range items {
		if i > 0 {
			p.b.WriteString(delim)
		}
		a := 1
		if i == len(items)-1 {
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
