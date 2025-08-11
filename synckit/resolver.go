package synckit

import (
	"context"
	"errors"
)

// Rule binds a matcher Specification to a ConflictResolver Strategy.
// Rules are evaluated in insertion order with first-match-wins semantics.
type Rule struct {
	Name     string
	Matcher  Spec
	Resolver ConflictResolver
}

// Hooks provides optional callbacks for observability around resolution.
// All hooks are optional; nil functions are safe no-ops.
type Hooks struct {
	OnRuleMatched func(conflict Conflict, rule Rule)
	OnResolved    func(conflict Conflict, result ResolvedConflict)
	OnFallback    func(conflict Conflict)
	OnError       func(conflict Conflict, err error)
}

// Validator can perform configuration validation at construction time.
// Return nil if configuration is valid; return error to prevent construction.
type Validator interface {
	Validate(options *resolverOptions) error
}

// resolverOptions holds construction-time options.
type resolverOptions struct {
	rules    []Rule
	fallback ConflictResolver
	logger   any
	hooks    Hooks
	validator Validator
}

// Option implements the Uber-style functional options pattern for construction.
type Option interface{ apply(*resolverOptions) }

type optionFn func(*resolverOptions)
func (f optionFn) apply(o *resolverOptions) { f(o) }

// WithFallback sets the required fallback ConflictResolver.
func WithFallback(r ConflictResolver) Option { return optionFn(func(o *resolverOptions) { o.fallback = r }) }

// WithRule appends a rule with a custom matcher and resolver in insertion order.
func WithRule(name string, matcher Spec, resolver ConflictResolver) Option {
	return optionFn(func(o *resolverOptions) {
		o.rules = append(o.rules, Rule{Name: name, Matcher: matcher, Resolver: resolver})
	})
}

// WithEventTypeRule is a convenience helper for matching by event type.
func WithEventTypeRule(name string, eventType string, resolver ConflictResolver) Option {
	return WithRule(name, EventTypeIs(eventType), resolver)
}

// WithLogger attaches an optional logger (opaque type to avoid dependency).
func WithLogger(l any) Option { return optionFn(func(o *resolverOptions) { o.logger = l }) }

// WithHooks sets optional observability hooks. Zero-value safe.
func WithHooks(h Hooks) Option { return optionFn(func(o *resolverOptions) { o.hooks = h }) }

// WithValidator sets an optional validator for construction-time checks.
func WithValidator(v Validator) Option { return optionFn(func(o *resolverOptions) { o.validator = v }) }

// DynamicResolver dispatches conflicts to strategies based on an ordered rule set.
// If no rule matches, it uses the fallback resolver. If no fallback is configured,
// Resolve returns an error.
type DynamicResolver struct {
	rules    []Rule
	fallback ConflictResolver
	logger   any
	hooks    Hooks
}

var _ ConflictResolver = (*DynamicResolver)(nil)

// NewDynamicResolver constructs a DynamicResolver with validation.
// Invariants:
// - At least one rule OR a non-nil fallback must be provided
// - No rule may have a nil matcher or resolver
// - If provided, Validator must approve the configuration
func NewDynamicResolver(opts ...Option) (*DynamicResolver, error) {
	cfg := &resolverOptions{}
	for _, opt := range opts { opt.apply(cfg) }

	// Basic validation
	if len(cfg.rules) == 0 && cfg.fallback == nil {
		return nil, errors.New("dynamic resolver requires at least one rule or a non-nil fallback")
	}
	for i, r := range cfg.rules {
		if r.Matcher == nil {
			return nil, errors.New("rule has nil matcher at index "+itoa(i))
		}
		if r.Resolver == nil {
			return nil, errors.New("rule has nil resolver at index "+itoa(i))
		}
	}
	if cfg.validator != nil {
		if err := cfg.validator.Validate(cfg); err != nil { return nil, err }
	}

	return &DynamicResolver{
		rules:    cfg.rules,
		fallback: cfg.fallback,
		logger:   cfg.logger,
		hooks:    cfg.hooks,
	}, nil
}

// Resolve implements the ConflictResolver interface using first-match-wins
// over the ordered rules, else delegates to fallback.
func (d *DynamicResolver) Resolve(ctx context.Context, c Conflict) (ResolvedConflict, error) {
	for _, r := range d.rules {
		if r.Matcher != nil && r.Matcher(c) {
			if d.hooks.OnRuleMatched != nil { d.hooks.OnRuleMatched(c, r) }
			res, err := r.Resolver.Resolve(ctx, c)
			if err != nil {
				if d.hooks.OnError != nil { d.hooks.OnError(c, err) }
				return ResolvedConflict{}, err
			}
			if d.hooks.OnResolved != nil { d.hooks.OnResolved(c, res) }
			return res, nil
		}
	}
	if d.fallback == nil {
		if d.hooks.OnError != nil { d.hooks.OnError(c, errors.New("no rule matched and no fallback configured")) }
		return ResolvedConflict{}, errors.New("no rule matched and no fallback configured")
	}
	if d.hooks.OnFallback != nil { d.hooks.OnFallback(c) }
	res, err := d.fallback.Resolve(ctx, c)
	if err != nil {
		if d.hooks.OnError != nil { d.hooks.OnError(c, err) }
		return ResolvedConflict{}, err
	}
	if d.hooks.OnResolved != nil { d.hooks.OnResolved(c, res) }
	return res, nil
}

// itoa avoids importing strconv for a simple positive int to string conversion.
func itoa(i int) string {
	if i == 0 { return "0" }
	neg := false
	if i < 0 { neg = true; i = -i }
	buf := [20]byte{}
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg { n--; buf[n] = '-' }
	return string(buf[n:])
}
