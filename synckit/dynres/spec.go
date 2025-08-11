package dynres

// Spec is a predicate used to match conflicts to rules. Combinators allow
// building complex match logic from small, testable pieces.
type Spec func(Conflict) bool

// And returns a spec that requires both specs to match.
func And(a, b Spec) Spec { return func(c Conflict) bool { return a != nil && b != nil && a(c) && b(c) } }

// Or returns a spec that requires at least one spec to match.
func Or(a, b Spec) Spec { return func(c Conflict) bool { return (a != nil && a(c)) || (b != nil && b(c)) } }

// Not returns a spec that negates the provided spec.
func Not(a Spec) Spec { return func(c Conflict) bool { return a == nil || !a(c) } }

// EventTypeIs matches a specific event type.
func EventTypeIs(t string) Spec {
	return func(c Conflict) bool { return c.EventType == t }
}

// AnyFieldIn matches when any of the conflict's changed fields is in the set.
func AnyFieldIn(fields ...string) Spec {
	set := map[string]struct{}{}
	for _, f := range fields { set[f] = struct{}{} }
	return func(c Conflict) bool {
		for _, f := range c.ChangedFields {
			if _, ok := set[f]; ok { return true }
		}
		return false
	}
}

// MetadataEq matches when Metadata[key] equals value.
func MetadataEq(key string, value any) Spec {
	return func(c Conflict) bool {
		if c.Metadata == nil { return false }
		v, ok := c.Metadata[key]
		if !ok { return false }
		return anyEqual(v, value)
	}
}

// anyEqual provides a basic equality check for common comparable types.
func anyEqual(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string); return ok && av == bv
	case int:
		bv, ok := b.(int); return ok && av == bv
	case int64:
		bv, ok := b.(int64); return ok && av == bv
	case uint64:
		bv, ok := b.(uint64); return ok && av == bv
	case bool:
		bv, ok := b.(bool); return ok && av == bv
	default:
		return a == b
	}
}

