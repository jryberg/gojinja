package environment

import (
	"encoding/json"
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
// Returns a list of `(grouper, [items])` tuples sorted by grouper.
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
	var attr string
	if len(args) > 0 {
		switch a := args[0].(type) {
		case string:
			attr = a
		case int:
			attr = fmt.Sprint(a)
		case int64:
			attr = fmt.Sprint(a)
		}
	} else if v, ok := kwargs["attribute"]; ok {
		switch a := v.(type) {
		case string:
			attr = a
		case int:
			attr = fmt.Sprint(a)
		case int64:
			attr = fmt.Sprint(a)
		}
	}
	if attr == "" {
		return nil, gjerrors.NewFilterArgumentError("groupby requires an attribute name")
	}
	defaultVal, hasDefault := kwargs["default"]
	if !hasDefault && len(args) > 1 {
		defaultVal = args[1]
		hasDefault = true
	}
	caseSensitive := false
	if v, ok := kwargs["case_sensitive"].(bool); ok {
		caseSensitive = v
	} else if len(args) > 2 {
		caseSensitive, _ = args[2].(bool)
	}
	type group struct {
		Key   any
		Items []any
	}
	groups := map[string]*group{}
	order := []string{}
	for _, it := range items {
		key, err := lookupDottedAttr(env, it, attr)
		if err != nil {
			return nil, err
		}
		if u, ok := key.(runtime.Undefiner); ok && u.IsUndefined() {
			if hasDefault {
				key = defaultVal
			}
		}
		k := fmt.Sprintf("%v", key)
		if !caseSensitive {
			if s, ok := key.(string); ok {
				k = strings.ToLower(s)
			}
		}
		g, ok := groups[k]
		if !ok {
			g = &group{Key: key}
			groups[k] = g
			order = append(order, k)
		}
		g.Items = append(g.Items, it)
	}
	sort.Strings(order)
	out := make([]any, 0, len(order))
	for _, k := range order {
		g := groups[k]
		out = append(out, runtime.Tuple{g.Key, g.Items})
	}
	return out, nil
}

// filterWordwrap implements the `wordwrap` filter: break long lines at
// word boundaries.
//
// Signature: wordwrap(s, width=79)
//
// Existing newlines are preserved; only lines longer than `width` get
// rewrapped at whitespace.
//
// Example:
//
//	{{ "the quick brown fox" | wordwrap(10) }}
//	→  "the quick\nbrown fox"
func filterWordwrap(_ *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	s := escape.SoftStr(value)
	width := 79
	if v, ok := paramArg(args, kwargs, 0, "width"); ok {
		// TextWrapper first checks `width <= 0`.
		bad, err := eval.Compare("lteq", v, int64(0))
		if err != nil {
			return nil, gjerrors.NewFilterArgumentError(err.Error())
		}
		if bad {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("invalid width %v (must be > 0)", escape.SoftStr(v)))
		}
		if width, err = indexArg(v); err != nil {
			return nil, err
		}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if len(para) <= width {
			out = append(out, para)
			continue
		}
		words := strings.Fields(para)
		var line strings.Builder
		for _, w := range words {
			if line.Len() == 0 {
				line.WriteString(w)
				continue
			}
			if line.Len()+1+len(w) > width {
				out = append(out, line.String())
				line.Reset()
				line.WriteString(w)
				continue
			}
			line.WriteByte(' ')
			line.WriteString(w)
		}
		if line.Len() > 0 {
			out = append(out, line.String())
		}
	}
	return strings.Join(out, "\n"), nil
}

// filterPprint implements the `pprint` filter: pretty-print any value as
// indented JSON.
//
// Signature: pprint(value)
//
// Useful for debugging template context. Falls back to `fmt.Sprintf("%v")`
// for values JSON can't represent.
//
// Example:
//
//	{{ ctx | pprint }}
func filterPprint(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value), nil
	}
	return string(b), nil
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
func filterXmlattr(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	// Accept both map[string]any (engine-side) and map[any]any (the
	// shape produced by `{...}` dict literals).
	m := make(map[string]any)
	switch x := value.(type) {
	case map[string]any:
		for k, v := range x {
			m[k] = v
		}
	case map[any]any:
		for k, v := range x {
			ks, ok := k.(string)
			if !ok {
				return nil, gjerrors.NewFilterArgumentError("xmlattr keys must be strings")
			}
			m[ks] = v
		}
	case *runtime.OrderedDict:
		for _, k := range x.Keys() {
			ks, ok := k.(string)
			if !ok {
				return nil, gjerrors.NewFilterArgumentError("xmlattr keys must be strings")
			}
			v, _ := x.Get(k)
			m[ks] = v
		}
	default:
		return nil, gjerrors.NewFilterArgumentError("xmlattr requires a mapping")
	}
	autospace := true
	if len(args) > 0 {
		autospace, _ = args[0].(bool)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if strings.ContainsAny(k, " \t\n\r/>=") {
			return nil, gjerrors.NewFilterArgumentError(fmt.Sprintf("xmlattr: invalid attribute name %q", k))
		}
		v := m[k]
		if v == nil {
			continue
		}
		if u, ok := v.(runtime.Undefiner); ok && u.IsUndefined() {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(string(escape.Escape(escape.SoftStr(v))))
		b.WriteByte('"')
	}
	out := b.String()
	if !autospace && strings.HasPrefix(out, " ") {
		out = out[1:]
	}
	return escape.Markup(out), nil
}

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
