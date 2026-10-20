package environment

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"sort"
	"strings"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/eval"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// filterGroupby implements the `groupby` filter: group items by a shared
// attribute value.
//
// Signature: groupby(value, attribute, default=None, case_sensitive=False)
//
// Returns a list of `(grouper, [items])` tuples sorted by grouper, which
// also have `.grouper` and `.list` attributes. Grouping is
// case-insensitive by default; the grouper is then the first item's own
// value.
// `attribute` supports dotted notation (`meta.k`) and numeric segments
// for indexing into tuples / lists. `default` is the value used when the
// attribute is undefined.
//
// Example:
//
//	{% for kind, group in items | groupby('category') %}
//	  {{ kind }}: {{ group | length }}
//	{% endfor %}
func filterGroupby(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	items, err := iterArg("groupby", value)
	if err != nil {
		return nil, err
	}
	attribute, ok := paramArg(args, kwargs, 0, "attribute")
	if !ok {
		return nil, gjerrors.NewFilterArgumentError("groupby requires an attribute name")
	}
	def, _ := paramArg(args, kwargs, 1, "default")
	caseSensitive := false
	if v, ok := paramArg(args, kwargs, 2, "case_sensitive"); ok {
		caseSensitive = eval.Truthy(v)
	}
	expr := makeAttrGetter(env, attribute, !caseSensitive, def)
	sorted := make([]any, len(items))
	copy(sorted, items)
	if err := sortBy(sorted, expr, false); err != nil {
		return nil, err
	}
	// itertools.groupby: runs of consecutive items with equal keys.
	output := makeAttrGetter(env, attribute, false, def)
	out := []any{}
	var runKey any
	var run []any
	flush := func() error {
		if run == nil {
			return nil
		}
		grouper := runKey
		if !caseSensitive {
			// The real key of the first item, not the lowercased one.
			g, err := output(run[0])
			if err != nil {
				return err
			}
			grouper = g
		}
		out = append(out, runtime.NewGroupTuple(grouper, runtime.NewPyList(run)))
		return nil
	}
	for _, it := range sorted {
		k, err := expr(it)
		if err != nil {
			return nil, err
		}
		if run != nil {
			eq, err := eval.ValuesEqual(runKey, k)
			if err != nil {
				return nil, err
			}
			if eq {
				run = append(run, it)
				continue
			}
			if err := flush(); err != nil {
				return nil, err
			}
		}
		runKey, run = k, []any{it}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

// filterWordwrap implements the `wordwrap` filter: break long lines at
// word boundaries.
//
// Signature: wordwrap(s, width=79, break_long_words=True, wrapstring=None, break_on_hyphens=True)
//
// A port of Python's textwrap.wrap: each existing line is wrapped on its
// own at whitespace and after hyphens (break_on_hyphens), words longer
// than `width` are split (break_long_words), and lines are joined with
// `wrapstring`, which defaults to the environment's newline sequence.
//
// Example:
//
//	{{ "the quick brown fox" | wordwrap(10) }}
//	→  "the quick\nbrown fox"
func filterWordwrap(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	var widthVal any = int64(79)
	if v, ok := paramArg(args, kwargs, 0, "width"); ok {
		widthVal = v
	}
	breakLong := true
	if v, ok := paramArg(args, kwargs, 1, "break_long_words"); ok {
		breakLong = eval.Truthy(v)
	}
	wrapstring := env.lexerOpts.NewlineSequence
	if v, ok := paramArg(args, kwargs, 2, "wrapstring"); ok && v != nil {
		wrapstring = escape.SoftStr(v)
	}
	breakHyphens := true
	if v, ok := paramArg(args, kwargs, 3, "break_on_hyphens"); ok {
		breakHyphens = eval.Truthy(v)
	}
	// Each line is wrapped on its own, so existing newlines are kept.
	var paras []string
	for _, line := range pySplitlines(escape.SoftStr(value), false) {
		// TextWrapper rejects width <= 0 whenever it wraps a line.
		bad, err := eval.Compare("lteq", widthVal, int64(0))
		if err != nil {
			return nil, gjerrors.NewFilterArgumentError(err.Error())
		}
		if bad {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("invalid width %v (must be > 0)", escape.SoftStr(widthVal)))
		}
		width, err := indexArg(widthVal)
		if err != nil {
			return nil, err
		}
		paras = append(paras, strings.Join(textWrap(line, width, breakLong, breakHyphens), wrapstring))
	}
	return strings.Join(paras, wrapstring), nil
}

// filterPprint implements the `pprint` filter: pretty-print a value.
//
// Signature: pprint(value)
//
// Python's pprint.pformat with its defaults: reprs on one line when they
// fit in 80 columns, otherwise dicts (keys sorted), lists and tuples one
// item per line and long strings split into adjacent literals.
//
// Example:
//
//	{{ {'b': 1, 'a': [1, 2]} | pprint }}  →  {'a': [1, 2], 'b': 1}
func filterPprint(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	return pformat(value), nil
}

// filterRandom implements the `random` filter: pick a random element from
// a sequence.
//
// Signature: random(seq)
//
// Like Python's random.choice, every call picks independently. Empty
// sequences yield Undefined. A mapping is indexed with the chosen
// position, as Python does, so it raises unless it has that int key.
//
// Example:
//
//	{{ ['rock', 'paper', 'scissors'] | random }}
func filterRandom(env *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	x, err := iterArg("random", value)
	if err != nil {
		return nil, err
	}
	if len(x) == 0 {
		return env.undefined("No random item, sequence was empty.", "", runtime.Missing, nil), nil
	}
	i := rand.IntN(len(x))
	switch value.(type) {
	case map[string]any, map[any]any, *runtime.OrderedDict:
		// random.choice(seq) is seq[i].
		v, err := env.GetItem(value, int64(i))
		if err != nil {
			return nil, err
		}
		if runtime.IsUndefined(v) {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("KeyError: %d", i))
		}
		return v, nil
	}
	return x[i], nil
}

// filterXmlattr implements the `xmlattr` filter: render a mapping as
// an SGML/XML attribute string.
//
// Signature: xmlattr(d, autospace=True)
//
// Keys containing whitespace, `/`, `=`, or `>` are rejected to prevent
// attribute injection. `None` and Undefined values are skipped (no
// attribute emitted). Values are HTML-escaped. With `autospace=True`,
// the result is prefixed with a space so it slots straight after a tag
// name.
//
// Example:
//
//	<div{{ {'class': 'red', 'id': 'x'} | xmlattr }}>
//	→  <div class="red" id="x">
func filterXmlattr(_ *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	d, ok := value.(*runtime.OrderedDict)
	if !ok {
		d, ok = runtime.NormalizeForTemplate(value).(*runtime.OrderedDict)
	}
	if !ok || d == nil {
		return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("'%s' object has no attribute 'items'", runtime.PyTypeName(value)))
	}
	autospace := true
	if v, ok := paramArg(args, kwargs, 0, "autospace"); ok {
		autospace = eval.Truthy(v)
	}
	var items []string
	for _, it := range d.Items() {
		kv := it.(runtime.Tuple)
		key, val := kv[0], kv[1]
		if val == nil || runtime.IsUndefined(val) {
			continue
		}
		k, ok := eqString(key)
		if !ok {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("expected string or bytes-like object, got '%s'", runtime.PyTypeName(key)))
		}
		if xmlattrKeyRe.MatchString(k) {
			return nil, gjerrors.NewFilterArgumentError("Invalid character in attribute name: " + escape.Repr(key))
		}
		items = append(items, string(escape.Escape(key))+`="`+string(escape.Escape(val))+`"`)
	}
	out := strings.Join(items, " ")
	if autospace && out != "" {
		out = " " + out
	}
	if ctx != nil && ctx.Eval != nil && ctx.Eval.Autoescape {
		return escape.Markup(out), nil
	}
	return out, nil
}

// xmlattrKeyRe is Jinja2's _attr_key_re: characters a key may not hold.
var xmlattrKeyRe = regexp.MustCompile(`[\t\n\v\f\r /=>]`)

// urlRegex matches URL-ish substrings: http(s)://, www., bare emails,
// and bare domains (`example.com`). Mirrors Python's urlize coverage
// closely enough for template parity.
// Regexes ported from jinja2.utils.urlize. Python's \w, \d and \s are
// Unicode-aware, so they are spelled out here; Go's are ASCII-only.
const (
	pyWord  = `\p{L}\p{N}_`
	pySpace = `\t\n\v\f\r \x1c-\x1f\x85\p{Z}`
)

var (
	urlizeSplitRe = regexp.MustCompile(`[` + pySpace + `]+`)
	urlizeHeadRe  = regexp.MustCompile(`^(?:[(<]|&lt;)+`)
	urlizeTailRe  = regexp.MustCompile(`(?:[)>.,\n]|&gt;)+$`)
	urlizeHTTPRe  = regexp.MustCompile(`(?i)^(?:` +
		`(?:https?://|www\.)(?:(?:[` + pyWord + `%-]+\.)+)?(?:[a-z]{2,63}|xn--[` + pyWord + `%]{2,59})` +
		`|(?:[` + pyWord + `%-]{2,63}\.)+(?:com|net|int|edu|gov|org|info|mil)` +
		`|https?://(?:\p{Nd}{1,3}(?:\.\p{Nd}{1,3}){3}|\[(?:[\p{Nd}a-f]{0,4}:){2}(?:[\p{Nd}a-f]{0,4}:?){1,6}\])` +
		`)(?::\p{Nd}{1,5})?(?:[/?#][^` + pySpace + `]*)?$`)
	urlizeEmailRe = regexp.MustCompile(`^[^` + pySpace + `]+@[` + pyWord + `][` + pyWord + `.-]*\.[` + pyWord + `]+$`)
	uriSchemeRe   = regexp.MustCompile(`^[` + pyWord + `.+-]{2,}:/{0,2}$`)
)

// filterUrlize implements the `urlize` filter: convert URLs and email
// addresses in text into HTML links.
//
// Signature: urlize(value, trim_url_limit=None, nofollow=False, target=None, rel=None, extra_schemes=None)
//
// `trim_url_limit` truncates display text (the href stays full). Web
// links get `rel` set to the sorted union of `rel`, "nofollow" (with
// `nofollow=True`) and the "noopener" policy, plus `target` when given;
// `mailto:` links get neither. Words starting with one of
// `extra_schemes` (validated like Python's) are linked too.
//
// The input is escaped first unless it is already Markup, and the result
// is Markup only under autoescape (as in Python).
//
// Example:
//
//	{{ "Visit https://example.com" | urlize }}
//	→  Visit <a href="https://example.com" rel="noopener">https://example.com</a>
func filterUrlize(env *Environment, ctx *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	// trim_url_limit=None leaves link text alone. Like Python it is only
	// compared and sliced with once a link needs trimming.
	var trim any
	if v, ok := paramArg(args, kwargs, 0, "trim_url_limit"); ok {
		trim = v
	}
	// rel is the sorted union of the rel argument, "nofollow" when asked
	// for, and the urlize.rel policy ("noopener"), as in do_urlize.
	relParts := map[string]bool{"noopener": true}
	if v, ok := paramArg(args, kwargs, 3, "rel"); ok && v != nil {
		for _, p := range strings.Fields(escape.SoftStr(v)) {
			relParts[p] = true
		}
	}
	if v, ok := paramArg(args, kwargs, 1, "nofollow"); ok && eval.Truthy(v) {
		relParts["nofollow"] = true
	}
	rels := make([]string, 0, len(relParts))
	for p := range relParts {
		rels = append(rels, p)
	}
	sort.Strings(rels)
	linkAttrs := fmt.Sprintf(` rel="%s"`, escape.Escape(strings.Join(rels, " ")))
	if v, ok := paramArg(args, kwargs, 2, "target"); ok && v != nil && eval.Truthy(v) {
		linkAttrs += fmt.Sprintf(` target="%s"`, escape.Escape(v))
	}

	var schemes []string
	if v, ok := paramArg(args, kwargs, 4, "extra_schemes"); ok && v != nil {
		items, err := iterArg("urlize", v)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			scheme := escape.SoftStr(it)
			if !uriSchemeRe.MatchString(scheme) {
				return nil, gjerrors.NewFilterArgumentError(escape.Repr(scheme) + " is not a valid URI scheme prefix.")
			}
			schemes = append(schemes, scheme)
		}
	}
	var trimErr error
	trimURL := func(u string) string {
		if trim == nil || trimErr != nil {
			return u
		}
		rs := []rune(u)
		long, err := eval.Compare("gt", int64(len(rs)), trim)
		if err == nil && long {
			var n int
			if n, err = indexArg(trim); err == nil {
				u = string(rs[:max(0, min(n, len(rs)))]) + "..."
			}
		}
		trimErr = err
		return u
	}
	// escape() leaves a Markup subject as is.
	text := string(escape.Escape(value))
	var b strings.Builder
	pos := 0
	for _, loc := range append(urlizeSplitRe.FindAllStringIndex(text, -1), []int{len(text), len(text)}) {
		b.WriteString(urlizeWord(text[pos:loc[0]], linkAttrs, schemes, trimURL))
		b.WriteString(text[loc[0]:loc[1]])
		pos = loc[1]
	}
	if trimErr != nil {
		return nil, trimErr
	}
	_ = env // signature parity
	if autoescapeOn(ctx) {
		return escape.Markup(b.String()), nil
	}
	return b.String(), nil
}

// urlizeWord links one whitespace-delimited word the way
// jinja2.utils.urlize does, keeping surrounding punctuation outside the
// link and balancing brackets.
func urlizeWord(word, attrs string, schemes []string, trimURL func(string) string) string {
	head, middle, tail := "", word, ""
	if m := urlizeHeadRe.FindString(middle); m != "" {
		head, middle = m, middle[len(m):]
	}
	for _, suffix := range []string{")", ">", ".", ",", "\n", "&gt;"} {
		if strings.HasSuffix(middle, suffix) {
			if loc := urlizeTailRe.FindStringIndex(middle); loc != nil {
				tail, middle = middle[loc[0]:], middle[:loc[0]]
			}
			break
		}
	}
	// Prefer balancing brackets in the URL over leaving a trailing one.
	for _, pair := range [][2]string{{"(", ")"}, {"<", ">"}, {"&lt;", "&gt;"}} {
		opens := strings.Count(middle, pair[0])
		if opens <= strings.Count(middle, pair[1]) {
			continue
		}
		for range min(opens, strings.Count(tail, pair[1])) {
			end := strings.Index(tail, pair[1]) + len(pair[1])
			middle += tail[:end]
			tail = tail[end:]
		}
	}
	switch {
	case urlizeHTTPRe.MatchString(middle):
		href := middle
		if !strings.HasPrefix(middle, "https://") && !strings.HasPrefix(middle, "http://") {
			href = "https://" + middle
		}
		middle = fmt.Sprintf(`<a href="%s"%s>%s</a>`, href, attrs, trimURL(middle))
	case strings.HasPrefix(middle, "mailto:") && urlizeEmailRe.MatchString(middle[7:]):
		middle = fmt.Sprintf(`<a href="%s">%s</a>`, middle, middle[7:])
	case strings.Contains(middle, "@") && !strings.HasPrefix(middle, "www.") &&
		!strings.HasPrefix(middle, "@") && !strings.Contains(middle, ":") && urlizeEmailRe.MatchString(middle):
		middle = fmt.Sprintf(`<a href="mailto:%s">%s</a>`, middle, middle)
	default:
		for _, scheme := range schemes {
			if middle != scheme && strings.HasPrefix(middle, scheme) {
				middle = fmt.Sprintf(`<a href="%s"%s>%s</a>`, middle, attrs, middle)
			}
		}
	}
	return head + middle + tail
}
