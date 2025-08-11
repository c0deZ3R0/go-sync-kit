package synckit

import (
	"context"
	"fmt"
)

// RuleGroup implements the Composite pattern for organizing rules hierarchically.
// It allows grouping related rules under a common namespace and provides
// domain modularity without polluting the top-level rule order.
type RuleGroup struct {
	Name        string
	Description string
	rules       []Rule
	subGroups   []*RuleGroup
	enabled     bool
	fallback    ConflictResolver // Optional fallback for this group
}

// NewRuleGroup creates a new rule group with the given name.
func NewRuleGroup(name string, opts ...RuleGroupOption) *RuleGroup {
	rg := &RuleGroup{
		Name:      name,
		enabled:   true,
		rules:     make([]Rule, 0),
		subGroups: make([]*RuleGroup, 0),
	}

	for _, opt := range opts {
		opt.apply(rg)
	}

	return rg
}

// RuleGroupOption provides configuration options for rule groups.
type RuleGroupOption interface {
	apply(*RuleGroup)
}

type ruleGroupOptionFunc func(*RuleGroup)

func (f ruleGroupOptionFunc) apply(rg *RuleGroup) {
	f(rg)
}

// WithDescription sets a description for the rule group.
func WithDescription(desc string) RuleGroupOption {
	return ruleGroupOptionFunc(func(rg *RuleGroup) {
		rg.Description = desc
	})
}

// WithGroupFallback sets a fallback resolver specific to this group.
func WithGroupFallback(resolver ConflictResolver) RuleGroupOption {
	return ruleGroupOptionFunc(func(rg *RuleGroup) {
		rg.fallback = resolver
	})
}

// WithEnabled sets the enabled state of the rule group.
func WithEnabled(enabled bool) RuleGroupOption {
	return ruleGroupOptionFunc(func(rg *RuleGroup) {
		rg.enabled = enabled
	})
}

// AddRule adds a rule to this group.
func (rg *RuleGroup) AddRule(rule Rule) *RuleGroup {
	if !rg.enabled {
		return rg // Skip adding rules if group is disabled
	}
	
	// Namespace the rule name with the group name
	namespacedRule := rule
	if rule.Name != "" {
		namespacedRule.Name = fmt.Sprintf("%s.%s", rg.Name, rule.Name)
	}
	
	rg.rules = append(rg.rules, namespacedRule)
	return rg
}

// AddSubGroup adds a sub-group to this group, allowing nested hierarchies.
func (rg *RuleGroup) AddSubGroup(subGroup *RuleGroup) *RuleGroup {
	if !rg.enabled {
		return rg // Skip adding subgroups if group is disabled
	}
	
	// Store original name
	originalSubGroupName := subGroup.Name
	
	// Namespace the subgroup name
	subGroup.Name = fmt.Sprintf("%s.%s", rg.Name, subGroup.Name)
	
	// Update all rules in the subgroup to use the full namespace
	for i := range subGroup.rules {
		originalRuleName := subGroup.rules[i].Name
		// Remove the old subgroup prefix if it exists
		if originalRuleName != "" {
			expectedPrefix := originalSubGroupName + "."
			if len(originalRuleName) > len(expectedPrefix) && originalRuleName[:len(expectedPrefix)] == expectedPrefix {
				originalRuleName = originalRuleName[len(expectedPrefix):]
			}
			// Apply the new full namespace
			subGroup.rules[i].Name = fmt.Sprintf("%s.%s", subGroup.Name, originalRuleName)
		}
	}
	
	rg.subGroups = append(rg.subGroups, subGroup)
	return rg
}

// Enable/Disable methods for runtime control
func (rg *RuleGroup) Enable() *RuleGroup {
	rg.enabled = true
	return rg
}

func (rg *RuleGroup) Disable() *RuleGroup {
	rg.enabled = false
	return rg
}

func (rg *RuleGroup) IsEnabled() bool {
	return rg.enabled
}

// GetAllRules returns all rules from this group and its subgroups in order.
// This flattens the hierarchy for execution by DynamicResolver.
func (rg *RuleGroup) GetAllRules() []Rule {
	if !rg.enabled {
		return nil
	}
	
	var allRules []Rule
	
	// Add rules from this group first
	allRules = append(allRules, rg.rules...)
	
	// Then add rules from subgroups (depth-first)
	for _, subGroup := range rg.subGroups {
		allRules = append(allRules, subGroup.GetAllRules()...)
	}
	
	return allRules
}

// ResolveInGroup attempts to resolve a conflict using only rules within this group.
// This allows for group-specific resolution before falling back to global rules.
func (rg *RuleGroup) ResolveInGroup(ctx context.Context, conflict Conflict) (ResolvedConflict, bool, error) {
	if !rg.enabled {
		return ResolvedConflict{}, false, nil
	}
	
	// Try rules in this group first
	for _, rule := range rg.rules {
		if rule.Matcher(conflict) {
			resolved, err := rule.Resolver.Resolve(ctx, conflict)
			if err != nil {
				return ResolvedConflict{}, false, err
			}
			return resolved, true, nil
		}
	}
	
	// Try subgroups
	for _, subGroup := range rg.subGroups {
		if resolved, matched, err := subGroup.ResolveInGroup(ctx, conflict); err != nil {
			return ResolvedConflict{}, false, err
		} else if matched {
			return resolved, true, nil
		}
	}
	
	// Try group-specific fallback if available
	if rg.fallback != nil {
		resolved, err := rg.fallback.Resolve(ctx, conflict)
		if err != nil {
			return ResolvedConflict{}, false, err
		}
		return resolved, true, nil
	}
	
	return ResolvedConflict{}, false, nil
}

// GetStats returns statistics about the rule group structure.
func (rg *RuleGroup) GetStats() RuleGroupStats {
	stats := RuleGroupStats{
		Name:         rg.Name,
		Enabled:      rg.enabled,
		RuleCount:    len(rg.rules),
		SubGroupCount: len(rg.subGroups),
		TotalRules:   len(rg.GetAllRules()),
	}
	
	for _, subGroup := range rg.subGroups {
		stats.SubGroups = append(stats.SubGroups, subGroup.GetStats())
	}
	
	return stats
}

// RuleGroupStats provides statistical information about rule group structure.
type RuleGroupStats struct {
	Name          string          `json:"name"`
	Enabled       bool            `json:"enabled"`
	RuleCount     int             `json:"rule_count"`      // Direct rules in this group
	SubGroupCount int             `json:"subgroup_count"`  // Number of direct subgroups
	TotalRules    int             `json:"total_rules"`     // Total rules including subgroups
	SubGroups     []RuleGroupStats `json:"subgroups,omitempty"`
}

// CompositeResolver implements ConflictResolver and acts as a root container
// for multiple rule groups with group-aware resolution.
type CompositeResolver struct {
	groups   []*RuleGroup
	fallback ConflictResolver
	logger   Logger
}

// NewCompositeResolver creates a new composite resolver that manages multiple rule groups.
func NewCompositeResolver(fallback ConflictResolver, opts ...CompositeOption) *CompositeResolver {
	cr := &CompositeResolver{
		groups:   make([]*RuleGroup, 0),
		fallback: fallback,
	}
	
	for _, opt := range opts {
		opt.apply(cr)
	}
	
	return cr
}

// CompositeOption provides configuration options for CompositeResolver.
type CompositeOption interface {
	apply(*CompositeResolver)
}

type compositeOptionFunc func(*CompositeResolver)

func (f compositeOptionFunc) apply(cr *CompositeResolver) {
	f(cr)
}

// WithCompositeLogger sets a logger for the composite resolver.
func WithCompositeLogger(logger Logger) CompositeOption {
	return compositeOptionFunc(func(cr *CompositeResolver) {
		cr.logger = logger
	})
}

// AddGroup adds a rule group to the composite resolver.
func (cr *CompositeResolver) AddGroup(group *RuleGroup) *CompositeResolver {
	cr.groups = append(cr.groups, group)
	return cr
}

// Resolve implements ConflictResolver interface for CompositeResolver.
// It tries each group in order, then falls back to the global fallback.
func (cr *CompositeResolver) Resolve(ctx context.Context, conflict Conflict) (ResolvedConflict, error) {
	// Try each group in order
	for _, group := range cr.groups {
		if resolved, matched, err := group.ResolveInGroup(ctx, conflict); err != nil {
			if cr.logger != nil {
				cr.logger.Error("Group resolution error", "group", group.Name, "error", err)
			}
			return ResolvedConflict{}, err
		} else if matched {
			if cr.logger != nil {
				cr.logger.Debug("Conflict resolved by group", "group", group.Name, "conflict", conflict.AggregateID)
			}
			return resolved, nil
		}
	}
	
	// Fall back to global fallback
	if cr.fallback != nil {
		if cr.logger != nil {
			cr.logger.Debug("Using global fallback for conflict", "conflict", conflict.AggregateID)
		}
		return cr.fallback.Resolve(ctx, conflict)
	}
	
	return ResolvedConflict{}, fmt.Errorf("no rule groups matched conflict and no fallback provided")
}

// GetAllGroups returns all rule groups managed by this composite resolver.
func (cr *CompositeResolver) GetAllGroups() []*RuleGroup {
	return append([]*RuleGroup(nil), cr.groups...)
}

// GetGroupStats returns statistics for all managed rule groups.
func (cr *CompositeResolver) GetGroupStats() []RuleGroupStats {
	stats := make([]RuleGroupStats, len(cr.groups))
	for i, group := range cr.groups {
		stats[i] = group.GetStats()
	}
	return stats
}

// Logger interface for composite resolver logging.
type Logger interface {
	Debug(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}
