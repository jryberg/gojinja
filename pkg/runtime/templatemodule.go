package runtime

import (
	"strings"

	"github.com/jryberg/gojinja/pkg/escape"
)

// TemplateModule is what `{% import %}` binds, a port of Jinja2's
// TemplateModule: the imported template's exported names, those not
// starting with an underscore, are its attributes, and printing it gives
// the template's rendered output.
type TemplateModule struct {
	name    string
	exports map[string]any
	body    string
}

// NewTemplateModule is the module of the template name, whose top-level
// names are vars and whose rendered output is body.
func NewTemplateModule(name string, vars map[string]any, body string) *TemplateModule {
	exports := make(map[string]any, len(vars))
	for k, v := range vars {
		if !strings.HasPrefix(k, "_") {
			exports[k] = v
		}
	}
	return &TemplateModule{name: name, exports: exports, body: body}
}

// Get returns an exported name.
func (m *TemplateModule) Get(attr string) (any, bool) {
	v, ok := m.exports[attr]
	return v, ok
}

// TypeName is the Python type name.
func (m *TemplateModule) TypeName() string { return "TemplateModule" }

// String is str(): the rendered output.
func (m *TemplateModule) String() string { return m.body }

// HTML is __html__: the rendered output, not escaped again.
func (m *TemplateModule) HTML() escape.Markup { return escape.Markup(m.body) }

// Repr is repr(): `<TemplateModule 'name'>`.
func (m *TemplateModule) Repr() string { return "<TemplateModule " + escape.Repr(m.name) + ">" }
