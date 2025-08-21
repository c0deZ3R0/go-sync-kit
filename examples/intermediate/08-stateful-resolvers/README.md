# Example 8: Stateful Conflict Resolvers

This example demonstrates the powerful new stateful conflict resolution system with state machine integration, performance monitoring, and workflow tracking.

## What You'll Learn

- **State Machine Integration**: How resolvers transition through states during conflict resolution
- **Performance Monitoring**: Real-time metrics and tracking for conflict resolution operations
- **Dynamic Rule-Based Resolution**: Advanced rule systems that adapt based on conflict characteristics
- **Workflow Tracking**: Complete audit trails and workflow state management
- **Observability Hooks**: Custom monitoring and logging integration
- **Rule Evaluation**: Complex condition matching and resolution strategies

## Key Features Demonstrated

### 1. Stateful Dynamic Resolvers
- **State Tracking**: Monitor resolver state transitions in real-time
- **Performance Metrics**: Automatic collection of resolution times and success rates
- **Workflow Management**: Track active workflows and completion status
- **Audit Trails**: Complete history of resolution decisions

### 2. Advanced Rule Systems
- **Event Type Matching**: Rules based on event types and patterns
- **Metadata Conditions**: Complex rule matching on event metadata
- **Field-Based Rules**: Resolution based on changed field analysis
- **Combinatorial Logic**: AND/OR/NOT rule combinations

### 3. Custom Resolution Strategies
- **Business Logic Integration**: Domain-specific resolution rules
- **Priority-Based Resolution**: Resolve conflicts based on business priorities
- **User-Driven Resolution**: Interactive resolution with approval workflows
- **Time-Based Strategies**: Resolution based on time windows and deadlines

### 4. Monitoring & Observability
- **Real-time Metrics**: Live performance dashboards
- **State Change Tracking**: Monitor resolver state machine transitions
- **Rule Evaluation Metrics**: Track which rules match and their performance
- **Custom Hook Integration**: Plug in your own monitoring systems

## Running the Example

```bash
cd 08-stateful-resolvers
go run main.go
```

## What Happens

1. **Setup**: Creates stateful resolvers with different rule configurations
2. **Rule Configuration**: Demonstrates various rule types and conditions
3. **State Machine Demo**: Shows state transitions during conflict resolution
4. **Performance Monitoring**: Real-time metrics collection and reporting
5. **Workflow Tracking**: Complete audit trails of resolution processes
6. **Custom Hooks**: Integration with monitoring and alerting systems

## Output Structure

- 🎛️ **Configuration Phase**: Setting up rules and resolvers
- 🔄 **State Transitions**: Real-time state machine changes
- 📊 **Performance Metrics**: Resolution times and success rates
- 📋 **Rule Evaluation**: Which rules matched and their execution times
- 🔍 **Audit Trails**: Complete workflow history
- 📈 **Final Statistics**: Summary of all resolution activities

## Advanced Concepts

- **State Machine Lifecycle**: Understanding resolver state transitions
- **Rule Priority**: How rule ordering affects resolution
- **Performance Optimization**: Monitoring and tuning resolver performance  
- **Workflow Management**: Tracking complex resolution workflows
- **Custom Observability**: Building monitoring systems around resolvers
- **Error Handling**: Graceful failure and recovery mechanisms

This example showcases the most advanced features of Go Sync Kit's conflict resolution system, providing production-ready patterns for complex distributed systems.

**Continue to Example 9** to learn about advanced monitoring and observability patterns.
