// Package filters holds opt-in filters that are not part of Jinja2.
//
// Built-in filters are registered directly on the Environment in
// `pkg/environment/builtins.go` and `pkg/environment/filters_extra.go`
// to avoid an import cycle (filters call back into the engine for
// sandboxed attribute access). The filters here are never registered by
// default; enable one with [environment.WithFilter], or by name through
// [Lookup] (the CLI's `--filter` flag):
//
//	env, _ := environment.New(
//	    environment.WithFilter("base64decode", filters.Base64Decode),
//	)
package filters
