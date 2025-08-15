package synckit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigLoader provides dynamic loading and validation of conflict resolution rules
// from YAML or JSON configuration files with runtime update capabilities.
type ConfigLoader struct {
	mu              sync.RWMutex
	currentConfig   *RuleConfig
	validators      []ConfigValidator
	watchers        []ConfigWatcher
	transformers    []ConfigTransformer
	logger          Logger
}

// RuleConfig represents the complete configuration structure for conflict resolution rules.
type RuleConfig struct {
	Version     string                 `json:"version" yaml:"version"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	
	// Global settings
	Settings ConfigSettings `json:"settings,omitempty" yaml:"settings,omitempty"`
	
	// Rule groups
	Groups []GroupConfig `json:"groups" yaml:"groups"`
	
	// Global rules (not in any group)
	Rules []RuleConfigEntry `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// ConfigSettings contains global configuration settings.
type ConfigSettings struct {
	DefaultFallback string            `json:"default_fallback,omitempty" yaml:"default_fallback,omitempty"`
	Timeouts        TimeoutSettings   `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
	Limits          LimitSettings     `json:"limits,omitempty" yaml:"limits,omitempty"`
	Features        map[string]bool   `json:"features,omitempty" yaml:"features,omitempty"`
	Custom          map[string]string `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// TimeoutSettings contains timeout configurations.
type TimeoutSettings struct {
	ResolutionTimeoutMs int `json:"resolution_timeout_ms,omitempty" yaml:"resolution_timeout_ms,omitempty"`
	ManualReviewTimeoutMs int `json:"manual_review_timeout_ms,omitempty" yaml:"manual_review_timeout_ms,omitempty"`
}

// LimitSettings contains various limits.
type LimitSettings struct {
	MaxConflictsPerBatch int `json:"max_conflicts_per_batch,omitempty" yaml:"max_conflicts_per_batch,omitempty"`
	MaxRulesPerGroup     int `json:"max_rules_per_group,omitempty" yaml:"max_rules_per_group,omitempty"`
}

// GroupConfig represents a rule group configuration.
type GroupConfig struct {
	Name        string              `json:"name" yaml:"name"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Fallback    string              `json:"fallback,omitempty" yaml:"fallback,omitempty"`
	Rules       []RuleConfigEntry   `json:"rules" yaml:"rules"`
	SubGroups   []GroupConfig       `json:"subgroups,omitempty" yaml:"subgroups,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// RuleConfigEntry represents a single rule configuration.
type RuleConfigEntry struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Priority    int                    `json:"priority,omitempty" yaml:"priority,omitempty"`
	
	// Matching conditions
	Conditions MatchConditions `json:"conditions" yaml:"conditions"`
	
	// Resolution configuration
	Resolution ResolutionConfig `json:"resolution" yaml:"resolution"`
	
	// Metadata for custom extensions
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// MatchConditions defines when a rule should be applied.
type MatchConditions struct {
	EventTypes    []string          `json:"event_types,omitempty" yaml:"event_types,omitempty"`
	AggregateIDs  []string          `json:"aggregate_ids,omitempty" yaml:"aggregate_ids,omitempty"`
	Fields        []string          `json:"fields,omitempty" yaml:"fields,omitempty"`
	CustomMatch   map[string]string `json:"custom_match,omitempty" yaml:"custom_match,omitempty"`
}

// ResolutionConfig defines how conflicts should be resolved.
type ResolutionConfig struct {
	Strategy string                 `json:"strategy" yaml:"strategy"`
	Options  map[string]interface{} `json:"options,omitempty" yaml:"options,omitempty"`
}

// ConfigValidator validates configuration before applying it.
type ConfigValidator interface {
	Validate(config *RuleConfig) error
	Name() string
}

// ConfigWatcher monitors configuration changes.
type ConfigWatcher interface {
	OnConfigChanged(oldConfig, newConfig *RuleConfig)
	OnConfigError(err error)
	Name() string
}

// ConfigTransformer allows modification of configuration during loading.
type ConfigTransformer interface {
	Transform(config *RuleConfig) (*RuleConfig, error)
	Name() string
}

// NewConfigLoader creates a new configuration loader.
func NewConfigLoader(opts ...ConfigLoaderOption) *ConfigLoader {
	cl := &ConfigLoader{
		validators:   make([]ConfigValidator, 0),
		watchers:     make([]ConfigWatcher, 0),
		transformers: make([]ConfigTransformer, 0),
	}
	
	for _, opt := range opts {
		opt.apply(cl)
	}
	
	return cl
}

// ConfigLoaderOption provides configuration options for ConfigLoader.
type ConfigLoaderOption interface {
	apply(*ConfigLoader)
}

type configLoaderOptionFunc func(*ConfigLoader)

func (f configLoaderOptionFunc) apply(cl *ConfigLoader) {
	f(cl)
}

// WithConfigValidator adds a configuration validator.
func WithConfigValidator(validator ConfigValidator) ConfigLoaderOption {
	return configLoaderOptionFunc(func(cl *ConfigLoader) {
		cl.validators = append(cl.validators, validator)
	})
}

// WithWatcher adds a configuration change watcher.
func WithWatcher(watcher ConfigWatcher) ConfigLoaderOption {
	return configLoaderOptionFunc(func(cl *ConfigLoader) {
		cl.watchers = append(cl.watchers, watcher)
	})
}

// WithTransformer adds a configuration transformer.
func WithTransformer(transformer ConfigTransformer) ConfigLoaderOption {
	return configLoaderOptionFunc(func(cl *ConfigLoader) {
		cl.transformers = append(cl.transformers, transformer)
	})
}

// WithConfigLogger sets a logger for the config loader.
func WithConfigLogger(logger Logger) ConfigLoaderOption {
	return configLoaderOptionFunc(func(cl *ConfigLoader) {
		cl.logger = logger
	})
}

// LoadFromFile loads configuration from a YAML or JSON file.
func (cl *ConfigLoader) LoadFromFile(filepath string) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	if cl.logger != nil {
		cl.logger.Debug("Loading configuration from file", "path", filepath)
	}
	
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %w", filepath, err)
	}
	defer file.Close()
	
	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filepath, err)
	}
	
	return cl.loadFromBytes(data, detectFormat(filepath))
}

// LoadFromBytes loads configuration from raw bytes.
func (cl *ConfigLoader) LoadFromBytes(data []byte, format string) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	return cl.loadFromBytes(data, format)
}

func (cl *ConfigLoader) loadFromBytes(data []byte, format string) error {
	var config RuleConfig
	
	switch strings.ToLower(format) {
	case "yaml", "yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case "json":
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config format: %s", format)
	}
	
	return cl.applyConfig(&config)
}

// applyConfig validates and applies a configuration.
func (cl *ConfigLoader) applyConfig(config *RuleConfig) error {
	// Apply transformers
	for _, transformer := range cl.transformers {
		transformed, err := transformer.Transform(config)
		if err != nil {
			if cl.logger != nil {
				cl.logger.Error("Configuration transformation failed", "transformer", transformer.Name(), "error", err)
			}
			return fmt.Errorf("transformer %s failed: %w", transformer.Name(), err)
		}
		config = transformed
	}
	
	// Validate configuration
	for _, validator := range cl.validators {
		if err := validator.Validate(config); err != nil {
			if cl.logger != nil {
				cl.logger.Error("Configuration validation failed", "validator", validator.Name(), "error", err)
			}
			return fmt.Errorf("validator %s failed: %w", validator.Name(), err)
		}
	}
	
	oldConfig := cl.currentConfig
	cl.currentConfig = config
	
	// Notify watchers
	for _, watcher := range cl.watchers {
		go func(w ConfigWatcher) {
			defer func() {
				if r := recover(); r != nil {
					if cl.logger != nil {
						cl.logger.Error("Config watcher panic", "watcher", w.Name(), "panic", r)
					}
				}
			}()
			w.OnConfigChanged(oldConfig, config)
		}(watcher)
	}
	
	if cl.logger != nil {
		cl.logger.Debug("Configuration applied successfully", "version", config.Version, "groups", len(config.Groups))
	}
	
	return nil
}

// GetCurrentConfig returns the current configuration.
func (cl *ConfigLoader) GetCurrentConfig() *RuleConfig {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	
	return cl.currentConfig
}

// BuildDynamicResolver creates a DynamicResolver from the current configuration.
func (cl *ConfigLoader) BuildDynamicResolver() (*DynamicResolver, error) {
	cl.mu.RLock()
	config := cl.currentConfig
	cl.mu.RUnlock()
	
	if config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}
	
	var options []Option
	
	// Set global fallback if specified
	if config.Settings.DefaultFallback != "" {
		fallback, err := cl.createResolverFromStrategy(config.Settings.DefaultFallback, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create default fallback: %w", err)
		}
		options = append(options, WithFallback(fallback))
	}
	
	// Add global rules
	for _, ruleConfig := range config.Rules {
		rule, err := cl.buildRule(ruleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build global rule %s: %w", ruleConfig.Name, err)
		}
		options = append(options, WithRule(rule.Name, rule.Matcher, rule.Resolver))
	}
	
	// Note: Group support would require CompositeResolver integration
	// For now, we'll add group rules as individual rules with namespaced names
	for _, groupConfig := range config.Groups {
		for _, ruleConfig := range groupConfig.Rules {
			rule, err := cl.buildRule(ruleConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to build rule %s in group %s: %w", ruleConfig.Name, groupConfig.Name, err)
			}
			// Namespace the rule name with the group
			namespacedName := fmt.Sprintf("%s.%s", groupConfig.Name, rule.Name)
			options = append(options, WithRule(namespacedName, rule.Matcher, rule.Resolver))
		}
	}
	
	return NewDynamicResolver(options...)
}

// buildRuleGroup creates a RuleGroup from configuration.
func (cl *ConfigLoader) buildRuleGroup(config GroupConfig) (*RuleGroup, error) {
	opts := make([]RuleGroupOption, 0)
	
	if config.Description != "" {
		opts = append(opts, WithDescription(config.Description))
	}
	
	enabled := true
	if config.Enabled != nil {
		enabled = *config.Enabled
	}
	opts = append(opts, WithEnabled(enabled))
	
	if config.Fallback != "" {
		fallback, err := cl.createResolverFromStrategy(config.Fallback, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create group fallback: %w", err)
		}
		opts = append(opts, WithGroupFallback(fallback))
	}
	
	group := NewRuleGroup(config.Name, opts...)
	
	// Add rules to group
	for _, ruleConfig := range config.Rules {
		rule, err := cl.buildRule(ruleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build rule %s in group %s: %w", ruleConfig.Name, config.Name, err)
		}
		group.AddRule(rule)
	}
	
	// Add subgroups
	for _, subGroupConfig := range config.SubGroups {
		subGroup, err := cl.buildRuleGroup(subGroupConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build subgroup %s: %w", subGroupConfig.Name, err)
		}
		group.AddSubGroup(subGroup)
	}
	
	return group, nil
}

// buildRule creates a Rule from configuration.
func (cl *ConfigLoader) buildRule(config RuleConfigEntry) (Rule, error) {
	// Build matcher
	matcher, err := cl.buildMatcher(config.Conditions)
	if err != nil {
		return Rule{}, fmt.Errorf("failed to build matcher: %w", err)
	}
	
	// Build resolver
	resolver, err := cl.createResolverFromStrategy(config.Resolution.Strategy, config.Resolution.Options)
	if err != nil {
		return Rule{}, fmt.Errorf("failed to create resolver: %w", err)
	}
	
	rule := Rule{
		Name:     config.Name,
		Matcher:  matcher,
		Resolver: resolver,
	}
	
	return rule, nil
}

// buildMatcher creates a Spec from conditions.
func (cl *ConfigLoader) buildMatcher(conditions MatchConditions) (Spec, error) {
	var matchers []Spec
	
	// Event type matchers
	for _, eventType := range conditions.EventTypes {
		matchers = append(matchers, EventTypeIs(eventType))
	}
	
	// Field matchers
	for _, field := range conditions.Fields {
		matchers = append(matchers, FieldChanged(field))
	}
	
	// Aggregate ID matchers
	for _, aggID := range conditions.AggregateIDs {
		matchers = append(matchers, AggregateIDMatches(aggID))
	}
	
	// Combine matchers
	if len(matchers) == 0 {
		return AlwaysMatch(), nil
	} else if len(matchers) == 1 {
		return matchers[0], nil
	}
	
	// Use AND logic for multiple conditions
	return AndMatcher(matchers...), nil
}

// createResolverFromStrategy creates a ConflictResolver from strategy name and options.
func (cl *ConfigLoader) createResolverFromStrategy(strategy string, options map[string]interface{}) (ConflictResolver, error) {
	switch strings.ToLower(strategy) {
	case "last_write_wins", "lww":
		return &LastWriteWinsResolver{}, nil
	case "first_write_wins", "fww":
		return &FirstWriteWinsResolver{}, nil
	case "additive_merge", "merge":
		return &AdditiveMergeResolver{}, nil
	case "manual_review", "manual":
		return &ManualReviewResolver{}, nil
	default:
		return nil, fmt.Errorf("unknown resolution strategy: %s", strategy)
	}
}

// detectFormat determines file format from extension.
func detectFormat(filepath string) string {
	ext := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])
	switch ext {
	case "yml", "yaml":
		return "yaml"
	case "json":
		return "json"
	default:
		return "yaml" // default
	}
}

// Built-in validators

// BasicValidator provides basic configuration validation.
type BasicValidator struct{}

func (v *BasicValidator) Name() string {
	return "basic"
}

func (v *BasicValidator) Validate(config *RuleConfig) error {
	if config.Version == "" {
		return fmt.Errorf("configuration version is required")
	}
	
	if config.Name == "" {
		return fmt.Errorf("configuration name is required")
	}
	
	// Validate groups
	groupNames := make(map[string]bool)
	for _, group := range config.Groups {
		if group.Name == "" {
			return fmt.Errorf("group name is required")
		}
		if groupNames[group.Name] {
			return fmt.Errorf("duplicate group name: %s", group.Name)
		}
		groupNames[group.Name] = true
		
		if err := v.validateRules(group.Rules, group.Name); err != nil {
			return fmt.Errorf("invalid rules in group %s: %w", group.Name, err)
		}
	}
	
	// Validate global rules
	if err := v.validateRules(config.Rules, "global"); err != nil {
		return fmt.Errorf("invalid global rules: %w", err)
	}
	
	return nil
}

func (v *BasicValidator) validateRules(rules []RuleConfigEntry, context string) error {
	ruleNames := make(map[string]bool)
	for _, rule := range rules {
		if rule.Name == "" {
			return fmt.Errorf("rule name is required in %s", context)
		}
		if ruleNames[rule.Name] {
			return fmt.Errorf("duplicate rule name: %s in %s", rule.Name, context)
		}
		ruleNames[rule.Name] = true
		
		if rule.Resolution.Strategy == "" {
			return fmt.Errorf("resolution strategy is required for rule %s in %s", rule.Name, context)
		}
	}
	return nil
}

// Built-in watchers

// LoggingWatcher logs configuration changes.
type LoggingWatcher struct {
	logger Logger
}

func NewLoggingWatcher(logger Logger) *LoggingWatcher {
	return &LoggingWatcher{logger: logger}
}

func (w *LoggingWatcher) Name() string {
	return "logging"
}

func (w *LoggingWatcher) OnConfigChanged(oldConfig, newConfig *RuleConfig) {
	if w.logger == nil {
		return
	}
	
	if oldConfig == nil {
		w.logger.Debug("Initial configuration loaded", "version", newConfig.Version, "groups", len(newConfig.Groups))
	} else {
		w.logger.Debug("Configuration updated", 
			"old_version", oldConfig.Version,
			"new_version", newConfig.Version,
			"old_groups", len(oldConfig.Groups),
			"new_groups", len(newConfig.Groups))
	}
}

func (w *LoggingWatcher) OnConfigError(err error) {
	if w.logger != nil {
		w.logger.Error("Configuration error", "error", err)
	}
}
