# Synckit Dynamic Resolver Showcase 🚀

A practical demonstration of go-sync-kit's dynamic conflict resolution system, featuring rule-based conflict resolution with custom business logic.

## Overview

This interactive showcase demonstrates the synckit dynamic resolver with:

- **Custom Business Rules**: Time-based and priority-based conflict resolution
- **Multiple Resolution Strategies**: Business hours awareness, document priority handling
- **Real-world Scenarios**: Document editing, incident management, manual review workflows
- **Interactive Experience**: Colorful terminal output with step-by-step demonstrations
- **Working Example**: Fully functional code that shows the resolver in action

## Features Demonstrated

### 🌟 Core Features

1. **Dynamic Rule Matching**
   - Context-aware rule selection
   - Priority-based rule ordering
   - Conditional rule application

2. **Sophisticated Resolution Strategies**
   - Last-write-wins with intelligent merging
   - Role-based permission resolution
   - Time-window and business-hours logic
   - Field-specific resolution per data type

3. **Rich Conflict Metadata**
   - Detailed conflict context
   - Resolution confidence scoring
   - Audit trail and reasoning

### 📋 Demo Scenarios

#### 1. Document Version Conflicts
- **Concurrent editing** by multiple users
- **Locked document** preference logic
- **Content similarity** detection and merging
- **Collaborative** tag and author management

#### 2. User Permission Conflicts
- **Role hierarchy** resolution (admin > manager > editor > viewer)
- **Permission merging** for same-level roles
- **Account activation** vs. permission grants
- **Project-specific** role assignments

#### 3. Project Management Conflicts
- **Priority-based** resolution (critical > high > medium > low)
- **Team lead** changes and authority conflicts
- **Budget** and resource allocation conflicts
- **Member** and permission management

#### 4. Time-Based Resolution
- **Business hours** vs. after-hours preferences
- **Emergency** incident response priorities
- **Weekend work** vs. business day reviews
- **Timezone-aware** conflict resolution

#### 5. Field-Specific Resolution
- **Per-field strategies**: Different rules for different data fields
- **User preferences** vs. administrative overrides
- **Additive merging** for collections (tags, permissions)
- **Smart content merging** for text fields

#### 6. Interactive Conflict Builder
- **Custom conflict creation** with guided input
- **Real-time resolution** demonstration
- **Educational** rule matching explanation

## Quick Start

### Prerequisites

- Go 1.21 or later
- Terminal with color support (recommended)

### Running the Showcase

1. **Navigate to the showcase directory:**
   ```bash
   cd examples/dynamic-resolver-showcase
   ```

2. **Run the simple showcase:**
   ```bash
   go run simple_showcase.go
   ```

3. **Follow the interactive prompts** - press Enter between scenarios to see each demo

### Demo Scenarios

The showcase demonstrates three key scenarios:

1. **📄 Document Conflict - Normal Priority**
   - Two users editing a normal document simultaneously
   - Resolved using Last-Write-Wins strategy
   - Shows the fallback resolver in action

2. **⚡ High-Priority Document Conflict**
   - Conflict in a high-priority document
   - Triggers manual review requirement
   - Demonstrates rule-based conditional resolution

3. **⏰ Time-Sensitive Business Hours Conflict**
   - Incident report conflict with time-sensitive metadata
   - Resolves based on business hours vs. offline work preference
   - Shows custom business logic in action

## Architecture

### Project Structure

```
dynamic-resolver-showcase/
├── main.go                 # Main interactive CLI application
├── entities/
│   └── types.go           # Data models (Document, User, Project, Task)
├── resolvers/
│   └── rules.go           # Custom resolution rules and strategies
├── scenarios/
│   └── generators.go      # Conflict scenario generators
├── config/
│   └── demo_config.yaml   # Configuration file
└── README.md              # This file
```

### Key Components

#### Dynamic Resolver Rules

Each rule consists of:
- **Name & Description**: Human-readable identification
- **Priority**: Determines rule application order (higher = first)
- **Condition**: Function that determines if rule applies to a conflict
- **Resolver**: Implementation that resolves matching conflicts

#### Resolution Strategies

- **DocumentVersionResolver**: Handles document editing conflicts with lock awareness
- **UserPermissionResolver**: Manages role hierarchy and permission conflicts
- **TimeBasedResolver**: Applies business hours and time window logic
- **FieldSpecificResolver**: Uses different strategies per data field
- **ProjectPriorityResolver**: Resolves based on project/task priority levels

#### Scenario Generators

Each generator creates realistic conflict scenarios:
- **DocumentConflictScenario**: Multi-user editing scenarios
- **UserPermissionScenario**: Role and permission conflicts
- **ProjectManagementScenario**: Project and task management conflicts
- **TimeBasedScenario**: Time-sensitive business rule conflicts
- **FieldSpecificScenario**: Complex field-level resolution scenarios

## Configuration

The showcase can be configured via `config/demo_config.yaml`:

- **Business Rules**: Hours, role hierarchy, priority levels
- **Resolution Strategies**: Priorities, confidence scores, descriptions
- **Demo Scenarios**: Which scenarios to enable and their parameters
- **Display Options**: Colors, interactivity, formatting
- **Performance**: Timeouts, limits, optimization settings

## Educational Value

This showcase demonstrates:

1. **Real-world complexity** of conflict resolution in collaborative systems
2. **Rule-based architecture** for maintainable and extensible conflict resolution
3. **Business logic integration** with technical conflict resolution
4. **Multi-dimensional conflicts** that require sophisticated resolution strategies
5. **Confidence scoring** and audit trails for resolution decisions
6. **Interactive exploration** of conflict resolution behavior

## Extending the Showcase

### Adding New Entity Types

1. Define your entity in `entities/types.go`
2. Create extraction helper in `resolvers/rules.go`
3. Add resolution logic to existing or new resolvers
4. Create scenario generator in `scenarios/generators.go`
5. Add menu option in `main.go`

### Adding New Resolution Rules

1. Implement the rule condition function
2. Create a resolver that implements the `conflict.Resolver` interface
3. Add the rule to the dynamic resolver chain in `main.go`
4. Test with relevant scenarios

### Adding New Scenarios

1. Create a new scenario generator type
2. Implement the `ScenarioGenerator` interface
3. Add rich, realistic conflict data
4. Include appropriate metadata for rule matching
5. Add to the demo menu

## Performance Considerations

- **Rule Ordering**: Higher priority rules are evaluated first
- **Early Termination**: First matching rule wins (no rule chaining)
- **Confidence Scoring**: Helps choose between competing resolutions
- **Metadata Efficiency**: Rich context without performance overhead
- **Memory Management**: Scenarios generate fresh data each time

## Troubleshooting

### Common Issues

1. **Import Path Errors**: Update import statements to match your module path
2. **Terminal Colors**: Disable colors in config if terminal doesn't support them
3. **Timeout Issues**: Increase timeouts in config for slower systems
4. **Memory Usage**: Reduce max conflicts per demo if memory is limited

### Debug Mode

Enable verbose output in `config/demo_config.yaml`:

```yaml
debugging:
  verbose_resolution: true
  show_rule_matching: true
  show_confidence_calculation: true
```

## Contributing

This showcase serves as both a demonstration and a template for building sophisticated conflict resolution systems. Feel free to:

- Add new scenarios that reflect your specific use cases
- Implement additional resolution strategies
- Enhance the interactive experience
- Improve the configuration system
- Add metrics and monitoring capabilities

## License

This showcase is part of the go-sync-kit project and follows the same license terms.

---

*Happy conflict resolving! 🎯*
