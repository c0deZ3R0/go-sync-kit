package http

// SimpleEvent is a basic implementation of sync.Event
type SimpleEvent struct {
    IDValue          string
    TypeValue        string
    AggregateIDValue string
    DataValue        map[string]interface{}
    MetadataValue    map[string]interface{}
}

func (e *SimpleEvent) ID() string          { return e.IDValue }
func (e *SimpleEvent) Type() string        { return e.TypeValue }
func (e *SimpleEvent) AggregateID() string { return e.AggregateIDValue }
func (e *SimpleEvent) Data() map[string]interface{}    { return e.DataValue }
func (e *SimpleEvent) Metadata() map[string]interface{} { return e.MetadataValue }
