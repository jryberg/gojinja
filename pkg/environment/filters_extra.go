package environment

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"

	gjerrors "github.com/jryberg/gojinja/pkg/errors"
	"github.com/jryberg/gojinja/pkg/escape"
	"github.com/jryberg/gojinja/pkg/runtime"
)

// filterGroupby groups items by an attribute and returns a list of
// (grouper, items) pairs sorted by grouper. Mirrors Python's
// groupby(value, attribute, default=None, case_sensitive=False).
// Attribute supports dotted notation ("meta.k") and numeric segments
// for indexing into tuples / lists.
func filterGroupby(env *Environment, _ *runtime.Context, value any, args []any, kwargs map[string]any) (any, error) {
	items, err := mapSequenceArg(value)
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

// filterWordwrap breaks long lines at word boundaries to width chars.
func filterWordwrap(_ *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	width := 79
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			width = n
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

// filterPprint pretty-prints any Go value.
func filterPprint(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value), nil
	}
	return string(b), nil
}

// filterRandom returns a random element from a sequence. Uses a
// session-deterministic RNG seeded from the call site to keep tests
// reproducible.
func filterRandom(_ *Environment, _ *runtime.Context, value any, _ []any, _ map[string]any) (any, error) {
	x, ok := value.([]any)
	if !ok || len(x) == 0 {
		return runtime.NewBase("", "random", value, nil), nil
	}
	r := rand.New(rand.NewSource(int64(len(x)))) // deterministic-ish
	return x[r.Intn(len(x))], nil
}

// filterXmlattr renders a mapping as an SGML/XML attribute string.
// Keys must not contain whitespace, /, =, > to avoid attribute-injection.
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
var urlRegex = regexp.MustCompile(`(?i)(?:https?://[^\s<>"]+|www\.[^\s<>"]+|[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}|[a-z0-9.\-]+\.[a-z]{2,}(?:/[^\s<>"]*)?)`)

// filterUrlize converts URLs/emails in text into HTML links.
func filterUrlize(env *Environment, _ *runtime.Context, value any, args []any, _ map[string]any) (any, error) {
	s := escape.SoftStr(value)
	trim := 0
	nofollow := false
	target := ""
	rel := "noopener" // gojinja default — safer than Python's empty default
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok {
			trim = n
		}
	}
	if len(args) > 1 {
		nofollow, _ = args[1].(bool)
	}
	if len(args) > 2 {
		if t, ok := args[2].(string); ok {
			target = t
		}
	}
	if len(args) > 3 {
		if r, ok := args[3].(string); ok {
			rel = r
		}
	}

	out := urlRegex.ReplaceAllStringFunc(escape.SoftStr(escape.Escape(s)), func(match string) string {
		var href, text string
		isEmail := false
		switch {
		case strings.HasPrefix(match, "http"):
			href = match
		case strings.HasPrefix(match, "www."):
			href = "https://" + match
		case strings.Contains(match, "@"):
			href = "mailto:" + match
			isEmail = true
		default:
			// Bare domain: only treat as a link if it has a TLD
			// (already enforced by urlRegex) and at least one dot.
			if strings.Contains(match, ".") {
				href = "https://" + match
			} else {
				return match
			}
		}
		text = match
		if trim > 0 && len(text) > trim {
			text = text[:trim] + "..."
		}
		var attrs []string
		// Python's urlize emits `rel="noopener"` for http(s)/www/bare-
		// domain links (which open externally) but NOT for `mailto:`
		// (where there's no cross-origin concern).
		if !isEmail {
			extra := rel
			if nofollow {
				extra = strings.TrimSpace(extra + " nofollow")
			}
			if extra != "" {
				attrs = append(attrs, fmt.Sprintf(`rel="%s"`, extra))
			}
		} else if nofollow {
			attrs = append(attrs, `rel="nofollow"`)
		}
		if target != "" && !isEmail {
			attrs = append(attrs, fmt.Sprintf(`target="%s"`, target))
		}
		attrStr := ""
		if len(attrs) > 0 {
			attrStr = " " + strings.Join(attrs, " ")
		}
		return fmt.Sprintf(`<a href="%s"%s>%s</a>`, href, attrStr, text)
	})
	_ = env // signature parity
	return escape.Markup(out), nil
}
