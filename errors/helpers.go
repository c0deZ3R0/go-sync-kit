package errors

// WrapOpComponent provides a convenience helper to wrap errors with consistent Op and Component propagation.
// It avoids repetition when creating structured errors throughout the codebase.
// If err is nil, returns nil.
func WrapOpComponent(err error, op, component string) error {
	if err == nil {
		return nil
	}
	// Use the builder pattern with Op() and Component() helpers
	return E(Op(op), Component(component), err)
}

// WrapOpComponentKind provides a convenience helper to wrap errors with Op, Component, and Kind.
// If err is nil, returns nil.
func WrapOpComponentKind(err error, op, component string, kind Kind) error {
	if err == nil {
		return nil
	}
	// Use the builder pattern with Op(), Component(), and Kind helpers
	return E(Op(op), Component(component), kind, err)
}
