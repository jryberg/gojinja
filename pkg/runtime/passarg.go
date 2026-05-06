package runtime

// PassArg names the categories of "first-arg injection" that filters and
// tests can opt into. These are the equivalents of Python's
// pass_context / pass_eval_context / pass_environment decorators.
type PassArg int

const (
	// PassNone — the function takes only its declared arguments.
	PassNone PassArg = iota
	// PassContext — runtime injects the *Context as the first argument.
	PassContext
	// PassEvalContext — runtime injects the *EvalContext.
	PassEvalContext
	// PassEnvironment — runtime injects the Environment.
	PassEnvironment
)

// String returns a human-readable label, useful for diagnostics.
func (p PassArg) String() string {
	switch p {
	case PassContext:
		return "pass_context"
	case PassEvalContext:
		return "pass_eval_context"
	case PassEnvironment:
		return "pass_environment"
	default:
		return "pass_none"
	}
}
