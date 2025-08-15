# Example 4: Conflict Resolution Demo

This example demonstrates how Go Sync Kit handles conflicts when multiple clients modify the same data simultaneously.

## What You'll Learn

- **Conflict Detection**: How conflicts are automatically identified by aggregate ID
- **Resolution Strategies**: Different approaches to handling conflicts
- **Custom Resolvers**: Implementing your own conflict resolution logic
- **Multi-Client Simulation**: Understanding distributed editing scenarios
- **Conflict Metadata**: How resolution decisions are tracked and explained

## Conflict Resolution Strategies

### 1. Last-Write-Wins (LWW)
- **Strategy**: Most recent event wins based on timestamps
- **Use Case**: Simple scenarios where latest change should always win
- **Trade-off**: May lose earlier changes, but guarantees consistency

### 2. First-Write-Wins (FWW)  
- **Strategy**: First event encountered wins
- **Use Case**: When you want to preserve the original change
- **Trade-off**: Later changes may be discarded

### 3. Additive Merge
- **Strategy**: Combines events when possible without conflict
- **Use Case**: Scenarios where changes can be merged (like counters)
- **Trade-off**: Only works for mergeable data types

### 4. Custom Resolver
- **Strategy**: Implement your own business logic
- **Use Case**: Complex scenarios requiring domain-specific rules
- **Trade-off**: More complex to implement but maximum flexibility

## Running the Example

```bash
cd 04-conflict-resolution
go run main.go
```

## What Happens

1. **Setup**: Creates two client databases (simulating distributed clients)
2. **Conflict Creation**: Both clients edit the same document at different times
3. **Strategy Comparison**: Shows how each resolution strategy handles the conflict
4. **Result Analysis**: Displays which event was chosen and why
5. **State Inspection**: Shows final state in both client databases

## Output Structure

For each strategy, you'll see:
- 🅰️ **Client A Event**: First edit with timestamp
- 🅱️ **Client B Event**: Conflicting edit with later timestamp  
- 🔄 **Sync Results**: How many events were pushed/pulled/resolved
- ✅ **Final State**: What each client's database contains after resolution
- 💡 **Explanation**: Why this strategy chose this particular event

## Key Concepts

- **Aggregate ID**: Events with the same aggregate ID can conflict
- **Automatic Detection**: Conflicts are found during sync operations
- **Transparent Resolution**: Happens automatically based on chosen strategy
- **Metadata Tracking**: Resolution decisions include reasons and timestamps
- **Multi-Client Support**: Each client can have different conflict preferences

**Continue to Example 5** to learn about real-time synchronization with automatic sync intervals.
