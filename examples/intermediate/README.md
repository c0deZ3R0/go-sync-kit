# Go Sync Kit - Intermediate Examples

This directory contains comprehensive intermediate examples demonstrating advanced features of the Go Sync Kit synchronization library.

## Example Overview

### Core Conflict Resolution (Examples 3-4)
- **03-events-and-storage**: Event storage patterns and data persistence strategies
- **04-conflict-resolution**: Basic conflict resolution strategies (LWW, FWW, Additive Merge, Custom)

### Real-time & Advanced Features (Examples 5-7)
- **05-realtime-autosync**: Automatic synchronization with background processing
- **06-custom-events-filters**: Custom event filtering and processing pipelines
- **07-structured-logging**: Comprehensive logging and debugging capabilities

### NEW: State Machine Integration (Examples 8-9)

#### **08-stateful-resolvers** - Advanced State Machine Integration
Demonstrates the powerful new stateful conflict resolution system:

**Key Features:**
- **State Machine Integration**: Real-time resolver state transitions
- **Performance Monitoring**: Automatic metrics collection and analysis  
- **Dynamic Rule-Based Resolution**: Advanced rule systems with:
  - Event type matching
  - Metadata conditions
  - Field-based rules
  - Combinatorial logic (AND/OR/NOT)
- **Workflow Tracking**: Complete audit trails and workflow management
- **Custom Observability Hooks**: Integration with monitoring systems

**Business Logic Examples:**
- Priority-based resolution (admin > premium > standard accounts)
- Administrative override patterns
- Permission-based conflict resolution
- Time-based resolution strategies

**Real-world Applications:**
- User account management systems
- Financial transaction processing
- Document collaboration platforms
- Multi-tenant application data sync

#### **09-advanced-observability** - Production Monitoring
Comprehensive monitoring and observability for production systems:

**Monitoring Capabilities:**
- **System Metrics**: CPU, memory, goroutine tracking
- **Application Metrics**: Sync operations, throughput, latency
- **Business Metrics**: Conflict resolution rates and success patterns
- **Real-time Dashboards**: Live operational status displays

**Operational Features:**
- **Alerting System**: Threshold-based and pattern-based alerts
- **Health Checks**: Database connectivity and resource monitoring
- **Performance Analysis**: Automated performance insights
- **Error Tracking**: Comprehensive error pattern analysis

**Integration Patterns:**
- Custom metrics collection
- External monitoring system integration
- Production-ready operational insights
- Resource utilization optimization

## New Features Showcased

### State Machine Integration
The new examples demonstrate the advanced state machine capabilities:
- **Conflict Resolution States**: Idle → Evaluating → Resolving → Completed
- **Performance Tracking**: Resolution times, success rates, rule evaluation metrics  
- **Workflow Management**: Active workflow tracking and completion status
- **Rule Evaluation**: Real-time rule matching and execution metrics

### Advanced Resolution Strategies
Beyond basic LWW/FWW patterns:
- **Priority-Based Resolution**: Business rule hierarchies
- **Administrative Overrides**: Authority-based conflict resolution
- **Multi-criteria Decision Making**: Complex resolution logic
- **Audit Trail Generation**: Complete resolution history

### Production-Ready Monitoring
Enterprise-grade observability:
- **Multi-dimensional Metrics**: System, application, and business metrics
- **Real-time Analysis**: Live performance analysis and insights
- **Predictive Alerting**: Proactive issue detection
- **Integration Readiness**: Compatible with existing monitoring infrastructure

## Running the Examples

Each example is self-contained with its own README and can be run independently:

```bash
# Run the stateful resolvers example
cd 08-stateful-resolvers
go mod init example-8
go mod edit -replace github.com/c0deZ3R0/go-sync-kit=../../..
go mod tidy
go run main.go

# Run the advanced observability example  
cd 09-advanced-observability
go mod init example-9
go mod edit -replace github.com/c0deZ3R0/go-sync-kit=../../..
go mod tidy
go run main.go
```

## Example Progression

1. **Examples 3-4**: Learn basic conflict resolution and storage patterns
2. **Examples 5-7**: Master real-time sync and advanced processing
3. **Examples 8-9**: Implement production-ready systems with state machines and monitoring

## Key Concepts Demonstrated

### State Machine Lifecycle
- State transitions during conflict resolution
- Performance metrics collection
- Workflow tracking and management
- Observer pattern implementation

### Advanced Rule Systems  
- Event type and metadata matching
- Complex conditional logic
- Priority-based resolution
- Business rule integration

### Production Monitoring
- Real-time system metrics
- Automated alerting
- Health check systems
- Performance analysis

### Integration Patterns
- Custom resolver implementation
- Observability hook integration
- Monitoring system connectivity
- Business logic customization

These examples provide production-ready patterns for building sophisticated synchronization systems with comprehensive monitoring and state management capabilities.
