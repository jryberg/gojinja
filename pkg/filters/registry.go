package filters

import (
	"sort"

	"github.com/jryberg/gojinja/pkg/environment"
)

var optional = map[string]environment.Filter{
	"base64decode": Base64Decode,
}

// Lookup returns the opt-in filter registered under name.
func Lookup(name string) (environment.Filter, bool) {
	f, ok := optional[name]
	return f, ok
}

// Names returns the names of all opt-in filters, sorted.
func Names() []string {
	names := make([]string, 0, len(optional))
	for n := range optional {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
