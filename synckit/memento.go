package synckit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ResolutionMemento captures the complete state and history of a conflict resolution.
// It implements the Memento pattern to provide audit trails and rollback capabilities.
// This struct is designed to be JSON serializable by avoiding Event interface fields.
type ResolutionMemento struct {
	// Unique identifier for this resolution
	ID string `json:"id"`
	
	// Timestamp when the resolution occurred
	Timestamp time.Time `json:"timestamp"`
	
	// Conflict information (serialization-friendly)
	EventType     string            `json:"event_type"`
	AggregateID   string            `json:"aggregate_id"`
	ChangedFields []string          `json:"changed_fields,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
	
	// Resolution result (serialization-friendly) 
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons,omitempty"`
	
	// Resolution metadata
	ResolverName string            `json:"resolver_name"`
	RuleName     string            `json:"rule_name,omitempty"`
	GroupName    string            `json:"group_name,omitempty"`
	Context      map[string]any    `json:"context,omitempty"`
	
	// Audit trail information
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Origin    string `json:"origin,omitempty"`
	
	// Performance metrics
	ResolutionDuration time.Duration `json:"resolution_duration"`
	
	// State before resolution (for rollback)
	BeforeState *ConflictState `json:"before_state,omitempty"`
	
	// State after resolution
	AfterState *ConflictState `json:"after_state,omitempty"`
	
	// For internal use - not serialized
	OriginalConflict Conflict `json:"-"`
	ResolvedConflict ResolvedConflict `json:"-"`
}

// ConflictState captures the state of entities involved in a conflict.
// This struct is designed to be JSON serializable by storing only the data
// payloads rather than the full Event interfaces.
type ConflictState struct {
	AggregateID    string         `json:"aggregate_id"`
	EventType      string         `json:"event_type,omitempty"`
	LocalVersion   string         `json:"local_version"`
	RemoteVersion  string         `json:"remote_version"`
	LocalData      any            `json:"local_data,omitempty"`
	RemoteData     any            `json:"remote_data,omitempty"`
	ResolvedData   any            `json:"resolved_data,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// MementoCaretaker manages the storage and retrieval of resolution mementos.
// It acts as the caretaker in the Memento pattern.
type MementoCaretaker interface {
	// Save stores a resolution memento
	Save(ctx context.Context, memento *ResolutionMemento) error
	
	// Get retrieves a specific memento by ID
	Get(ctx context.Context, id string) (*ResolutionMemento, error)
	
	// List retrieves mementos based on criteria
	List(ctx context.Context, criteria *MementoCriteria) ([]*ResolutionMemento, error)
	
	// Delete removes a memento (with appropriate authorization)
	Delete(ctx context.Context, id string) error
	
	// GetAuditTrail returns the complete audit trail for an aggregate
	GetAuditTrail(ctx context.Context, aggregateID string) ([]*ResolutionMemento, error)
}

// MementoCriteria defines search criteria for querying mementos.
type MementoCriteria struct {
	AggregateID   string     `json:"aggregate_id,omitempty"`
	UserID        string     `json:"user_id,omitempty"`
	ResolverName  string     `json:"resolver_name,omitempty"`
	GroupName     string     `json:"group_name,omitempty"`
	FromTime      *time.Time `json:"from_time,omitempty"`
	ToTime        *time.Time `json:"to_time,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
}

// InMemoryMementoCaretaker provides an in-memory implementation of MementoCaretaker.
// For production use, implement a persistent storage backend.
type InMemoryMementoCaretaker struct {
	mementos map[string]*ResolutionMemento
}

// NewInMemoryMementoCaretaker creates a new in-memory memento caretaker.
func NewInMemoryMementoCaretaker() *InMemoryMementoCaretaker {
	return &InMemoryMementoCaretaker{
		mementos: make(map[string]*ResolutionMemento),
	}
}

func (c *InMemoryMementoCaretaker) Save(ctx context.Context, memento *ResolutionMemento) error {
	if memento.ID == "" {
		return fmt.Errorf("memento ID cannot be empty")
	}
	
	// Deep copy to avoid external mutations
	data, err := json.Marshal(memento)
	if err != nil {
		return fmt.Errorf("failed to serialize memento: %w", err)
	}
	
	var copy ResolutionMemento
	if err := json.Unmarshal(data, &copy); err != nil {
		return fmt.Errorf("failed to deserialize memento: %w", err)
	}
	
	c.mementos[memento.ID] = &copy
	return nil
}

func (c *InMemoryMementoCaretaker) Get(ctx context.Context, id string) (*ResolutionMemento, error) {
	memento, exists := c.mementos[id]
	if !exists {
		return nil, fmt.Errorf("memento with ID %s not found", id)
	}
	
	// Return deep copy to prevent external mutations
	data, err := json.Marshal(memento)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize memento: %w", err)
	}
	
	var copy ResolutionMemento
	if err := json.Unmarshal(data, &copy); err != nil {
		return nil, fmt.Errorf("failed to deserialize memento: %w", err)
	}
	
	return &copy, nil
}

func (c *InMemoryMementoCaretaker) List(ctx context.Context, criteria *MementoCriteria) ([]*ResolutionMemento, error) {
	var results []*ResolutionMemento
	
	for _, memento := range c.mementos {
		if c.matchesCriteria(memento, criteria) {
			// Deep copy
			data, _ := json.Marshal(memento)
			var copy ResolutionMemento
			json.Unmarshal(data, &copy)
			results = append(results, &copy)
		}
	}
	
	// Apply limit and offset
	if criteria != nil {
		if criteria.Offset > 0 && criteria.Offset < len(results) {
			results = results[criteria.Offset:]
		}
		if criteria.Limit > 0 && criteria.Limit < len(results) {
			results = results[:criteria.Limit]
		}
	}
	
	return results, nil
}

func (c *InMemoryMementoCaretaker) Delete(ctx context.Context, id string) error {
	if _, exists := c.mementos[id]; !exists {
		return fmt.Errorf("memento with ID %s not found", id)
	}
	
	delete(c.mementos, id)
	return nil
}

func (c *InMemoryMementoCaretaker) GetAuditTrail(ctx context.Context, aggregateID string) ([]*ResolutionMemento, error) {
	criteria := &MementoCriteria{
		AggregateID: aggregateID,
	}
	return c.List(ctx, criteria)
}

func (c *InMemoryMementoCaretaker) matchesCriteria(memento *ResolutionMemento, criteria *MementoCriteria) bool {
	if criteria == nil {
		return true
	}
	
	if criteria.AggregateID != "" && memento.AggregateID != criteria.AggregateID {
		return false
	}
	
	if criteria.UserID != "" && memento.UserID != criteria.UserID {
		return false
	}
	
	if criteria.ResolverName != "" && memento.ResolverName != criteria.ResolverName {
		return false
	}
	
	if criteria.GroupName != "" && memento.GroupName != criteria.GroupName {
		return false
	}
	
	if criteria.FromTime != nil && memento.Timestamp.Before(*criteria.FromTime) {
		return false
	}
	
	if criteria.ToTime != nil && memento.Timestamp.After(*criteria.ToTime) {
		return false
	}
	
	return true
}

// AuditableResolver wraps any ConflictResolver to provide audit trail capabilities.
type AuditableResolver struct {
	wrapped   ConflictResolver
	caretaker MementoCaretaker
	logger    Logger
	
	// Context extractors for audit metadata
	userIDExtractor    func(context.Context) string
	sessionIDExtractor func(context.Context) string
	originExtractor    func(context.Context) string
}

// NewAuditableResolver creates a new auditable resolver that wraps an existing resolver.
func NewAuditableResolver(resolver ConflictResolver, caretaker MementoCaretaker, opts ...AuditableOption) *AuditableResolver {
	ar := &AuditableResolver{
		wrapped:   resolver,
		caretaker: caretaker,
	}
	
	for _, opt := range opts {
		opt.apply(ar)
	}
	
	return ar
}

// AuditableOption provides configuration options for AuditableResolver.
type AuditableOption interface {
	apply(*AuditableResolver)
}

type auditableOptionFunc func(*AuditableResolver)

func (f auditableOptionFunc) apply(ar *AuditableResolver) {
	f(ar)
}

// WithAuditLogger sets a logger for the auditable resolver.
func WithAuditLogger(logger Logger) AuditableOption {
	return auditableOptionFunc(func(ar *AuditableResolver) {
		ar.logger = logger
	})
}

// WithUserIDExtractor sets a function to extract user ID from context.
func WithUserIDExtractor(extractor func(context.Context) string) AuditableOption {
	return auditableOptionFunc(func(ar *AuditableResolver) {
		ar.userIDExtractor = extractor
	})
}

// WithSessionIDExtractor sets a function to extract session ID from context.
func WithSessionIDExtractor(extractor func(context.Context) string) AuditableOption {
	return auditableOptionFunc(func(ar *AuditableResolver) {
		ar.sessionIDExtractor = extractor
	})
}

// WithOriginExtractor sets a function to extract origin information from context.
func WithOriginExtractor(extractor func(context.Context) string) AuditableOption {
	return auditableOptionFunc(func(ar *AuditableResolver) {
		ar.originExtractor = extractor
	})
}

// Resolve implements ConflictResolver interface with audit trail creation.
func (ar *AuditableResolver) Resolve(ctx context.Context, conflict Conflict) (ResolvedConflict, error) {
	start := time.Now()
	
	// Create before state
	beforeState := &ConflictState{
		AggregateID:   conflict.AggregateID,
		EventType:     conflict.EventType,
		LocalVersion:  conflict.Local.Version.String(),
		RemoteVersion: conflict.Remote.Version.String(),
		LocalData:     conflict.Local.Event.Data(),
		RemoteData:    conflict.Remote.Event.Data(),
		Metadata:      conflict.Metadata,
	}
	
	// Perform the actual resolution
	resolved, err := ar.wrapped.Resolve(ctx, conflict)
	duration := time.Since(start)
	
	// Create memento regardless of success/failure for audit purposes
	memento := &ResolutionMemento{
		ID:                 generateMementoID(),
		Timestamp:         time.Now(),
		EventType:         conflict.EventType,
		AggregateID:       conflict.AggregateID,
		ChangedFields:     conflict.ChangedFields,
		Metadata:          conflict.Metadata,
		OriginalConflict:  conflict,
		ResolverName:      fmt.Sprintf("%T", ar.wrapped),
		Context:           make(map[string]any),
		ResolutionDuration: duration,
		BeforeState:       beforeState,
	}
	
	// Extract audit metadata from context
	if ar.userIDExtractor != nil {
		memento.UserID = ar.userIDExtractor(ctx)
	}
	if ar.sessionIDExtractor != nil {
		memento.SessionID = ar.sessionIDExtractor(ctx)
	}
	if ar.originExtractor != nil {
		memento.Origin = ar.originExtractor(ctx)
	}
	
	if err != nil {
		// Record the error in context
		memento.Context["error"] = err.Error()
		memento.Context["success"] = false
	} else {
		// Record successful resolution
		memento.ResolvedConflict = resolved
		memento.Decision = resolved.Decision
		memento.Reasons = resolved.Reasons
		memento.Context["success"] = true
		
		// Create after state
		if len(resolved.ResolvedEvents) > 0 {
			// Use the first resolved event as representative
			resolvedEvent := resolved.ResolvedEvents[0]
			memento.AfterState = &ConflictState{
				AggregateID:   conflict.AggregateID,
				LocalVersion:  beforeState.LocalVersion,
				RemoteVersion: beforeState.RemoteVersion,
				ResolvedData:  resolvedEvent.Event.Data(),
				Metadata:      conflict.Metadata,
			}
		}
	}
	
	// Save memento (don't fail the resolution if audit save fails)
	if saveErr := ar.caretaker.Save(ctx, memento); saveErr != nil && ar.logger != nil {
		ar.logger.Error("Failed to save resolution memento", "error", saveErr, "memento_id", memento.ID)
	} else if ar.logger != nil {
		ar.logger.Debug("Saved resolution memento", "memento_id", memento.ID, "aggregate_id", conflict.AggregateID)
	}
	
	return resolved, err
}

// GetAuditTrail retrieves the audit trail for a specific aggregate.
func (ar *AuditableResolver) GetAuditTrail(ctx context.Context, aggregateID string) ([]*ResolutionMemento, error) {
	return ar.caretaker.GetAuditTrail(ctx, aggregateID)
}

// RollbackCapability provides methods for analyzing rollback possibilities.
// Note: Actual rollback implementation would require coordination with the storage layer.
type RollbackCapability struct {
	caretaker MementoCaretaker
}

// NewRollbackCapability creates a new rollback capability analyzer.
func NewRollbackCapability(caretaker MementoCaretaker) *RollbackCapability {
	return &RollbackCapability{caretaker: caretaker}
}

// AnalyzeRollback analyzes what would be involved in rolling back a specific resolution.
func (rc *RollbackCapability) AnalyzeRollback(ctx context.Context, mementoID string) (*RollbackAnalysis, error) {
	memento, err := rc.caretaker.Get(ctx, mementoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memento: %w", err)
	}
	
	// Get subsequent resolutions for this aggregate
	subsequent, err := rc.caretaker.GetAuditTrail(ctx, memento.AggregateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit trail: %w", err)
	}
	
	// Find resolutions that occurred after this one
	var affectedResolutions []*ResolutionMemento
	for _, other := range subsequent {
		if other.Timestamp.After(memento.Timestamp) {
			affectedResolutions = append(affectedResolutions, other)
		}
	}
	
	analysis := &RollbackAnalysis{
		TargetMemento:        memento,
		AffectedResolutions:  affectedResolutions,
		RollbackComplexity:   calculateComplexity(affectedResolutions),
		RequiresReprocessing: len(affectedResolutions) > 0,
	}
	
	return analysis, nil
}

// RollbackAnalysis provides information about the implications of rolling back a resolution.
type RollbackAnalysis struct {
	TargetMemento        *ResolutionMemento   `json:"target_memento"`
	AffectedResolutions  []*ResolutionMemento `json:"affected_resolutions"`
	RollbackComplexity   string              `json:"rollback_complexity"`
	RequiresReprocessing bool                `json:"requires_reprocessing"`
	Warnings             []string            `json:"warnings,omitempty"`
}

func calculateComplexity(affectedResolutions []*ResolutionMemento) string {
	if len(affectedResolutions) == 0 {
		return "simple"
	} else if len(affectedResolutions) <= 3 {
		return "moderate"
	} else {
		return "complex"
	}
}

// generateMementoID generates a unique ID for a memento.
// In production, this should use a proper UUID library.
func generateMementoID() string {
	return fmt.Sprintf("memento_%d", time.Now().UnixNano())
}
