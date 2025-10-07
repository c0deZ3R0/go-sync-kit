package synckit

import (
	"testing"
)

func TestConfigLoader_BasicFunctionality(t *testing.T) {
	// Test YAML config
	yamlConfig := `
version: "1.0"
name: "test-config"
description: "Test configuration"

settings:
  default_fallback: "last_write_wins"

rules:
  - name: "user-events"
    conditions:
      event_types: ["UserCreated", "UserUpdated"]
    resolution:
      strategy: "last_write_wins"

  - name: "field-specific"
    conditions:
      fields: ["email", "name"]
    resolution:
      strategy: "first_write_wins"

groups:
  - name: "priority-group"
    description: "High priority rules"
    rules:
      - name: "urgent-updates"
        conditions:
          event_types: ["UrgentUpdate"]
        resolution:
          strategy: "manual_review"
`

	loader := NewConfigLoader(
		WithConfigValidator(&BasicValidator{}),
	)

	// Test loading from bytes
	err := loader.LoadFromBytes([]byte(yamlConfig), "yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config was loaded
	config := loader.GetCurrentConfig()
	if config == nil {
		t.Fatal("Config is nil after loading")
	}

	if config.Version != "1.0" {
		t.Fatalf("Expected version 1.0, got %s", config.Version)
	}

	if config.Name != "test-config" {
		t.Fatalf("Expected name 'test-config', got %s", config.Name)
	}

	if len(config.Rules) != 2 {
		t.Fatalf("Expected 2 global rules, got %d", len(config.Rules))
	}

	if len(config.Groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(config.Groups))
	}

	// Test building dynamic resolver
	resolver, err := loader.BuildDynamicResolver()
	if err != nil {
		t.Fatalf("Failed to build dynamic resolver: %v", err)
	}

	if resolver == nil {
		t.Fatal("Built resolver is nil")
	}
}

func TestConfigLoader_InvalidConfig(t *testing.T) {
	invalidConfig := `
version: ""
name: ""
`

	loader := NewConfigLoader(
		WithConfigValidator(&BasicValidator{}),
	)

	err := loader.LoadFromBytes([]byte(invalidConfig), "yaml")
	if err == nil {
		t.Fatal("Expected error for invalid config")
	}
}

func TestConfigLoader_BuildMatchersAndResolvers(t *testing.T) {
	loader := NewConfigLoader()

	// Test building matchers
	conditions := MatchConditions{
		EventTypes:   []string{"UserCreated"},
		Fields:       []string{"email"},
		AggregateIDs: []string{"user-123"},
	}

	matcher, err := loader.buildMatcher(conditions)
	if err != nil {
		t.Fatalf("Failed to build matcher: %v", err)
	}

	// Test the matcher function
	conflict := Conflict{
		EventType:     "UserCreated",
		AggregateID:   "user-123",
		ChangedFields: []string{"email", "name"},
	}

	if !matcher(conflict) {
		t.Fatal("Matcher should match the conflict")
	}

	// Test with non-matching conflict
	nonMatchingConflict := Conflict{
		EventType:     "UserDeleted", // Different event type
		AggregateID:   "user-123",
		ChangedFields: []string{"email"},
	}

	if matcher(nonMatchingConflict) {
		t.Fatal("Matcher should not match non-matching conflict")
	}

	// Test building resolver
	resolver, err := loader.createResolverFromStrategy("last_write_wins", nil)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}

	if resolver == nil {
		t.Fatal("Created resolver is nil")
	}
}

func TestConfigLoader_JSONSupport(t *testing.T) {
	jsonConfig := `{
		"version": "1.0",
		"name": "json-test-config",
		"settings": {
			"default_fallback": "additive_merge"
		},
		"rules": [
			{
				"name": "json-rule",
				"conditions": {
					"event_types": ["JsonEvent"]
				},
				"resolution": {
					"strategy": "additive_merge"
				}
			}
		]
	}`

	loader := NewConfigLoader()

	err := loader.LoadFromBytes([]byte(jsonConfig), "json")
	if err != nil {
		t.Fatalf("Failed to load JSON config: %v", err)
	}

	config := loader.GetCurrentConfig()
	if config.Name != "json-test-config" {
		t.Fatalf("Expected name 'json-test-config', got %s", config.Name)
	}
}

// TestConfigLoader_BuildRuleGroup tests the buildRuleGroup method.
// This method is reserved for future CompositeResolver integration.
func TestConfigLoader_BuildRuleGroup(t *testing.T) {
	loader := NewConfigLoader()

	// Create a test group configuration
	enabled := true
	groupConfig := GroupConfig{
		Name:        "test-group",
		Description: "A test rule group",
		Enabled:     &enabled,
		Fallback:    "last_write_wins",
		Rules: []RuleConfigEntry{
			{
				Name: "test-rule-1",
				Conditions: MatchConditions{
					EventTypes: []string{"TestEvent1"},
				},
				Resolution: ResolutionConfig{
					Strategy: "first_write_wins",
				},
			},
			{
				Name: "test-rule-2",
				Conditions: MatchConditions{
					Fields: []string{"testField"},
				},
				Resolution: ResolutionConfig{
					Strategy: "last_write_wins",
				},
			},
		},
		SubGroups: []GroupConfig{
			{
				Name:        "test-subgroup",
				Description: "A test subgroup",
				Rules: []RuleConfigEntry{
					{
						Name: "subgroup-rule",
						Conditions: MatchConditions{
							EventTypes: []string{"SubgroupEvent"},
						},
						Resolution: ResolutionConfig{
							Strategy: "additive_merge",
						},
					},
				},
			},
		},
	}

	// Build the rule group
	group, err := loader.buildRuleGroup(groupConfig)
	if err != nil {
		t.Fatalf("Failed to build rule group: %v", err)
	}

	// Verify the group was built correctly
	if group == nil {
		t.Fatal("Built group is nil")
	}

	if group.Name != "test-group" {
		t.Fatalf("Expected group name 'test-group', got %s", group.Name)
	}

	if group.Description != "A test rule group" {
		t.Fatalf("Expected description 'A test rule group', got %s", group.Description)
	}

	if !group.IsEnabled() {
		t.Fatal("Expected group to be enabled")
	}

	// Get stats to verify rule and subgroup counts
	stats := group.GetStats()
	if stats.RuleCount != 2 {
		t.Fatalf("Expected 2 rules in group, got %d", stats.RuleCount)
	}

	if stats.SubGroupCount != 1 {
		t.Fatalf("Expected 1 subgroup, got %d", stats.SubGroupCount)
	}

	// Verify the subgroup stats
	if len(stats.SubGroups) != 1 {
		t.Fatalf("Expected 1 subgroup in stats, got %d", len(stats.SubGroups))
	}

	subGroupStats := stats.SubGroups[0]
	if subGroupStats.Name != "test-group.test-subgroup" {
		t.Fatalf("Expected subgroup name 'test-group.test-subgroup', got %s", subGroupStats.Name)
	}

	if subGroupStats.RuleCount != 1 {
		t.Fatalf("Expected 1 rule in subgroup, got %d", subGroupStats.RuleCount)
	}

	// Verify GetAllRules returns flattened rules
	allRules := group.GetAllRules()
	if len(allRules) != 3 { // 2 from main group + 1 from subgroup
		t.Fatalf("Expected 3 total rules (including subgroup), got %d", len(allRules))
	}
}
